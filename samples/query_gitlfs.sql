-- Example: Using Git LFS storage adapter
-- Git LFS is useful for version-controlling large files in Git repositories

CREATE TABLE gitlfs_data (
  id INT,
  name STRING,
  value STRING
)
WITH (
  storage = 'git-lfs',
  git_lfs_repo = '/path/to/git/repo',
  format = 'csv',
  file_pattern = '*.csv'
);

CREATE TABLE gitlfs_result (
  id INT,
  name STRING
)
WITH (
  storage = 'gitlfs',
  git_lfs_path = '/lfs/objects',
  format = 'csv',
  file_name = 'processed.csv'
);

-- Query Git LFS managed data
SELECT id, name FROM gitlfs_data WHERE value > '100';

-- Write results to Git LFS
INSERT OVERWRITE TABLE gitlfs_result
SELECT id, name FROM gitlfs_data WHERE name LIKE 'data_%';
