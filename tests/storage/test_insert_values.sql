-- Test INSERT INTO ... VALUES
CREATE TABLE users (
  id INT,
  name STRING,
  email STRING
)
WITH (
  storage = 'local',
  format = 'csv',
  path = '/tmp/users',
  file_name = 'users.csv'
);

-- Insert single row (overwrite)
INSERT OVERWRITE TABLE users VALUES (1, 'Alice', 'alice@example.com');

-- Insert multiple rows (append)
INSERT INTO users VALUES 
  (2, 'Bob', 'bob@example.com'),
  (3, 'Charlie', 'charlie@example.com');

-- Verify
SELECT * FROM users ORDER BY id;