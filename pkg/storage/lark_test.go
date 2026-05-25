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
		t.Error("expected error for missing root (folder_name or root_token), got nil")
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
		},
	}

	if appID := tbl.Option("lark_app_id", ""); appID != "cli_xxx" {
		t.Errorf("expected lark_app_id 'cli_xxx', got %q", appID)
	}
	if chatID := tbl.Option("lark_chat_id", ""); chatID != "oc_xxxxx" {
		t.Errorf("expected lark_chat_id 'oc_xxxxx', got %q", chatID)
	}
}

func TestLarkStorageWithFolderName(t *testing.T) {
	tbl := &catalog.Table{
		Name: "test_lark_folder_name",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":     "lark",
			"app_id":      "cli_xxx",
			"app_secret":  "secret_xxx",
			"folder_name": "gsql-data",
		},
	}

	store, err := newLarkStorage(tbl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ls := store.(*larkStorage)
	if ls.folderName != "gsql-data" {
		t.Errorf("expected folder_name 'gsql-data', got %q", ls.folderName)
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
			// no folder_name and no root_token
		},
	}

	_, err := newLarkStorage(tbl)
	if err == nil {
		t.Error("expected error when neither folder_name nor root_token is set, got nil")
	}
}
