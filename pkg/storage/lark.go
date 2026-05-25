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
	"path"
	"path/filepath"
	"sort"
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
	larkSendMsgAPI = "/open-apis/im/v1/messages?receive_id_type=chat_id"

	larkAuthTTL     = 90 * time.Minute
	larkHTTPTimeout = 30 * time.Second
)

type larkStorage struct {
	appID     string
	appSecret string
	rootToken string
	chatID    string

	mu        sync.Mutex
	token     string
	tokenExp  time.Time
	httpCli   *http.Client
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

	rootToken := tbl.Option("root_token", "")
	if rootToken == "" {
		rootToken = tbl.Option("lark_root_token", "")
		rawURL := tbl.Option("url", "")
		if rootToken == "" && rawURL != "" {
			if u, err := url.Parse(rawURL); err == nil && u.Scheme == "lark" {
				rootToken = u.Host
			}
		}
	}
	if rootToken == "" {
		return nil, fmt.Errorf("missing root_token for Lark table %s", tbl.Name)
	}

	chatID := tbl.Option("chat_id", "")
	if chatID == "" {
		chatID = tbl.Option("lark_chat_id", "")
	}

	return &larkStorage{
		appID:     appID,
		appSecret: appSecret,
		rootToken: rootToken,
		chatID:    chatID,
		httpCli:   &http.Client{Timeout: larkHTTPTimeout},
	}, nil
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

type larkMetaResp struct {
	Code int `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Token string `json:"token"`
		Name  string `json:"name"`
		Type  string `json:"type"`
		Size  int    `json:"size"`
		URL   string `json:"url"`
		ParentToken string `json:"parent_token"`
	} `json:"data"`
}

type larkCreateFolderResp struct {
	Code int `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		NodeToken string `json:"node_token"`
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

// ---- path resolution ----

func (s *larkStorage) listDir(ctx context.Context, parentToken string) ([]larkFileEntry, error) {
	var all []larkFileEntry
	pageToken := ""
	for {
		u := fmt.Sprintf("%s%s?parent_node_token=%s&page_size=100", larkBaseURL, larkDriveAPI, parentToken)
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

func (s *larkStorage) resolveChildToken(ctx context.Context, parentToken, childName string) (string, bool, error) {
	entries, err := s.listDir(ctx, parentToken)
	if err != nil {
		return "", false, err
	}
	for _, e := range entries {
		if e.Name == childName {
			return e.Token, e.Type == "folder", nil
		}
	}
	return "", false, nil
}

func (s *larkStorage) getFileMeta(ctx context.Context, token string) (*larkFileEntry, error) {
	u := fmt.Sprintf("%s%s/%s", larkBaseURL, larkDriveAPI, token)
	data, err := s.doGet(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp larkMetaResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("lark meta decode failed: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("lark meta error code %d: %s", resp.Code, resp.Msg)
	}
	return &larkFileEntry{
		Token: resp.Data.Token,
		Name:  resp.Data.Name,
		Type:  resp.Data.Type,
		Size:  resp.Data.Size,
		URL:   resp.Data.URL,
		ParentToken: resp.Data.ParentToken,
	}, nil
}

func (s *larkStorage) resolvePath(ctx context.Context, name string) (token string, isFolder bool, err error) {
	name = strings.TrimPrefix(name, "/")
	name = strings.TrimSuffix(name, "/")
	if name == "" {
		return s.rootToken, true, nil
	}
	parts := strings.Split(name, "/")
	currentToken := s.rootToken
	for i, part := range parts {
		t, isDir, err := s.resolveChildToken(ctx, currentToken, part)
		if err != nil {
			return "", false, err
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

func (s *larkStorage) ensureFolder(ctx context.Context, parentToken, name string) (string, error) {
	t, isDir, err := s.resolveChildToken(ctx, parentToken, name)
	if err != nil {
		return "", err
	}
	if t != "" && isDir {
		return t, nil
	}
	body, _ := json.Marshal(map[string]string{
		"name":             name,
		"parent_node_token": parentToken,
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
	return resp.Data.NodeToken, nil
}

func (s *larkStorage) ensurePath(ctx context.Context, name string) (parentToken, fileName string, err error) {
	name = strings.TrimPrefix(name, "/")
	name = strings.TrimSuffix(name, "/")
	dir, file := path.Split(name)
	if dir == "" {
		return s.rootToken, file, nil
	}
	dir = strings.TrimSuffix(dir, "/")
	parts := strings.Split(dir, "/")
	currentToken := s.rootToken
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
	token, _, err := s.resolvePath(ctx, name)
	if err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s%s/%s/download", larkBaseURL, larkDriveAPI, token)
	data, err := s.doGet(ctx, u)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			FileToken string `json:"file_token"`
			FileName  string `json:"file_name"`
			Size      int    `json:"size"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err == nil && resp.Code != 0 {
		return nil, fmt.Errorf("lark download error code %d: %s", resp.Code, resp.Msg)
	}
	meta, err := s.getFileMeta(ctx, token)
	if err != nil {
		return nil, err
	}
	return &larkReadFile{
		baseFile: baseFile{},
		reader:   bytes.NewReader(data),
		name:     name,
		size:     uint64(meta.Size),
	}, nil
}

func (s *larkStorage) Stat(ctx context.Context, name string) (FileInfo, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	token, isDir, err := s.resolvePath(ctx, name)
	if err != nil {
		return nil, err
	}
	meta, err := s.getFileMeta(ctx, token)
	if err != nil {
		return nil, err
	}
	return &baseFileInfo{
		name:  meta.Name,
		path:  name,
		size:  uint64(meta.Size),
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
		parentToken = s.rootToken
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
		parentToken = s.rootToken
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
	parts := strings.Split(name, "/")
	parentToken := s.rootToken
	for i := 0; i < len(parts)-1; i++ {
		var err error
		parentToken, err = s.ensureFolder(ctx, parentToken, parts[i])
		if err != nil {
			return err
		}
	}
	_, err := s.ensureFolder(ctx, parentToken, parts[len(parts)-1])
	return err
}

func (s *larkStorage) MkdirAll(ctx context.Context, name string, perm fs.FileMode) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	name = strings.TrimSuffix(strings.TrimPrefix(name, "/"), "/")
	if name == "" {
		return nil
	}
	currentToken := s.rootToken
	for _, part := range strings.Split(name, "/") {
		var err error
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
	token, _, err := s.resolvePath(ctx, name)
	if err != nil {
		return err
	}
	u := fmt.Sprintf(larkBaseURL+larkDeleteAPI, token)
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

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField("file_name", fileName)
	w.WriteField("parent_type", "explorer")
	w.WriteField("parent_node", parentToken)
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
		_ = s.shareFile(ctx, resp.Data.FileToken, s.chatID)
	}

	return nil
}

func (s *larkStorage) Rename(ctx context.Context, oldName, newName string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	token, _, err := s.resolvePath(ctx, oldName)
	if err != nil {
		return err
	}
	_, newFileName := path.Split(newName)
	body, _ := json.Marshal(map[string]string{"name": newFileName})
	u := fmt.Sprintf(larkBaseURL+larkUpdateAPI, token)
	data, err := s.doPatch(ctx, u, body)
	if err != nil {
		return err
	}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("lark rename decode failed: %w", err)
	}
	if resp.Code != 0 {
		return fmt.Errorf("lark rename error code %d: %s", resp.Code, resp.Msg)
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

// ---- Share to chat ----

type larkMsgContent struct {
	FileToken string `json:"file_token"`
}

func (s *larkStorage) shareFile(ctx context.Context, fileToken, chatID string) error {
	content, _ := json.Marshal(larkMsgContent{FileToken: fileToken})
	body, _ := json.Marshal(map[string]string{
		"receive_id": chatID,
		"msg_type":   "file",
		"content":    string(content),
	})
	data, err := s.doPost(ctx, larkBaseURL+larkSendMsgAPI, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("lark share request failed: %w", err)
	}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("lark share decode failed: %w", err)
	}
	if resp.Code != 0 {
		return fmt.Errorf("lark share error code %d: %s", resp.Code, resp.Msg)
	}
	return nil
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
	return f.store.WriteFile(context.Background(), f.name, f.buf.Bytes(), 0o644)
}
func (f *larkWriteFile) String() string { return f.name }
