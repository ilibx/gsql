-- 两张维度相同（request_id + model_name）的日志表，LEFT JOIN 后
-- 用 CASE WHEN 多条件判断多个字段是否存在差异，有差异置 1，无差异置 0
CREATE TABLE tbl_supply_logs (
  request_id STRING,
  model_name STRING,
  input      LONG,
  cache_read LONG
)
WITH (
  storage = 'local',
  format = 'csv',
  path = 'samples/case_diff',
  file_pattern = 'supply.csv'
);

CREATE TABLE tbl_platform_logs (
  request_id STRING,
  model_name STRING,
  input      LONG,
  cache_read LONG
)
WITH (
  storage = 'local',
  format = 'csv',
  path = 'samples/case_diff',
  file_pattern = 'platform.csv'
);

SELECT
  s.request_id,
  s.model_name,
  s.input,
  s.cache_read,
  l.input      AS plat_input,
  l.cache_read AS plat_cache_read,
  CASE
    WHEN s.request_id <> l.request_id
      OR s.model_name <> l.model_name
      OR s.input      <> l.input
      OR s.cache_read <> l.cache_read
    THEN 1
    ELSE 0
  END AS diff
FROM tbl_supply_logs s
LEFT JOIN tbl_platform_logs l ON l.request_id = s.request_id
ORDER BY s.request_id;
