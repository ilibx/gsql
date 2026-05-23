-- skip first 2 header lines, pipe delimiter
CREATE TABLE header_scores (
  id INT,
  name STRING,
  score INT
)
WITH (
  storage = 'local',
  format = 'csv',
  location = 'samples/csv_opts',
  file_pattern = 'with_header.csv',
  delimiter = '|',
  skip_header_lines = '2'
);

SELECT name, score FROM header_scores ORDER BY id;
