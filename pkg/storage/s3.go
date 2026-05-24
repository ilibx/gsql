package storage

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/serde"
)

// S3 storage option keys (without s3_ prefix)
const (
	s3URLKey      = "url"
	s3RegionKey   = "region"
	s3EndpointKey = "endpoint"
	s3Timeout     = 30 * time.Second
)

// parseS3URL extracts bucket and prefix from an S3 URL like s3://bucket/prefix.
func parseS3URL(rawURL string) (bucket, prefix string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid S3 URL %q: %w", rawURL, err)
	}
	if u.Scheme != "s3" {
		return "", "", fmt.Errorf("expected s3:// URL, got %q", rawURL)
	}
	bucket = u.Host
	if bucket == "" {
		return "", "", fmt.Errorf("missing bucket in S3 URL %q", rawURL)
	}
	prefix = strings.TrimPrefix(u.Path, "/")
	return bucket, prefix, nil
}

// readS3Table reads data from S3
func readS3Table(tbl *catalog.Table, filters []PartitionFilter) ([]Row, error) {
	// Try to get URL from either 'url' or individual parameters
	rawURL := tbl.Option(s3URLKey, "")
	bucket := tbl.Option("bucket", "")
	prefix := tbl.Option("prefix", "")

	// If URL is provided, parse it to get bucket and prefix
	if rawURL != "" {
		parsedBucket, parsedPrefix, err := parseS3URL(rawURL)
		if err != nil {
			return nil, err
		}
		// Override individual parameters with URL values
		if bucket == "" {
			bucket = parsedBucket
		}
		if prefix == "" {
			prefix = parsedPrefix
		}
	}

	if bucket == "" {
		return nil, fmt.Errorf("missing bucket for S3 table %s", tbl.Name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), s3Timeout)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

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

	pattern := tbl.Option("file_pattern", "*")
	format := strings.ToLower(tbl.Option("format", ""))

	if format == "" {
		return nil, fmt.Errorf("missing format for table %s", tbl.Name)
	}

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
			if matchPattern(path.Base(key), pattern) {
				keys = append(keys, key)
			}
		}
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no files found in S3 bucket %s with prefix %s", bucket, prefix)
	}

	// Read files in parallel using streaming reads
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

			switch format {
			case "csv":
				csvOpts := serde.NewCSVOptions(tbl)
				rows, err := serde.Decode(ctx, "csv", obj.Body, tbl.Columns, csvOpts)
				resultCh <- fileResult{rows: rows, err: err}
			case "json":
				rows, err := serde.Decode(ctx, "json", obj.Body, tbl.Columns, serde.CSVOptions{})
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
	rawURL := tbl.Option(s3URLKey, "")
	if rawURL == "" {
		return fmt.Errorf("missing url for table %s", tbl.Name)
	}

	bucket, prefix, err := parseS3URL(rawURL)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), s3Timeout)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

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

	fileName := tbl.Option("file_name", "result.csv")
	format := strings.ToLower(tbl.Option("format", "csv"))
	csvOpts := serde.NewCSVOptions(tbl)

	if appendMode {
		fileName = fmt.Sprintf("append_%d_%s", len(rows), fileName)
	}

	key := path.Join(prefix, fileName)

	var data []byte
	switch format {
	case "csv":
		buf := &bytes.Buffer{}
		if err := serde.Encode(ctx, "csv", rows, tbl.Columns, buf, csvOpts); err != nil {
			return err
		}
		data = buf.Bytes()
	case "json":
		buf := &bytes.Buffer{}
		if err := serde.Encode(ctx, "json", rows, tbl.Columns, buf, serde.CSVOptions{}); err != nil {
			return err
		}
		data = buf.Bytes()
	default:
		return fmt.Errorf("unsupported write format %q", format)
	}

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
	matched, err := filepath.Match(pattern, name)
	return err == nil && matched
}
