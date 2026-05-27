-- Test URL-based storage configuration

-- Test local URL
CREATE TABLE local_data_url (
  id INT,
  name STRING
)
WITH (
  url = 'local:///tmp/test_data',
  format = 'csv'
);

-- Test FTP URL
CREATE TABLE ftp_data_url (
  id INT,
  name STRING
)
WITH (
  url = 'ftp://testuser:testpass@ftp.example.com:21/test',
  format = 'csv'
);

-- Test S3 URL
CREATE TABLE s3_data_url (
  id INT,
  name STRING
)
WITH (
  url = 's3://test-bucket/data/path?region=us-east-1&endpoint=s3.example.com',
  format = 'csv'
);

-- Test SFTP URL
CREATE TABLE sftp_data_url (
  id INT,
  name STRING
)
WITH (
  url = 'sftp://testuser:testpass@sftp.example.com/test',
  format = 'csv'
);

-- Test WebDAV URL
CREATE TABLE webdav_data_url (
  id INT,
  name STRING
)
WITH (
  url = 'webdav://testuser:testpass@webdav.example.com/data',
  format = 'csv'
);

-- Test Git LFS URL
CREATE TABLE gitlfs_data_url (
  id INT,
  name STRING
)
WITH (
  url = 'gitlfs:///path/to/git/repo',
  format = 'csv'
);

-- Test mixed mode (URL + override parameters)
CREATE TABLE ftp_mixed_url (
  id INT,
  name STRING
)
WITH (
  url = 'ftp://olduser:oldpass@ftp.example.com:21/test',
  username = 'custom_user',
  password = 'custom_pass',
  format = 'csv'
);

-- Verify tables were created
SELECT 'ftp_data_url' AS table_name UNION ALL
SELECT 's3_data_url' UNION ALL
SELECT 'sftp_data_url' UNION ALL
SELECT 'webdav_data_url' UNION ALL
SELECT 'gitlfs_data_url' UNION ALL
SELECT 'ftp_mixed_url';
