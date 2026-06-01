CREATE TABLE users (
  id INT,
  name STRING,
  email STRING,
  age INT,
  city STRING
)
WITH (
  storage = 'local',
  format = 'csv',
  path = 'samples/users',
  file_pattern = 'users.csv'
);

SELECT id, name
FROM users
WHERE age >= 35
ORDER BY id;
