package storage

import (
	"testing"

	"github.com/ilibx/gsql/pkg/catalog"
)

// Mock S3 test - these are integration tests that require AWS credentials
// For now, we provide basic structure for S3 functionality

func TestS3StorageTypeDetection(t *testing.T) {
	tbl := &catalog.Table{
		Name: "test_s3_table",
		Columns: []catalog.ColumnDef{
			{Name: "id", Type: "INT"},
			{Name: "name", Type: "STRING"},
		},
		WithOptions: map[string]string{
			"storage":      "s3",
			"s3_bucket":    "test-bucket",
			"s3_region":    "us-east-1",
			"s3_prefix":    "data/",
			"format":       "csv",
			"file_pattern": "*.csv",
		},
	}

	// Verify storage type is correctly identified
	storageType := tbl.Option("storage", "local")
	if storageType != "s3" {
		t.Errorf("expected storage type 's3', got %q", storageType)
	}

	// Verify S3 options are present
	if bucket := tbl.Option("s3_bucket", ""); bucket != "test-bucket" {
		t.Errorf("expected bucket 'test-bucket', got %q", bucket)
	}
	if region := tbl.Option("s3_region", ""); region != "us-east-1" {
		t.Errorf("expected region 'us-east-1', got %q", region)
	}
	if prefix := tbl.Option("s3_prefix", ""); prefix != "data/" {
		t.Errorf("expected prefix 'data/', got %q", prefix)
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
			"s3_bucket":     "test-bucket",
			"s3_endpoint":   "http://localhost:9000",
			"format":        "csv",
			"file_pattern":  "*.csv",
		},
	}

	if endpoint := tbl.Option("s3_endpoint", ""); endpoint != "http://localhost:9000" {
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
			"storage":      "s3",
			"s3_bucket":    "test-bucket",
			"s3_prefix":    "output/",
			"format":       "csv",
			"file_name":    "result.csv",
		},
	}

	fileName := tbl.Option("file_name", "result.csv")
	if fileName != "result.csv" {
		t.Errorf("expected file_name 'result.csv', got %q", fileName)
	}
}
