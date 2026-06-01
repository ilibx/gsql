package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ilibx/gsql/pkg/catalog"
)

const (
	larkBaseURL    = "https://open.feishu.cn"
	larkAuthAPI    = "/open-apis/auth/v3/tenant_access_token/internal"
	larkDriveAPI   = "/open-apis/drive/v1/files"
	larkUploadAPI  = "/open-apis/drive/v1/files/upload_all"
	larkCreateFolderAPI = "/open-apis/drive/v1/files/create_folder"
	larkDownloadAPI = "/open-apis/drive/v1/files/%s/download"
	larkDeleteAPI  = "/open-apis/drive/v1/files/%s"
	larkMoveAPI    = "/open-apis/drive/v1/files/%s/move"
	larkUpdateAPI  = "/open-apis/drive/v1/files/%s"
	larkMetaAPI    = "/open-apis/drive/v1/metas/batch_query"
	larkRootFolderMetaAPI = "/open-apis/drive/explorer/v2/root_folder/meta"
	larkExportCreateAPI   = "/open-apis/drive/v1/export_tasks"
	larkExportGetAPI      = "/open-apis/drive/v1/export_tasks/%s"
	larkExportDownloadAPI = "/open-apis/drive/v1/export_tasks/file/%s/download"

	larkAuthTTL     = 90 * time.Minute
	larkHTTPTimeout = 30 * time.Second
)

type larkStorage struct {
	appID     string
	appSecret string
	folder string
	chatID    string

	mu        sync.Mutex
	token     string
	tokenExp  time.Time
	rootToken string
	rootOnce  sync.Once
	rootErr   error
	httpCli   *http.Client

	writeBufs   map[string][]byte
	writeBufsMu sync.Mutex
}

func newLarkStorage(tbl *catalog.Table) (Storage, error) {
	appID := tbl.Option("app_id", "")
	if appID == "" {
		appID = tbl.Option("lark_app_id", "")
	}
	appSecret := tbl.Option("app_secret", "")
	if appSecret == "" {
		appSecret = tbl.Option("lark_app_secret", "")
	}
	if appID == "" || appSecret == "" {
		return nil, fmt.Errorf("missing app_id or app_secret for Lark table %s", tbl.Name)
	}

	// Support both folder name and direct token for root
	folder := tbl.Option("path", "")
	rootToken := tbl.Option("root_token", "")
	if rootToken == "" {
		rootToken = tbl.Option("lark_root_token", "")
		rawURL := tbl.Option("url", "")
		if rootToken == "" && rawURL != "" {
			if u, err := url.Parse(rawURL); err == nil && u.Scheme == "lark" {
				folder = u.Host
			}
		}
	}
	if rootToken == "" && folder == "" {
		// If root_token is explicitly set to empty, allow resolving to root
		if _, hasRootToken := tbl.WithOptions["root_token"]; !hasRootToken {
			if _, hasLarkRootToken := tbl.WithOptions["lark_root_token"]; !hasLarkRootToken {
				return nil, fmt.Errorf("missing root: configure folder or root_token for Lark table %s", tbl.Name)
			}
		}
	}

	chatID := tbl.Option("chat_id", "")
	if chatID == "" {
		chatID = tbl.Option("lark_chat_id", "")
	}

	return &larkStorage{
		appID:      appID,
		appSecret:  appSecret,
		folder:     folder,
		rootToken:  rootToken,
		chatID:     chatID,
		httpCli:    &http.Client{Timeout: larkHTTPTimeout},
		writeBufs:  make(map[string][]byte),
	}, nil
}

// resolveRoot lazily resolves the root token from folder or returns rootToken directly.
func (s *larkStorage) resolveRoot(ctx context.Context) (string, error) {
	s.rootOnce.Do(func() {
		if s.rootToken != "" {
			return // Already have a direct token
		}
		// Resolve the app root (also needed when root_token is explicitly "")
		appRoot, err := s.getAppRoot(ctx)
		if err != nil {
			if s.folder == "" {
				s.rootErr = fmt.Errorf("missing root: configure folder or root_token")
				return
			}
			s.rootErr = fmt.Errorf("get app root failed: %w", err)
			return
		}
		if s.folder == "" {
			s.rootToken = appRoot
			return
		}
		// Look up or create the folder under the app's drive root
		token, err := s.ensureFolder(ctx, appRoot, s.folder)
		if err != nil {
			s.rootErr = fmt.Errorf("resolve folder %q failed: %w", s.folder, err)
			return
		}
		s.rootToken = token
	})
	return s.rootToken, s.rootErr
}

