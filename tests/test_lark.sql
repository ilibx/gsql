CREATE TABLE lark_data (
  id INT,
  name STRING
)
WITH (
  storage = 'lark',
  app_id = 'cli_aab9399e15',
  app_secret = 'gmZTJNIbczgViZqt7LJ',
  folder = 'gsql-data',     
  chat_id = 'oc_940331b4696076ab6811540',
  format = 'csv',
  file_pattern = '*.csv'
);

INSERT OVERWRITE TABLE lark_data
SELECT id, name FROM (
  VALUES
     (1, 'zhangsan'), 
     (200, 'lisi')
) as t(id, name)
WHERE id > 100;
