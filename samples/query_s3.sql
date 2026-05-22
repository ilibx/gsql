-- Example: Using S3 storage adapter
-- This demonstrates how to use gsql with S3-compatible storage services

CREATE TABLE s3_users (
  id INT,
  name STRING,
  email STRING
)
WITH (
  storage = 's3',
  s3_bucket = 'my-data-bucket',
  s3_region = 'us-east-1',
  s3_prefix = 'users/',
  format = 'csv',
  file_pattern = '*.csv'
);

CREATE TABLE s3_result (
  id INT,
  name STRING
)
WITH (
  storage = 's3',
  s3_bucket = 'my-results-bucket',
  s3_region = 'us-east-1',
  s3_prefix = 'results/',
  format = 'csv',
  file_name = 'users_filtered.csv'
);

-- Query S3 data
SELECT id, name FROM s3_users WHERE name LIKE 'alice';

-- Write results back to S3
INSERT OVERWRITE TABLE s3_result
SELECT id, name FROM s3_users WHERE id > 5;
