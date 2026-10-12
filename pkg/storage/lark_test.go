package storage

import (
	"testing"

	"github.com/ilibx/gsql/pkg/catalog"
)

func TestLarkStorageTypeDetection(t *testing.T) {
	tbl := &catalog.Table{
		Name: "test_lark_table",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
		},
		WithOptions: map[string]string{
			"storage":    "lark",
			"app_id":     "cli_xxx",
			"app_secret": "secret_xxx",
			"root_token": "folder_token_xxx",
			"format":     "csv",
		},
	}

	storageType := tbl.Option("storage", "local")
	if storageType != "lark" {
		t.Errorf("expected storage type 'lark', got %q", storageType)
	}
	if appID := tbl.Option("app_id", ""); appID != "cli_xxx" {
		t.Errorf("expected app_id 'cli_xxx', got %q", appID)
	}
	if appSecret := tbl.Option("app_secret", ""); appSecret != "secret_xxx" {
		t.Errorf("expected app_secret 'secret_xxx', got %q", appSecret)
	}
	if rootToken := tbl.Option("root_token", ""); rootToken != "folder_token_xxx" {
		t.Errorf("expected root_token 'folder_token_xxx', got %q", rootToken)
	}
}

func TestLarkStorageMissingCredentials(t *testing.T) {
	tbl := &catalog.Table{
		Name: "test_lark_no_creds",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage": "lark",
		},
	}

	_, err := newLarkStorage(tbl)
	if err == nil {
		t.Error("expected error for missing credentials, got nil")
	}
}

func TestLarkStorageMissingRootToken(t *testing.T) {
	tbl := &catalog.Table{
		Name: "test_lark_no_root",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":    "lark",
			"app_id":     "cli_xxx",
			"app_secret": "secret_xxx",
		},
	}

	_, err := newLarkStorage(tbl)
	if err == nil {
		t.Error("expected error for missing root (folder or root_token), got nil")
	}
}

func TestLarkStorageWithFolder(t *testing.T) {
	tbl := &catalog.Table{
		Name: "test_lark_folder",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":      "lark",
			"app_id":       "cli_xxx",
			"app_secret":   "secret_xxx",
			"path":         "gsql-data",
			"parent_token": "parent_folder_token",
		},
	}

	store, err := newLarkStorage(tbl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ls := store.(*larkStorage)
	if ls.folder != "gsql-data" {
		t.Errorf("expected folder 'gsql-data', got %q", ls.folder)
	}
}

func TestLarkStorageFolderWithoutParentToken(t *testing.T) {
	tbl := &catalog.Table{
		Name: "test_lark_folder_no_parent",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":     "lark",
			"app_id":      "cli_xxx",
			"app_secret":  "secret_xxx",
			"path":       "gsql-data",
			// no parent_token — resolveRoot will try to find/create at app root
		},
	}

	store, err := newLarkStorage(tbl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ls := store.(*larkStorage)
	if ls.folder != "gsql-data" {
		t.Errorf("expected folder 'gsql-data', got %q", ls.folder)
	}
}

func TestLarkStorageAlternativeOptionKeys(t *testing.T) {
	tbl := &catalog.Table{
		Name: "test_lark_alt_keys",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":        "lark",
			"lark_app_id":    "cli_xxx",
			"lark_app_secret": "secret_xxx",
			"lark_root_token": "folder_token_xxx",
			"lark_chat_id":   "oc_xxxxx",
			"path":    "gsql-data",
		},
	}

	if appID := tbl.Option("lark_app_id", ""); appID != "cli_xxx" {
		t.Errorf("expected lark_app_id 'cli_xxx', got %q", appID)
	}
	if chatID := tbl.Option("lark_chat_id", ""); chatID != "oc_xxxxx" {
		t.Errorf("expected lark_chat_id 'oc_xxxxx', got %q", chatID)
	}
	if p := tbl.Option("path", ""); p != "gsql-data" {
		t.Errorf("expected path 'gsql-data', got %q", p)
	}
}

func TestLarkStorageFolderNameWithParentToken(t *testing.T) {
	tbl := &catalog.Table{
		Name: "test_lark_folder_name_with_parent",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":      "lark",
			"app_id":       "cli_xxx",
			"app_secret":   "secret_xxx",
			"path":         "gsql-data",
			"parent_token": "parent_folder_token",
		},
	}

	store, err := newLarkStorage(tbl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ls := store.(*larkStorage)
	if ls.folder != "gsql-data" {
		t.Errorf("expected folder 'gsql-data', got %q", ls.folder)
	}
}

func TestLarkStorageFolderBackwardCompat(t *testing.T) {
	tbl := &catalog.Table{
		Name: "test_lark_folder_backward_compat",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":    "lark",
			"app_id":     "cli_xxx",
			"app_secret": "secret_xxx",
			"path":       "gsql-data",
			// no parent_token — resolveRoot will try to find/create at app root
		},
	}

	store, err := newLarkStorage(tbl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ls := store.(*larkStorage)
	if ls.folder != "gsql-data" {
		t.Errorf("expected folder 'gsql-data', got %q", ls.folder)
	}
}

func TestLarkStorageFolderNameMissingRoot(t *testing.T) {
	tbl := &catalog.Table{
		Name: "test_lark_folder_no_root",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":    "lark",
			"app_id":     "cli_xxx",
			"app_secret": "secret_xxx",
			// no path and no root_token
		},
	}

	_, err := newLarkStorage(tbl)
	if err == nil {
		t.Error("expected error when neither path nor root_token is set, got nil")
	}
}

func TestLarkChunkCount(t *testing.T) {
	block := larkDefaultBlockSize
	tests := []struct {
		size int
		want int
	}{
		{size: 0, want: 1},
		{size: 1, want: 1},
		{size: block, want: 1},
		{size: block + 1, want: 2},
		{size: 3 * block, want: 3},
		{size: 3*block + 7, want: 4},
		{size: 5 * block, want: 5},
	}
	for _, tt := range tests {
		if got := chunkCount(tt.size, block); got != tt.want {
			t.Errorf("chunkCount(%d, %d) = %d, want %d", tt.size, block, got, tt.want)
		}
	}
}

func TestLarkRetryableCodes(t *testing.T) {
	retryable := []int{larkCodeInternalError, larkCodeCanRetry, larkCodePlatformError}
	for _, code := range retryable {
		if !isRetryableLarkCode(code) {
			t.Errorf("expected code %d to be retryable", code)
		}
	}
	permanent := []int{1061002, 1061043, 1062009, 1061073}
	for _, code := range permanent {
		if isRetryableLarkCode(code) {
			t.Errorf("expected code %d to be non-retryable", code)
		}
	}
}

func TestLarkAPIErrorMessage(t *testing.T) {
	err := &larkAPIError{API: "lark upload", Code: 2200, Msg: "Internal Error"}
	want := "lark upload error code 2200: Internal Error"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}