// getAppRoot finds the app's drive root folder token.
func (s *larkStorage) getAppRoot(ctx context.Context) (string, error) {
	// Try all known approaches to find the root folder token.
	token, err := s.getRootViaMetaAPI(ctx)
	if err == nil && token != "" {
		return token, nil
	}
	token, err = s.getRootViaListAPI(ctx)
	if err == nil && token != "" {
		return token, nil
	}
	return "", fmt.Errorf("cannot determine app root folder; use root_token directly or ensure folder exists under the app's drive space")
}

func (s *larkStorage) getRootViaMetaAPI(ctx context.Context) (string, error) {
	data, err := s.doGet(ctx, larkBaseURL+larkRootFolderMetaAPI)
	if err != nil {
		return "", err
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &resp) != nil || resp.Code != 0 || resp.Data.Token == "" {
		return "", fmt.Errorf("root_folder/meta: code=%d", resp.Code)
	}
	return resp.Data.Token, nil
}

func (s *larkStorage) getRootViaListAPI(ctx context.Context) (string, error) {
	data, err := s.doGet(ctx, larkBaseURL+larkDriveAPI+"?page_size=10")
	if err != nil {
		return "", err
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Files []struct {
				Token       string `json:"token"`
				ParentToken string `json:"parent_token"`
			} `json:"files"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &resp) != nil || resp.Code != 0 {
		return "", fmt.Errorf("list files: code=%d", resp.Code)
	}
	// parent_token directly from the listing
	for _, f := range resp.Data.Files {
		if f.ParentToken != "" {
			return f.ParentToken, nil
		}
	}
	// Fallback: query metadata of the first file via batch_query
	if len(resp.Data.Files) > 0 {
		body, _ := json.Marshal(map[string]interface{}{
			"request_docs": []map[string]string{
				{"doc_token": resp.Data.Files[0].Token, "doc_type": "file"},
			},
			"with_url":        false,
			"with_properties": true,
		})
		data, err := s.doPost(ctx, larkBaseURL+larkMetaAPI, "application/json", bytes.NewReader(body))
		if err == nil {
			var metaResp struct {
				Code int `json:"code"`
				Data struct {
					Metas []struct {
						Properties struct {
							ParentToken string `json:"parent_token"`
						} `json:"properties"`
					} `json:"metas"`
				} `json:"data"`
			}
			if json.Unmarshal(data, &metaResp) == nil && metaResp.Code == 0 && len(metaResp.Data.Metas) > 0 {
				if pt := metaResp.Data.Metas[0].Properties.ParentToken; pt != "" {
					return pt, nil
				}
			}
		}
	}
	return "", fmt.Errorf("no root token found from file listing")
}

func (s *larkStorage) getToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	if s.token != "" && time.Now().Before(s.tokenExp) {
		t := s.token
		s.mu.Unlock()
		return t, nil
	}
	s.mu.Unlock()

	body, _ := json.Marshal(map[string]string{
		"app_id":     s.appID,
		"app_secret": s.appSecret,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, larkBaseURL+larkAuthAPI, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpCli.Do(req)
	if err != nil {
		return "", fmt.Errorf("lark auth request failed: %w", err)
	}
	defer resp.Body.Close()

	var res struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Token string `json:"tenant_access_token"`
		Expire int `json:"expire"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", fmt.Errorf("lark auth decode failed: %w", err)
	}
	if res.Code != 0 {
		return "", fmt.Errorf("lark auth error code %d: %s", res.Code, res.Msg)
	}

	s.mu.Lock()
	s.token = res.Token
	s.tokenExp = time.Now().Add(time.Duration(res.Expire-300) * time.Second)
	s.mu.Unlock()
	return res.Token, nil
}

