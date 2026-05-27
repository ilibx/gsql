CREATE TABLE lark_data (
  id INT,
  name STRING
)
WITH (
  storage = 'lark',
  app_id = 'cli_aa9915',
  app_secret = 'gmZTViZqt7LJ',
  folder = 'gsql-data',     
  chat_id = 'oc_94036811540',
  format = 'csv',
  include_header = 'true',
  file_pattern = '*.csv'
);

INSERT OVERWRITE TABLE lark_data
SELECT id, name FROM (
  VALUES
     (1, 'zhangsan'), 
     (200, 'lisi')
) as t(id, name)
WHERE id > 100;
