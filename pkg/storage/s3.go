package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/ilibx/gsql/pkg/catalog"
)

const s3Timeout = 30 * time.Second

// parseS3URL extracts bucket and prefix from an S3 URL like s3://bucket/prefix.
// This is a fallback parser for tables created programmatically without going
// through CREATE TABLE (where catalog.go's parseStorageURL handles the new
// s3://endpoint/bucket/prefix format).
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

type s3Storage struct {
	client *s3.Client
	bucket string
	prefix string
}

func newS3Storage(tbl *catalog.Table) (Storage, error) {
	rawURL := tbl.Option("url", "")
	bucket := tbl.Option("bucket", "")
	prefix := tbl.Option("prefix", "")

	if rawURL != "" {
		parsedBucket, parsedPrefix, err := parseS3URL(rawURL)
		if err != nil {
			return nil, err
		}
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

	region := tbl.Option("region", "")
	if region == "" {
		region = tbl.Option("s3_region", "")
	}
	accessKey := tbl.Option("access_key", "")
	accessSecret := tbl.Option("access_secret", "")

	var loadOpts []func(*config.LoadOptions) error
	if region != "" {
		loadOpts = append(loadOpts, config.WithRegion(region))
	}
	if accessKey != "" && accessSecret != "" {
		loadOpts = append(loadOpts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, accessSecret, ""),
		))
	}
	if retryMode := retryModeFromOption(tbl); retryMode != "" {
		loadOpts = append(loadOpts, config.WithRetryMode(retryMode))
	}

	cfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	usePathStyle := tbl.Option("use_path_style", "") == "true" ||
		tbl.Option("s3_use_path_style", "") == "true"

	var client *s3.Client
	endpoint := tbl.Option("endpoint", "")
	if endpoint == "" {
		endpoint = tbl.Option("s3_endpoint", "")
	}
	if endpoint != "" {
		client = s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.BaseEndpoint = &endpoint
			o.UsePathStyle = usePathStyle
		})
	} else {
		client = s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.UsePathStyle = usePathStyle
		})
	}

	return &s3Storage{client: client, bucket: bucket, prefix: prefix}, nil
}

// retryModeFromOption returns the AWS retry mode from table options.
// Accepts: "standard", "adaptive" (case-insensitive).
func retryModeFromOption(tbl *catalog.Table) aws.RetryMode {
	val := tbl.Option("retry_mode", "")
	if val == "" {
		val = tbl.Option("s3_retry_mode", "")
	}
	switch strings.ToLower(val) {
	case "standard":
		return aws.RetryModeStandard
	case "adaptive":
		return aws.RetryModeAdaptive
	default:
		return ""
	}
}

func (s *s3Storage) resolveKey(name string) string {
	name = path.Clean(name)
	if name == "." || name == "" {
		return s.prefix
	}
	if s.prefix == "" {
		return name
	}
	return s.prefix + "/" + name
}

func (s *s3Storage) relKey(key string) string {
	if s.prefix == "" {
		return key
	}
	rel := strings.TrimPrefix(key, s.prefix+"/")
	if rel == key {
		rel = strings.TrimPrefix(key, s.prefix)
	}
	return strings.TrimPrefix(rel, "/")
}

func (s *s3Storage) Open(ctx context.Context, name string) (File, error) {
	key := s.resolveKey(name)
	obj, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open S3 object %s: %w", name, err)
	}
	return &s3ReadFile{baseFile: baseFile{}, body: obj.Body, key: key}, nil
}

func (s *s3Storage) Stat(ctx context.Context, name string) (FileInfo, error) {
	key := s.resolveKey(name)
	obj, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
	if err != nil {
		// Check if it's a "directory" prefix
		dirKey := key + "/"
		result, listErr := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:  &s.bucket,
			Prefix:  &dirKey,
			MaxKeys: aws.Int32(1),
		})
		if listErr != nil || len(result.Contents) == 0 {
			return nil, fmt.Errorf("failed to stat %s: %w", name, err)
		}
		return &baseFileInfo{
			name:  filepath.Base(name),
			path:  name,
			isDir: true,
		}, nil
	}
	return &baseFileInfo{
		name:    filepath.Base(name),
		path:    name,
		size:    uint64(*obj.ContentLength),
		modTime: *obj.LastModified,
	}, nil
}

func (s *s3Storage) Glob(ctx context.Context, pattern string) ([]string, error) {
	prefix := s.resolveKey("")
	// List all objects under the prefix
	var keys []string
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: &s.bucket,
		Prefix: &prefix,
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list S3 objects: %w", err)
		}
		for _, obj := range page.Contents {
			relKey := s.relKey(*obj.Key)
			if relKey == "" {
				continue
			}
			if matchPattern(path.Base(relKey), pattern) {
				keys = append(keys, relKey)
			}
		}
	}
	return keys, nil
}