func (s *larkStorage) doGet(ctx context.Context, apiURL string) ([]byte, error) {
	token, err := s.getToken(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.httpCli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *larkStorage) doDownload(ctx context.Context, apiURL string) ([]byte, error) {
	data, err := s.doGet(ctx, apiURL)
	if err != nil {
		return nil, err
	}
	// Check for Lark error JSON response (some download APIs return
	// error JSON even with 200 status, others return non-200 with
	// error JSON).
	var errResp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if json.Unmarshal(data, &errResp) == nil && errResp.Code != 0 {
		return nil, fmt.Errorf("lark download error code %d: %s", errResp.Code, errResp.Msg)
	}
	return data, nil
}

func (s *larkStorage) doPost(ctx context.Context, apiURL string, contentType string, body io.Reader) ([]byte, error) {
	token, err := s.getToken(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := s.httpCli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *larkStorage) doPatch(ctx context.Context, apiURL string, body []byte) ([]byte, error) {
	token, err := s.getToken(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpCli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *larkStorage) doDelete(ctx context.Context, apiURL string) ([]byte, error) {
	token, err := s.getToken(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.httpCli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

type larkFileEntry struct {
	Token      string `json:"token"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	ParentToken string `json:"parent_token"`
	URL        string `json:"url"`
	Size       int    `json:"size"`
}

type larkListResp struct {
	Code int `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Entries []larkFileEntry `json:"files"`
		HasMore bool `json:"has_more"`
		PageToken string `json:"page_token"`
	} `json:"data"`
}

type larkMetaBatchResp struct {
	Code int `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Metas []struct {
			DocToken string `json:"doc_token"`
			DocType  string `json:"doc_type"`
			Title    string `json:"title"`
			URL      string `json:"url"`
		} `json:"metas"`
	} `json:"data"`
}

type larkCreateFolderResp struct {
	Code int `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		NodeToken string `json:"token"`
		URL       string `json:"url"`
	} `json:"data"`
}

type larkUploadResp struct {
	Code int `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		FileToken string `json:"file_token"`
	} `json:"data"`
}

type larkExportCreateResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Ticket string `json:"ticket"`
	} `json:"data"`
}

type larkExportStatusResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Result struct {
			FileToken   string `json:"file_token"`
			JobStatus   int    `json:"job_status"`
			JobErrorMsg string `json:"job_error_msg"`
		} `json:"result"`
	} `json:"data"`
}

// ---- path resolution ----

func (s *larkStorage) listDir(ctx context.Context, parentToken string) ([]larkFileEntry, error) {
	var all []larkFileEntry
	pageToken := ""
	for {
		u := fmt.Sprintf("%s%s?folder_token=%s&page_size=100", larkBaseURL, larkDriveAPI, parentToken)
		if pageToken != "" {
			u += "&page_token=" + pageToken
		}
		data, err := s.doGet(ctx, u)
		if err != nil {
			return nil, err
		}
		var resp larkListResp
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("lark list decode failed: %w", err)
		}
		if resp.Code != 0 {
			return nil, fmt.Errorf("lark list error code %d: %s", resp.Code, resp.Msg)
		}
		all = append(all, resp.Data.Entries...)
		if !resp.Data.HasMore {
			break
		}
		pageToken = resp.Data.PageToken
	}
	return all, nil
}

func (s *larkStorage) resolveChildToken(ctx context.Context, parentToken, childName string) (string, bool, string, error) {
	entries, err := s.listDir(ctx, parentToken)
	if err != nil {
		return "", false, "", err
	}
	for _, e := range entries {
		if e.Name == childName {
			return e.Token, e.Type == "folder", e.Type, nil
		}
	}
	return "", false, "", os.ErrNotExist
}

func (s *larkStorage) getFileMeta(ctx context.Context, token string) (*larkFileEntry, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"request_docs": []map[string]string{
			{"doc_token": token, "doc_type": "file"},
		},
		"with_url": true,
	})
	data, err := s.doPost(ctx, larkBaseURL+larkMetaAPI, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	var resp larkMetaBatchResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("lark meta decode failed: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("lark meta error code %d: %s", resp.Code, resp.Msg)
	}
	if len(resp.Data.Metas) == 0 {
		return nil, fmt.Errorf("lark meta: no metadata found for token %s", token)
	}
	m := resp.Data.Metas[0]
	return &larkFileEntry{
		Token: m.DocToken,
		Name:  m.Title,
		Type:  m.DocType,
		URL:   m.URL,
	}, nil
}

// root returns the resolved root token, calling resolveRoot lazily if needed.
func (s *larkStorage) root(ctx context.Context) (string, error) {
	token, err := s.resolveRoot(ctx)
	if err != nil {
		return "", err
	}
	if token == "" {
		return "", fmt.Errorf("root token not available")
	}
	return token, nil
}

func (s *larkStorage) resolvePath(ctx context.Context, name string) (token string, isFolder bool, err error) {
	name = strings.TrimPrefix(name, "/")
	name = strings.TrimSuffix(name, "/")
	rootToken, err := s.root(ctx)
	if err != nil {
		return "", false, err
	}
	if name == "" {
		return rootToken, true, nil
	}
	parts := strings.Split(name, "/")
	currentToken := rootToken
	for i, part := range parts {
		t, isDir, _, err := s.resolveChildToken(ctx, currentToken, part)
		if err != nil {
			return "", false, fmt.Errorf("path %q not found at %q: %w", name, strings.Join(parts[:i+1], "/"), err)
		}
		if t == "" {
			return "", false, fmt.Errorf("path %q not found at %q", name, strings.Join(parts[:i+1], "/"))
		}
		if i == len(parts)-1 {
			return t, isDir, nil
		}
		if !isDir {
			return "", false, fmt.Errorf("%q is not a folder", strings.Join(parts[:i+1], "/"))
		}
		currentToken = t
	}
	return currentToken, true, nil
}

// grantFullAccess grants full_access permission to the configured chat.
// tokenType must be "file" or "folder" to match the entity type.
func (s *larkStorage) grantFullAccess(ctx context.Context, token, tokenType string) error {
	if s.chatID == "" {
		return nil
	}

	body, _ := json.Marshal(map[string]string{
		"member_type": "openchat",
		"member_id":   s.chatID,
		"perm":        "full_access",
	})
	u := fmt.Sprintf("%s/open-apis/drive/v1/permissions/%s/members?type=%s", larkBaseURL, token, tokenType)
	data, err := s.doPost(ctx, u, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("grant access request failed: %w", err)
	}

	// Handle non-JSON error responses (e.g., permission denied)
	trimmed := strings.TrimSpace(string(data))
	if !strings.HasPrefix(trimmed, "{") {
		// Not JSON - likely an error message
		return fmt.Errorf("grant access failed: %s", trimmed)
	}

	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("grant access decode failed: %w", err)
	}
	if resp.Code != 0 {
		return fmt.Errorf("grant access error code %d: %s", resp.Code, resp.Msg)
	}
	return nil
}

func (s *larkStorage) ensureFolder(ctx context.Context, parentToken, name string) (string, error) {
	t, isDir, _, err := s.resolveChildToken(ctx, parentToken, name)
	if err != nil {
		return "", err
	}
	if t != "" && isDir {
		if s.chatID != "" {
			if perr := s.grantFullAccess(ctx, t, "folder"); perr != nil {
				fmt.Fprintf(os.Stderr, "WARNING: grant folder access to chat failed: %v\n", perr)
			}
		}
		return t, nil
	}
	body, _ := json.Marshal(map[string]string{
		"name":         name,
		"folder_token": parentToken,
	})
	data, err := s.doPost(ctx, larkBaseURL+larkCreateFolderAPI, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	var resp larkCreateFolderResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("lark create folder decode failed: %w", err)
	}
	if resp.Code != 0 {
		return "", fmt.Errorf("lark create folder error code %d: %s", resp.Code, resp.Msg)
	}
	// Grant full access to the configured chat (best effort - continue if it fails)
	if s.chatID != "" {
		if perr := s.grantFullAccess(ctx, resp.Data.NodeToken, "folder"); perr != nil {
			fmt.Fprintf(os.Stderr, "WARNING: grant folder access to chat failed: %v\n", perr)
		}
	}

	return resp.Data.NodeToken, nil
}

func (s *larkStorage) ensurePath(ctx context.Context, name string) (parentToken, fileName string, err error) {
	name = strings.TrimPrefix(name, "/")
	name = strings.TrimSuffix(name, "/")
	dir, file := path.Split(name)
	rootToken, err := s.root(ctx)
	if err != nil {
		return "", "", err
	}
	if dir == "" {
		return rootToken, file, nil
	}
	dir = strings.TrimSuffix(dir, "/")
	parts := strings.Split(dir, "/")
	currentToken := rootToken
	for _, part := range parts {
		currentToken, err = s.ensureFolder(ctx, currentToken, part)
		if err != nil {
			return "", "", err
		}
	}
	return currentToken, file, nil
}

// ---- Storage interface ----

func (s *larkStorage) Open(ctx context.Context, name string) (File, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if len(name) == 0 {
		return nil, fmt.Errorf("empty file name")
	}
	token, isDir, err := s.resolvePath(ctx, name)
	if err != nil {
		return nil, err
	}
	if isDir {
		return nil, fmt.Errorf("%q is a folder, not a file", name)
	}
	if err != nil {
		return nil, err
	}

	entryType, err := s.lookupEntryType(ctx, name)
	if err != nil {
		return nil, err
	}

	var data []byte
	if entryType == "file" {
		u := fmt.Sprintf("%s%s/%s/download", larkBaseURL, larkDriveAPI, token)
		data, err = s.doDownload(ctx, u)
	} else {
		data, err = s.downloadViaExport(ctx, token, entryType)
		if err != nil {
			return nil, err
		}
	}

	return &larkReadFile{
		baseFile: baseFile{},
		reader:   bytes.NewReader(data),
		name:     name,
		size:     uint64(len(data)),
	}, nil
}

// lookupEntryType returns the Lark entry type ("file", "sheet", "doc", etc.) for the given path.
func (s *larkStorage) lookupEntryType(ctx context.Context, name string) (string, error) {
	name = strings.TrimPrefix(name, "/")
	name = strings.TrimSuffix(name, "/")
	dir, file := path.Split(name)
	var parentToken string
	var err error
	if dir == "" {
		parentToken, err = s.root(ctx)
	} else {
		parentToken, _, err = s.resolvePath(ctx, dir)
	}
	if err != nil {
		return "", err
	}
	_, _, entryType, err := s.resolveChildToken(ctx, parentToken, file)
	if err != nil {
		return "", err
	}
	return entryType, nil
}

// downloadViaExport uses the Lark export API to download non-file type entries
// (e.g., sheets, docs, bitables) as a file.
func (s *larkStorage) downloadViaExport(ctx context.Context, token, entryType string) ([]byte, error) {
	fileExt := "xlsx"

	body, _ := json.Marshal(map[string]string{
		"file_extension": fileExt,
		"token":          token,
		"type":           entryType,
	})
	data, err := s.doPost(ctx, larkBaseURL+larkExportCreateAPI, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create export task failed: %w", err)
	}
	var createResp larkExportCreateResp
	if err := json.Unmarshal(data, &createResp); err != nil {
		return nil, fmt.Errorf("create export decode failed: %w", err)
	}
	if createResp.Code != 0 {
		return nil, fmt.Errorf("create export error code %d: %s", createResp.Code, createResp.Msg)
	}

	ticket := createResp.Data.Ticket
	pollURL := fmt.Sprintf(larkBaseURL+larkExportGetAPI, ticket) + "?token=" + token

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}

		respData, err := s.doGet(ctx, pollURL)
		if err != nil {
			return nil, fmt.Errorf("query export task failed: %w", err)
		}
		var statusResp larkExportStatusResp
		if err := json.Unmarshal(respData, &statusResp); err != nil {
			return nil, fmt.Errorf("query export decode failed: %w", err)
		}
		if statusResp.Code != 0 {
			return nil, fmt.Errorf("query export error code %d: %s", statusResp.Code, statusResp.Msg)
		}
		switch statusResp.Data.Result.JobStatus {
		case 0:
			fileToken := statusResp.Data.Result.FileToken
			if fileToken == "" {
				return nil, fmt.Errorf("export succeeded but file token is empty, raw response: %s", string(respData))
			}
			u := fmt.Sprintf(larkBaseURL+larkExportDownloadAPI, fileToken)
			downloadData, err := s.doGet(ctx, u)
			if err != nil {
				return nil, fmt.Errorf("download exported file failed: %w", err)
			}
			return downloadData, nil
		case 1:
			continue
		case 2:
			return nil, fmt.Errorf("export failed: %s", statusResp.Data.Result.JobErrorMsg)
		}
	}
}

func (s *larkStorage) Stat(ctx context.Context, name string) (FileInfo, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	_, isDir, err := s.resolvePath(ctx, name)
	if err != nil {
		return nil, err
	}
	return &baseFileInfo{
		name:  name,
		path:  name,
		size:  0,
		isDir: isDir,
	}, nil
}

func (s *larkStorage) Glob(ctx context.Context, pattern string) ([]string, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	dir, filePat := path.Split(pattern)
	dir = strings.TrimSuffix(dir, "/")

	var parentToken string
	if dir == "" {
		var err error
		parentToken, err = s.root(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		t, _, err := s.resolvePath(ctx, dir)
		if err != nil {
			return nil, err
		}
		parentToken = t
	}

	entries, err := s.listDir(ctx, parentToken)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, e := range entries {
		if e.Type == "folder" {
			continue
		}
		if matched, _ := filepath.Match(filePat, e.Name); matched {
			if dir == "" {
				result = append(result, e.Name)
			} else {
				result = append(result, dir+"/"+e.Name)
			}
		}
	}
	sort.Strings(result)
	return result, nil
}

func (s *larkStorage) List(ctx context.Context, dirPath string) ([]string, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	dirPath = strings.TrimPrefix(dirPath, "/")
	dirPath = strings.TrimSuffix(dirPath, "/")

	var parentToken string
	if dirPath == "" {
		var err error
		parentToken, err = s.root(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		t, _, err := s.resolvePath(ctx, dirPath)
		if err != nil {
			return nil, err
		}
		parentToken = t
	}

	entries, err := s.listDir(ctx, parentToken)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(entries))
	for _, e := range entries {
		if dirPath == "" {
			result = append(result, e.Name)
		} else {
			result = append(result, dirPath+"/"+e.Name)
		}
	}
	sort.Strings(result)
	return result, nil
}

func (s *larkStorage) Mkdir(ctx context.Context, name string, perm fs.FileMode) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	name = strings.TrimSuffix(strings.TrimPrefix(name, "/"), "/")
	if name == "" {
		return nil
	}
	rootToken, err := s.root(ctx)
	if err != nil {
		return err
	}
	parts := strings.Split(name, "/")
	parentToken := rootToken
	for i := 0; i < len(parts)-1; i++ {
		parentToken, err = s.ensureFolder(ctx, parentToken, parts[i])
		if err != nil {
			return err
		}
	}
	_, err = s.ensureFolder(ctx, parentToken, parts[len(parts)-1])
	return err
}

func (s *larkStorage) MkdirAll(ctx context.Context, name string, perm fs.FileMode) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	name = strings.TrimSuffix(strings.TrimPrefix(name, "/"), "/")
	if name == "" || name == "." {
		return nil
	}
	rootToken, err := s.root(ctx)
	if err != nil {
		return err
	}
	currentToken := rootToken
	for _, part := range strings.Split(name, "/") {
		currentToken, err = s.ensureFolder(ctx, currentToken, part)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *larkStorage) Remove(ctx context.Context, name string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	token, isDir, err := s.resolvePath(ctx, name)
	if err != nil {
		return err
	}
	fileType := "file"
	if isDir {
		fileType = "folder"
	}
	return s.RemoveByToken(ctx, token, fileType)
}

func (s *larkStorage) RemoveByToken(ctx context.Context, token, fileType string) error {
	u := fmt.Sprintf(larkBaseURL+larkDeleteAPI+"?type=%s", token, fileType)
	data, err := s.doDelete(ctx, u)
	if err != nil {
		return err
	}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("lark delete decode failed: %w", err)
	}
	if resp.Code != 0 {
		return fmt.Errorf("lark delete error code %d: %s", resp.Code, resp.Msg)
	}
	return nil
}

func (s *larkStorage) RemoveAll(ctx context.Context, name string) error {
	return s.Remove(ctx, name)
}

func (s *larkStorage) WriteFile(ctx context.Context, name string, data []byte, perm fs.FileMode) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	parentToken, fileName, err := s.ensurePath(ctx, name)
	if err != nil {
		return err
	}

	// Delete existing file with the same name to avoid duplicates in Lark Drive
	if token, isDir, _, derr := s.resolveChildToken(ctx, parentToken, fileName); derr == nil && !isDir {
		if rerr := s.RemoveByToken(ctx, token, "file"); rerr != nil {
			fmt.Fprintf(os.Stderr, "WARNING: remove existing file %s failed: %v\n", fileName, rerr)
		}
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField("file_name", fileName)
	w.WriteField("parent_type", "explorer")
	w.WriteField("parent_node", parentToken)
	w.WriteField("size", strconv.Itoa(len(data)))
	fw, err := w.CreateFormFile("file", fileName)
	if err != nil {
		return err
	}
	if _, err := fw.Write(data); err != nil {
		return err
	}
	w.Close()

	respData, err := s.doPost(ctx, larkBaseURL+larkUploadAPI, w.FormDataContentType(), &buf)
	if err != nil {
		return err
	}
	var resp larkUploadResp
	if err := json.Unmarshal(respData, &resp); err != nil {
		return fmt.Errorf("lark upload decode failed: %w", err)
	}
	if resp.Code != 0 {
		return fmt.Errorf("lark upload error code %d: %s", resp.Code, resp.Msg)
	}

	if s.chatID != "" {
		if serr := s.shareFile(ctx, resp.Data.FileToken, s.chatID); serr != nil {
			fmt.Fprintf(os.Stderr, "WARNING: share file to chat failed: %v\n", serr)
		}
	}

	return nil
}

func (s *larkStorage) Rename(ctx context.Context, oldName, newName string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Check for buffered write (from Create+Close without upload)
	s.writeBufsMu.Lock()
	data, hasBuf := s.writeBufs[oldName]
	if hasBuf {
		delete(s.writeBufs, oldName)
	}
	s.writeBufsMu.Unlock()

	if !hasBuf {
		// Download old file content
		file, err := s.Open(ctx, oldName)
		if err != nil {
			return fmt.Errorf("rename open old file failed: %w", err)
		}
		data, err = io.ReadAll(file)
		file.Close()
		if err != nil {
			return fmt.Errorf("rename read old file failed: %w", err)
		}
	}

	// Upload to new path
	if err := s.WriteFile(ctx, newName, data, 0o644); err != nil {
		return fmt.Errorf("rename write new file failed: %w", err)
	}

	// Delete old file — only if it was persisted in Lark (not buffered)
	if !hasBuf {
		if err := s.Remove(ctx, oldName); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: rename remove old file %s failed: %v\n", oldName, err)
		}
	}
	return nil
}

func (s *larkStorage) Create(ctx context.Context, name string) (File, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return &larkWriteFile{
		baseFile: baseFile{},
		store:    s,
		name:     name,
		buf:      &bytes.Buffer{},
	}, nil
}

func (s *larkStorage) Exists(ctx context.Context, name string) bool {
	if ctx.Err() != nil {
		return false
	}
	_, _, err := s.resolvePath(ctx, name)
	return err == nil
}

func (s *larkStorage) Join(elem ...string) string {
	return path.Join(elem...)
}

func (s *larkStorage) shareFile(ctx context.Context, fileToken, chatID string) error {
	// Grant full access to the chat so members can see the file
	return s.grantFullAccess(ctx, fileToken, "file")
}

func (s *larkStorage) ShareToChat(ctx context.Context, name, chatID string) error {
	if chatID == "" {
		chatID = s.chatID
	}
	if chatID == "" {
		return fmt.Errorf("no chat_id configured")
	}
	token, _, err := s.resolvePath(ctx, name)
	if err != nil {
		return fmt.Errorf("resolve path for share failed: %w", err)
	}
	return s.shareFile(ctx, token, chatID)
}

// ---- file types ----

type larkReadFile struct {
	baseFile
	reader *bytes.Reader
	name   string
	size   uint64
}

func (f *larkReadFile) Read(p []byte) (int, error)  { return f.reader.Read(p) }
func (f *larkReadFile) Close() error                 { return nil }
func (f *larkReadFile) String() string               { return f.name }
func (f *larkReadFile) Stat(ctx context.Context) (FileInfo, error) {
	return &baseFileInfo{name: f.name, path: f.name, size: f.size, modTime: time.Now()}, nil
}

type larkWriteFile struct {
	baseFile
	store *larkStorage
	name  string
	buf   *bytes.Buffer
}

func (f *larkWriteFile) Write(p []byte) (int, error) { return f.buf.Write(p) }
func (f *larkWriteFile) Close() error {
	f.store.writeBufsMu.Lock()
	f.store.writeBufs[f.name] = f.buf.Bytes()
	f.store.writeBufsMu.Unlock()
	return nil
}
func (f *larkWriteFile) String() string { return f.name }
