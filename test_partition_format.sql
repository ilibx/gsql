-- Test partition_format = 'value' option
CREATE TABLE monthly_summary_bare (
  month STRING,
  order_count INT,
  total_amount INT
)
WITH (
  storage = 'local',
  format = 'csv',
  location = 'samples/result/monthly_bare',
  file_name = 'summary.csv',
  partition_format = 'value'
)
PARTITIONED BY (year);

-- INSERT OVERWRITE: should create directories like samples/result/monthly_bare/2026/summary.csv
INSERT OVERWRITE TABLE monthly_summary_bare
SELECT month, COUNT(*) AS order_count, SUM(amount) AS total_amount, year
FROM partitioned_orders
GROUP BY month, year;

-- Read back to verify
SELECT month, order_count, total_amount, year FROM monthly_summary_bare ORDER BY month;