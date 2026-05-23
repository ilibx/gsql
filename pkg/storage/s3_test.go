package storage

import (
	"testing"

	"github.com/ilibx/gsql/pkg/catalog"
)

func TestS3StorageTypeDetection(t *testing.T) {
	tbl := &catalog.Table{
		Name: "test_s3_table",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
		},
		WithOptions: map[string]string{
			"storage":      "s3",
			"url":          "s3://test-bucket/data/",
			"region":       "us-east-1",
			"format":       "csv",
			"file_pattern": "*.csv",
		},
	}

	storageType := tbl.Option("storage", "local")
	if storageType != "s3" {
		t.Errorf("expected storage type 's3', got %q", storageType)
	}

	if u := tbl.Option("url", ""); u != "s3://test-bucket/data/" {
		t.Errorf("expected url 's3://test-bucket/data/', got %q", u)
	}
	if region := tbl.Option("region", ""); region != "us-east-1" {
		t.Errorf("expected region 'us-east-1', got %q", region)
	}
}

func TestS3OptionalEndpoint(t *testing.T) {
	tbl := &catalog.Table{
		Name: "test_s3_compatible",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":       "s3",
			"url":           "s3://test-bucket/",
			"endpoint":      "http://localhost:9000",
			"format":        "csv",
			"file_pattern":  "*.csv",
		},
	}

	if endpoint := tbl.Option("endpoint", ""); endpoint != "http://localhost:9000" {
		t.Errorf("expected endpoint 'http://localhost:9000', got %q", endpoint)
	}
}

func TestS3FileNameGeneration(t *testing.T) {
	tbl := &catalog.Table{
		Name: "test_table",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
		},
		WithOptions: map[string]string{
			"storage":   "s3",
			"url":       "s3://test-bucket/output/",
			"format":    "csv",
			"file_name": "result.csv",
		},
	}

	fileName := tbl.Option("file_name", "result.csv")
	if fileName != "result.csv" {
		t.Errorf("expected file_name 'result.csv', got %q", fileName)
	}
}
