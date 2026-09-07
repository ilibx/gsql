-- ============================================================
--  mr_li 消耗明细分析（2026-08）
--
--  用法：
--    ./bin/gsql -s channel_bill.sql        -- 从 data/ 目录执行
--    ./bin/gsql -s data/channel_bill.sql   -- 从项目根目录执行
--
--  path='.' 在加载 SQL 文件时会自动解析为 SQL 脚本所在目录。
--
--  数据源：消耗明细0801-0831 sheet，含每条请求的 token 用量和费用。
--  输出：总账单（总体+错误）、每日汇总、月度总览。
-- ============================================================

-- ── 读取原始消耗明细 ────────────────────────────────────────
CREATE TABLE tbl_usage_detail (
  date             STRING,       -- 日期，格式 YYYY-MM-DD
  request_id       STRING,       -- 请求唯一 ID
  req_type         STRING,       -- 请求类型：正常 / 错误
  token            STRING,       -- API 令牌名称
  model_name       STRING,       -- 模型名称，如 gemini-3.1-flash-lite
  group_name       STRING,       -- 分组名称，如 gemini专属
  billing_tier     STRING,       -- 计费阶梯
  fee_multiplier   STRING,       -- 费用倍率（为空表示标准价）
  input_tokens     BIGINT,       -- 输入 Token 数量
  input_rate       DOUBLE,       -- 输入有效单价（USD/百万 Token）
  output_tokens    BIGINT,       -- 输出 Token 数量
  output_rate      DOUBLE,       -- 输出有效单价（USD/百万 Token）
  cache_read_tokens  BIGINT,      -- 缓存读取 Token 数量
  cache_read_rate    DOUBLE,      -- 缓存读取有效单价（USD/百万 Token）
  cache_write_tokens BIGINT,      -- 缓存写入通用 Token 数量
  cache_write_rate   DOUBLE,      -- 缓存写入通用有效单价（USD/百万 Token）
  cache_5m_tokens    BIGINT,      -- 缓存写入 5m Token 数量
  cache_5m_rate      DOUBLE,      -- 缓存写入 5m 有效单价（USD/百万 Token）
  cache_1h_tokens    BIGINT,      -- 缓存写入 1h Token 数量
  cache_1h_rate      DOUBLE,      -- 缓存写入 1h 有效单价（USD/百万 Token）
  cost_usd         DOUBLE        -- 该条请求的总费用（USD）
)
WITH (
  storage        = 'local',
  format         = 'xlsx',
  path           = '.',
  file_pattern   = '*.xlsx',
  sheet          = '消耗明细0801-0831',
  include_header = 'true'
);

-- ── 总账单（总体） ──────────────────────────────────────────
CREATE TABLE _bill_overall (
  section           STRING,       -- 标注"总体汇总"
  input_tokens      BIGINT,       -- 全部请求输入 Token 累计
  output_tokens     BIGINT,       -- 全部请求输出 Token 累计
  cache_read_tokens BIGINT,       -- 全部请求缓存读取 Token 累计
  cache_write_tokens BIGINT,      -- 全部请求缓存写入 Token 累计
  total_cost_usd    DOUBLE,       -- 全部请求总费用（USD）
  request_count     BIGINT        -- 全部请求总数
)
WITH (
  storage        = 'local',
  format         = 'csv',
  path           = 'data',
  file_name      = 'bill_overall.csv',
  include_header = 'true'
);

INSERT OVERWRITE TABLE _bill_overall
SELECT
  '总体汇总' AS section,
  SUM(input_tokens)  AS input_tokens,
  SUM(output_tokens) AS output_tokens,
  SUM(cache_read_tokens)  AS cache_read_tokens,
  SUM(cache_write_tokens) AS cache_write_tokens,
  SUM(cost_usd)      AS total_cost_usd,
  COUNT(*)           AS request_count
FROM tbl_usage_detail;

-- 错误请求数量（单独查询，SUM(CASE) 暂不支持）
CREATE TABLE _bill_error_count (
  section     STRING,      -- 标注"总体汇总"
  error_count BIGINT       -- 错误请求总数
)
WITH (
  storage        = 'local',
  format         = 'csv',
  path           = 'data',
  file_name      = 'bill_error_count.csv',
  include_header = 'true'
);

INSERT OVERWRITE TABLE _bill_error_count
SELECT
  '总体汇总' AS section,
  COUNT(*)   AS error_count
FROM tbl_usage_detail
WHERE req_type = '错误';

