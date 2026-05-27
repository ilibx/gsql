CREATE TABLE lark_data (
  surply_name string,
  surply_id string,
  customer_name string,
  customer_id string,
  discount float
)
WITH (
  storage = 'lark',
  app_id = 'cli_aa99536cb9399e15',
  app_secret = 'gmZTJxY4x9jFeuLrjINIbczgViZqt7LJ',
  folder = 'gsql-data',     
  chat_id = 'oc_94033dd71120221b4696076ab6811540',
  format = 'excel',
  include_header = 'true',
  skip_lines = "1",
  file_name = '供应商-客户'
);

SELECT * FROM 
lark_data;