func (s *s3Storage) List(ctx context.Context, dirPath string) ([]string, error) {
	key := s.resolveKey(dirPath)
	if key != "" {
		key += "/"
	}

	result, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:    &s.bucket,
		Prefix:    &key,
		Delimiter: aws.String("/"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list S3 objects: %w", err)
	}

	var entries []string
	for _, cp := range result.CommonPrefixes {
		rel := strings.TrimPrefix(*cp.Prefix, key)
		rel = strings.TrimSuffix(rel, "/")
		if strings.Contains(rel, "/") {
			continue
		}
		if dirPath == "" || dirPath == "." {
			entries = append(entries, rel)
		} else {
			entries = append(entries, filepath.Join(dirPath, rel))
		}
	}
	for _, obj := range result.Contents {
		rel := strings.TrimPrefix(*obj.Key, key)
		if rel == "" {
			continue
		}
		if strings.Contains(rel, "/") {
			continue
		}
		if dirPath == "" || dirPath == "." {
			entries = append(entries, rel)
		} else {
			entries = append(entries, filepath.Join(dirPath, rel))
		}
	}
	return entries, nil
}

func (s *s3Storage) Mkdir(ctx context.Context, name string, perm fs.FileMode) error {
	return nil
}

func (s *s3Storage) MkdirAll(ctx context.Context, name string, perm fs.FileMode) error {
	return nil
}

func (s *s3Storage) Remove(ctx context.Context, name string) error {
	key := s.resolveKey(name)
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
	return err
}

func (s *s3Storage) RemoveAll(ctx context.Context, name string) error {
	key := s.resolveKey(name)
	var toDelete []string
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: &s.bucket,
		Prefix: &key,
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}
		for _, obj := range page.Contents {
			toDelete = append(toDelete, *obj.Key)
		}
	}
	if len(toDelete) == 0 {
		return nil
	}
	// Delete in batches of 1000 (S3 limit)
	for i := 0; i < len(toDelete); i += 1000 {
		end := i + 1000
		if end > len(toDelete) {
			end = len(toDelete)
		}
		batch := toDelete[i:end]
		objects := make([]types.ObjectIdentifier, len(batch))
		for j, k := range batch {
			objects[j] = types.ObjectIdentifier{Key: &k}
		}
		_, err := s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: &s.bucket,
			Delete: &types.Delete{Objects: objects},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *s3Storage) WriteFile(ctx context.Context, name string, data []byte, perm fs.FileMode) error {
	key := s.resolveKey(name)
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
		Body:   bytes.NewReader(data),
	})
	return err
}

func (s *s3Storage) Rename(ctx context.Context, oldName, newName string) error {
	oldKey := s.resolveKey(oldName)
	newKey := s.resolveKey(newName)

	// S3 doesn't have rename; must copy then delete
	_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     &s.bucket,
		CopySource: aws.String(s.bucket + "/" + oldKey),
		Key:        &newKey,
	})
	if err != nil {
		return fmt.Errorf("failed to copy S3 object: %w", err)
	}
	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    &oldKey,
	})
	return err
}

func (s *s3Storage) Create(ctx context.Context, name string) (File, error) {
	key := s.resolveKey(name)
	return &s3WriteFile{
		baseFile: baseFile{},
		client:   s.client,
		bucket:   s.bucket,
		key:      key,
		buf:      &bytes.Buffer{},
	}, nil
}

func (s *s3Storage) Exists(ctx context.Context, name string) bool {
	key := s.resolveKey(name)
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &s.bucket,
		Key:    &key,
	})
	if err == nil {
		return true
	}
	// Check as directory prefix
	dirKey := key + "/"
	result, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  &s.bucket,
		Prefix:  &dirKey,
		MaxKeys: aws.Int32(1),
	})
	return err == nil && len(result.Contents) > 0
}

func (s *s3Storage) Join(elem ...string) string {
	return path.Join(elem...)
}

// s3ReadFile implements File for reading S3 objects.
type s3ReadFile struct {
	baseFile
	body io.ReadCloser
	key  string
}

func (f *s3ReadFile) Read(p []byte) (int, error)  { return f.body.Read(p) }
func (f *s3ReadFile) Close() error                 { return f.body.Close() }
func (f *s3ReadFile) String() string               { return f.key }

// s3WriteFile implements File for writing to S3 (buffered upload).
type s3WriteFile struct {
	baseFile
	client *s3.Client
	bucket string
	key    string
	buf    *bytes.Buffer
}

func (f *s3WriteFile) Write(p []byte) (int, error) { return f.buf.Write(p) }

func (f *s3WriteFile) Close() error {
	if f.buf == nil {
		return nil
	}
	_, err := f.client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: &f.bucket,
		Key:    &f.key,
		Body:   bytes.NewReader(f.buf.Bytes()),
	})
	f.buf = nil
	return err
}

func (f *s3WriteFile) String() string { return f.key }

// Helper functions

func matchPattern(name, pattern string) bool {
	if pattern == "*" || pattern == "*.*" {
		return true
	}
	matched, err := filepath.Match(pattern, name)
	return err == nil && matched
}
