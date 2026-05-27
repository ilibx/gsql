CREATE TABLE tbl_cctv (
  id STRING,
  'date' STRING,
  title STRING,
  content STRING
) WITH (
  url = 'postgres://dev:dev123@192.168.2.12:5432/stock?sslmode=disable',
  table_name = "tbl_cctv"
);

SELECT * FROM tbl_cctv LIMIT 10;
