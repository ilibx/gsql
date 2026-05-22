-- Example: Using FTP, SFTP, and WebDAV storage adapters

-- FTP Example
CREATE TABLE ftp_users (
  id INT,
  name STRING,
  email STRING
)
WITH (
  storage = 'ftp',
  ftp_host = 'ftp.example.com',
  ftp_port = '21',
  ftp_user = 'ftpuser',
  ftp_pass = 'ftppass',
  ftp_path = '/data/users',
  format = 'csv',
  file_pattern = '*.csv'
);

-- SFTP Example
CREATE TABLE sftp_products (
  id INT,
  name STRING,
  price STRING
)
WITH (
  storage = 'sftp',
  sftp_host = 'sftp.example.com',
  sftp_port = '22',
  sftp_user = 'sftpuser',
  sftp_pass = 'sftppass',
  sftp_path = '/home/user/products',
  format = 'csv',
  file_pattern = '*.csv'
);

-- WebDAV Example
CREATE TABLE webdav_orders (
  id INT,
  user_id INT,
  amount STRING
)
WITH (
  storage = 'webdav',
  webdav_url = 'http://webdav.example.com',
  webdav_user = 'webdavuser',
  webdav_pass = 'webdavpass',
  webdav_path = '/public/orders',
  format = 'csv',
  file_pattern = '*.csv'
);

-- FTP Result table
CREATE TABLE ftp_result (
  id INT,
  name STRING
)
WITH (
  storage = 'ftp',
  ftp_host = 'ftp.example.com',
  ftp_port = '21',
  ftp_user = 'ftpuser',
  ftp_pass = 'ftppass',
  ftp_path = '/results',
  format = 'csv',
  file_name = 'filtered_users.csv'
);

-- Query FTP data
SELECT id, name FROM ftp_users WHERE id > 10;

-- Write results to FTP
INSERT OVERWRITE TABLE ftp_result
SELECT id, name FROM ftp_users WHERE name LIKE '%a%';
