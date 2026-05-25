package storage

import (
	"testing"

	"github.com/ilibx/gsql/pkg/catalog"
)

// Mock FTP/SFTP/WebDAV tests - these are integration tests that require actual servers
// For now, we provide basic structure for remote storage functionality

func TestFTPStorageTypeDetection(t *testing.T) {
	tbl := &catalog.Table{
		Name: "test_ftp_table",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
		},
		WithOptions: map[string]string{
			"storage":      "ftp",
			"url":          "ftp://ftp.example.com:21/data",
			"username":     "testuser",
			"password":     "testpass",
			"format":       "csv",
			"file_pattern": "*.csv",
		},
	}

	storageType := tbl.Option("storage", "local")
	if storageType != "ftp" {
		t.Errorf("expected storage type 'ftp', got %q", storageType)
	}

	if u := tbl.Option("url", ""); u != "ftp://ftp.example.com:21/data" {
		t.Errorf("expected url 'ftp://ftp.example.com:21/data', got %q", u)
	}
}

func TestSFTPStorageTypeDetection(t *testing.T) {
	tbl := &catalog.Table{
		Name: "test_sftp_table",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":      "sftp",
			"url":          "sftp://sftp.example.com:22/home/user/data",
			"username":     "sftpuser",
			"password":     "sftppass",
			"format":       "csv",
			"file_pattern": "*.csv",
		},
	}

	storageType := tbl.Option("storage", "local")
	if storageType != "sftp" {
		t.Errorf("expected storage type 'sftp', got %q", storageType)
	}

	if u := tbl.Option("url", ""); u != "sftp://sftp.example.com:22/home/user/data" {
		t.Errorf("expected url 'sftp://sftp.example.com:22/home/user/data', got %q", u)
	}
}

func TestWebDAVStorageTypeDetection(t *testing.T) {
	tbl := &catalog.Table{
		Name: "test_webdav_table",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":       "webdav",
			"url":           "http://webdav.example.com",
			"username":      "webdavuser",
			"password":      "webdavpass",
			"path":          "/public/data",
			"format":        "csv",
			"file_pattern":  "*.csv",
		},
	}

	storageType := tbl.Option("storage", "local")
	if storageType != "webdav" {
		t.Errorf("expected storage type 'webdav', got %q", storageType)
	}

	if u := tbl.Option("url", ""); u != "http://webdav.example.com" {
		t.Errorf("expected url 'http://webdav.example.com', got %q", u)
	}
}

func TestRemoteStorageFileNameGeneration(t *testing.T) {
	testCases := []struct {
		name    string
		storage string
		url     string
	}{
		{"FTP", "ftp", "ftp://example.com/data"},
		{"SFTP", "sftp", "sftp://example.com/data"},
		{"WebDAV", "webdav", "http://example.com"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := map[string]string{
				"storage":   tc.storage,
				"format":    "csv",
				"file_name": "result.csv",
				"url":       tc.url,
			}

			tbl := &catalog.Table{
				Name: "test",
				Columns: []catalog.ColumnDef{
					{Name: "id", Type: "INT"},
				},
				WithOptions: opts,
			}

			fileName := tbl.Option("file_name", "result.csv")
			if fileName != "result.csv" {
				t.Errorf("expected file_name 'result.csv', got %q", fileName)
			}
		})
	}
}
