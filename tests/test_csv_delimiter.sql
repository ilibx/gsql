-- custom pipe delimiter
CREATE TABLE pipe_users (
  id INT,
  name STRING,
  email STRING
)
WITH (
  storage = 'local',
  format = 'csv',
  location = 'samples/csv_opts',
  file_pattern = 'pipe_delim.csv',
  delimiter = '|'
);

SELECT name FROM pipe_users ORDER BY id;
