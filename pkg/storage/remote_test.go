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
			"ftp_host":     "ftp.example.com",
			"ftp_port":     "21",
			"ftp_user":     "testuser",
			"ftp_pass":     "testpass",
			"ftp_path":     "/data",
			"format":       "csv",
			"file_pattern": "*.csv",
		},
	}

	storageType := tbl.Option("storage", "local")
	if storageType != "ftp" {
		t.Errorf("expected storage type 'ftp', got %q", storageType)
	}

	if host := tbl.Option("ftp_host", ""); host != "ftp.example.com" {
		t.Errorf("expected ftp_host 'ftp.example.com', got %q", host)
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
			"sftp_host":    "sftp.example.com",
			"sftp_port":    "22",
			"sftp_user":    "sftpuser",
			"sftp_pass":    "sftppass",
			"sftp_path":    "/home/user/data",
			"format":       "csv",
			"file_pattern": "*.csv",
		},
	}

	storageType := tbl.Option("storage", "local")
	if storageType != "sftp" {
		t.Errorf("expected storage type 'sftp', got %q", storageType)
	}

	if host := tbl.Option("sftp_host", ""); host != "sftp.example.com" {
		t.Errorf("expected sftp_host 'sftp.example.com', got %q", host)
	}
	if port := tbl.Option("sftp_port", "22"); port != "22" {
		t.Errorf("expected sftp_port '22', got %q", port)
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
			"webdav_url":    "http://webdav.example.com",
			"webdav_user":   "webdavuser",
			"webdav_pass":   "webdavpass",
			"webdav_path":   "/public/data",
			"format":        "csv",
			"file_pattern":  "*.csv",
		},
	}

	storageType := tbl.Option("storage", "local")
	if storageType != "webdav" {
		t.Errorf("expected storage type 'webdav', got %q", storageType)
	}

	if url := tbl.Option("webdav_url", ""); url != "http://webdav.example.com" {
		t.Errorf("expected webdav_url 'http://webdav.example.com', got %q", url)
	}
}

func TestRemoteStorageFileNameGeneration(t *testing.T) {
	testCases := []struct {
		name      string
		storage   string
		optionKey string
	}{
		{"FTP", "ftp", "ftp_path"},
		{"SFTP", "sftp", "sftp_path"},
		{"WebDAV", "webdav", "webdav_path"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := map[string]string{
				"storage":      tc.storage,
				"format":       "csv",
				"file_name":    "result.csv",
				tc.optionKey:   "/data",
			}

			// Set appropriate credential keys
			if tc.storage == "ftp" {
				opts["ftp_host"] = "example.com"
			} else if tc.storage == "sftp" {
				opts["sftp_host"] = "example.com"
			} else if tc.storage == "webdav" {
				opts["webdav_url"] = "http://example.com"
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