-- ── 总账单（错误请求） ─────────────────────────────────────
CREATE TABLE _bill_error (
  section           STRING,       -- 标注"错误请求"
  input_tokens      BIGINT,       -- 错误请求的输入 Token 累计
  output_tokens     BIGINT,       -- 错误请求的输出 Token 累计
  cache_read_tokens BIGINT,       -- 错误请求的缓存读取 Token 累计
  cache_write_tokens BIGINT,      -- 错误请求的缓存写入 Token 累计
  total_cost_usd    DOUBLE,       -- 错误请求总费用（USD）
  request_count     BIGINT        -- 错误请求总数
)
WITH (
  storage        = 'local',
  format         = 'csv',
  path           = 'data',
  file_name      = 'bill_error.csv',
  include_header = 'true'
);

INSERT OVERWRITE TABLE _bill_error
SELECT
  '错误请求' AS section,
  SUM(input_tokens)  AS input_tokens,
  SUM(output_tokens) AS output_tokens,
  SUM(cache_read_tokens)  AS cache_read_tokens,
  SUM(cache_write_tokens) AS cache_write_tokens,
  SUM(cost_usd)      AS total_cost_usd,
  COUNT(*)           AS request_count
FROM tbl_usage_detail
WHERE req_type = '错误';

-- ── 每日明细汇总 ────────────────────────────────────────────
CREATE TABLE _daily_summary (
  date               STRING,       -- 日期
  group_name         STRING,       -- 分组名称
  model_name         STRING,       -- 模型名称
  input_tokens       BIGINT,       -- 当日该分组该模型的输入 Token 累计
  output_tokens      BIGINT,       -- 当日该分组该模型的输出 Token 累计
  cache_read_tokens  BIGINT,       -- 当日该分组该模型的缓存读取 Token 累计
  cache_write_tokens BIGINT,       -- 当日该分组该模型的缓存写入 Token 累计
  total_cost_usd     DOUBLE,       -- 当日该分组该模型总费用（USD）
  request_count      BIGINT        -- 当日该分组该模型请求总数
)
WITH (
  storage        = 'local',
  format         = 'csv',
  path           = 'data',
  file_name      = 'daily_summary.csv',
  include_header = 'true'
);

INSERT OVERWRITE TABLE _daily_summary
SELECT
  date,
  group_name,
  model_name,
  SUM(input_tokens)  AS input_tokens,
  SUM(output_tokens) AS output_tokens,
  SUM(cache_read_tokens)  AS cache_read_tokens,
  SUM(cache_write_tokens) AS cache_write_tokens,
  SUM(cost_usd)          AS total_cost_usd,
  COUNT(*)               AS request_count
FROM tbl_usage_detail
GROUP BY date, group_name, model_name
ORDER BY date DESC, total_cost_usd DESC;

-- ── 月度总览 ────────────────────────────────────────────────
CREATE TABLE _monthly_summary (
  group_name         STRING,       -- 分组名称
  input_tokens       BIGINT,       -- 该分组全月输入 Token 累计
  output_tokens      BIGINT,       -- 该分组全月输出 Token 累计
  cache_read_tokens  BIGINT,       -- 该分组全月缓存读取 Token 累计
  cache_write_tokens BIGINT,       -- 该分组全月缓存写入 Token 累计
  total_cost_usd     DOUBLE,       -- 该分组全月总费用（USD）
  request_count      BIGINT        -- 该分组全月请求总数
)
WITH (
  storage        = 'local',
  format         = 'csv',
  path           = 'data',
  file_name      = 'monthly_summary.csv',
  include_header = 'true'
);

INSERT OVERWRITE TABLE _monthly_summary
SELECT
  group_name,
  SUM(input_tokens)  AS input_tokens,
  SUM(output_tokens) AS output_tokens,
  SUM(cache_read_tokens)  AS cache_read_tokens,
  SUM(cache_write_tokens) AS cache_write_tokens,
  SUM(cost_usd)          AS total_cost_usd,
  COUNT(*)               AS request_count
FROM tbl_usage_detail
GROUP BY group_name
ORDER BY total_cost_usd DESC;

-- ── 输出结果 ────────────────────────────────────────────────
SELECT '========== 总账单（总体） ==========' AS section;
SELECT * FROM _bill_overall;

SELECT '========== 总账单（错误请求） ==========' AS section;
SELECT * FROM _bill_error;

SELECT '========== 错误请求数 ==========' AS section;
SELECT * FROM _bill_error_count;

SELECT '========== 每日汇总 (按分组+模型) ==========' AS section;
SELECT * FROM _daily_summary;

SELECT '========== 月度总览 (按分组) ==========' AS section;
SELECT * FROM _monthly_summary;
