package storage

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ilibx/gsql/pkg/catalog"
)

// S3 storage constants
const (
	s3BucketKey    = "s3_bucket"
	s3RegionKey    = "s3_region"
	s3EndpointKey  = "s3_endpoint"
	s3PrefixKey    = "s3_prefix"
	s3AccessKeyKey = "s3_access_key"
	s3SecretKeyKey = "s3_secret_key"
	s3Timeout      = 30 * time.Second
)

// readS3Table reads data from S3
func readS3Table(tbl *catalog.Table, filters []PartitionFilter) ([]Row, error) {
	bucket := tbl.Option(s3BucketKey, "")
	if bucket == "" {
		return nil, fmt.Errorf("missing s3_bucket for table %s", tbl.Name)
	}

	// Create context with timeout for S3 operations
	ctx, cancel := context.WithTimeout(context.Background(), s3Timeout)
	defer cancel()

	// Create S3 client
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Override region if provided
	if region := tbl.Option(s3RegionKey, ""); region != "" {
		cfg.Region = region
	}

	// Override endpoint if provided (for S3-compatible services)
	var s3Client *s3.Client
	if endpoint := tbl.Option(s3EndpointKey, ""); endpoint != "" {
		opts := []func(*s3.Options){
			func(o *s3.Options) {
				o.BaseEndpoint = &endpoint
			},
		}
		s3Client = s3.NewFromConfig(cfg, opts...)
	} else {
		s3Client = s3.NewFromConfig(cfg)
	}

	prefix := tbl.Option(s3PrefixKey, "")
	pattern := tbl.Option("file_pattern", "*")
	format := strings.ToLower(tbl.Option("format", ""))

	if format == "" {
		return nil, fmt.Errorf("missing format for table %s", tbl.Name)
	}

	// List objects in S3
	var keys []string
	paginator := s3.NewListObjectsV2Paginator(s3Client, &s3.ListObjectsV2Input{
		Bucket: &bucket,
		Prefix: &prefix,
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list S3 objects: %w", err)
		}

		for _, obj := range page.Contents {
			key := *obj.Key
			if matchPattern(filepath.Base(key), pattern) {
				keys = append(keys, key)
			}
		}
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no files found in S3 bucket %s with prefix %s", bucket, prefix)
	}

	// Read files in parallel
	type fileResult struct {
		rows []Row
		err  error
	}

	resultCh := make(chan fileResult, len(keys))
	for _, key := range keys {
		go func(key string) {
			obj, err := s3Client.GetObject(ctx, &s3.GetObjectInput{
				Bucket: &bucket,
				Key:    &key,
			})
			if err != nil {
				resultCh <- fileResult{err: fmt.Errorf("failed to get S3 object %s: %w", key, err)}
				return
			}
			defer obj.Body.Close()

			// Read object body into memory
			body, err := io.ReadAll(obj.Body)
			if err != nil {
				resultCh <- fileResult{err: fmt.Errorf("failed to read S3 object %s: %w", key, err)}
				return
			}

			switch format {
			case "csv":
				rows, err := readCSVFromBytes(body, tbl.Columns)
				resultCh <- fileResult{rows: rows, err: err}
			case "json":
				rows, err := readJSONFromBytes(body, tbl.Columns)
				resultCh <- fileResult{rows: rows, err: err}
			default:
				resultCh <- fileResult{err: fmt.Errorf("unsupported format %q", format)}
			}
		}(key)
	}

	var rows []Row
	for i := 0; i < len(keys); i++ {
		result := <-resultCh
		if result.err != nil {
			return nil, result.err
		}
		rows = append(rows, result.rows...)
	}

	return rows, nil
}

// writeS3Table writes data to S3
func writeS3Table(tbl *catalog.Table, rows []Row, appendMode bool) error {
	bucket := tbl.Option(s3BucketKey, "")
	if bucket == "" {
		return fmt.Errorf("missing s3_bucket for table %s", tbl.Name)
	}

	// Create context with timeout for S3 operations
	ctx, cancel := context.WithTimeout(context.Background(), s3Timeout)
	defer cancel()

	// Create S3 client
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Override region if provided
	if region := tbl.Option(s3RegionKey, ""); region != "" {
		cfg.Region = region
	}

	var s3Client *s3.Client
	if endpoint := tbl.Option(s3EndpointKey, ""); endpoint != "" {
		opts := []func(*s3.Options){
			func(o *s3.Options) {
				o.BaseEndpoint = &endpoint
			},
		}
		s3Client = s3.NewFromConfig(cfg, opts...)
	} else {
		s3Client = s3.NewFromConfig(cfg)
	}

	prefix := tbl.Option(s3PrefixKey, "")
	fileName := tbl.Option("file_name", "result.csv")
	format := strings.ToLower(tbl.Option("format", "csv"))

	// If append mode, generate a unique filename
	if appendMode {
		fileName = fmt.Sprintf("append_%d_%s", len(rows), fileName)
	}

	key := filepath.Join(prefix, fileName)

	var data []byte
	switch format {
	case "csv":
		buf := &bytes.Buffer{}
		if err := writeCSVToBuffer(buf, tbl.Columns, rows); err != nil {
			return err
		}
		data = buf.Bytes()
	case "json":
		buf := &bytes.Buffer{}
		if err := writeJSONToBuffer(buf, rows); err != nil {
			return err
		}
		data = buf.Bytes()
	default:
		return fmt.Errorf("unsupported write format %q", format)
	}

	// Upload to S3
	uploader := manager.NewUploader(s3Client)
	_, err = uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
		Body:   bytes.NewReader(data),
	})

	if err != nil {
		return fmt.Errorf("failed to write to S3: %w", err)
	}

	return nil
}

// Helper functions

func matchPattern(name, pattern string) bool {
	if pattern == "*" || pattern == "*.*" {
		return true
	}
	// Simple glob pattern matching
	matched, err := filepath.Match(pattern, name)
	return err == nil && matched
}

func readCSVFromBytes(data []byte, columns []catalog.ColumnDef) ([]Row, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.TrimLeadingSpace = true

	var rows []Row
	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		row := make(Row)
		for i, col := range columns {
			var value string
			if i < len(record) {
				value = record[i]
			}
			row[col.Name] = value
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func readJSONFromBytes(data []byte, columns []catalog.ColumnDef) ([]Row, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var rows []Row
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		rowMap := make(map[string]any)
		if err := json.Unmarshal([]byte(line), &rowMap); err != nil {
			return nil, err
		}
		row := make(Row)
		for _, col := range columns {
			if value, ok := rowMap[col.Name]; ok {
				row[col.Name] = fmt.Sprint(value)
			} else {
				row[col.Name] = ""
			}
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

func writeCSVToBuffer(buf io.Writer, columns []catalog.ColumnDef, rows []Row) error {
	writer := csv.NewWriter(buf)
	defer writer.Flush()

	for _, row := range rows {
		record := make([]string, len(columns))
		for i, col := range columns {
			record[i] = row[col.Name]
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	return nil
}

func writeJSONToBuffer(buf io.Writer, rows []Row) error {
	for _, row := range rows {
		data, err := json.Marshal(row)
		if err != nil {
			return err
		}
		if _, err := buf.Write(data); err != nil {
			return err
		}
		if _, err := buf.Write([]byte("\n")); err != nil {
			return err
		}
	}
	return nil
}
