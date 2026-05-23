-- 创建分区结果表（year 是分区列，不在数据列中重复）
CREATE TABLE monthly_summary (
  month STRING,
  order_count INT,
  total_amount INT
)
WITH (
  storage = 'local',
  format = 'csv',
  location = 'samples/result/monthly',
  file_name = 'summary.csv'
)
PARTITIONED BY (year);

-- INSERT OVERWRITE：按 year 分区自动写入 samples/result/monthly/year=2026/summary.csv
INSERT OVERWRITE TABLE monthly_summary
SELECT month, COUNT(*) AS order_count, SUM(amount) AS total_amount, year
FROM partitioned_orders
GROUP BY month, year;

-- 读回验证
SELECT month, order_count, total_amount, year FROM monthly_summary ORDER BY month;
