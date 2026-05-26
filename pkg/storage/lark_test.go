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
			"folder":       "gsql-data",
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
			"folder":      "gsql-data",
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
			"lark_folder":    "gsql-data",
		},
	}

	if appID := tbl.Option("lark_app_id", ""); appID != "cli_xxx" {
		t.Errorf("expected lark_app_id 'cli_xxx', got %q", appID)
	}
	if chatID := tbl.Option("lark_chat_id", ""); chatID != "oc_xxxxx" {
		t.Errorf("expected lark_chat_id 'oc_xxxxx', got %q", chatID)
	}
	if folder := tbl.Option("lark_folder", ""); folder != "gsql-data" {
		t.Errorf("expected lark_folder 'gsql-data', got %q", folder)
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
			"folder_name": "gsql-data",
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
			"storage":     "lark",
			"app_id":      "cli_xxx",
			"app_secret":  "secret_xxx",
			"folder_name": "gsql-data",
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
			// no folder and no root_token
		},
	}

	_, err := newLarkStorage(tbl)
	if err == nil {
		t.Error("expected error when neither folder nor root_token is set, got nil")
	}
}
