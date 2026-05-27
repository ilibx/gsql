SELECT order_id, amount, year, month FROM partitioned_orders ORDER BY order_id;
SELECT order_id, amount, order_date FROM partitioned_orders WHERE year = 2026 AND month = '03';
SELECT order_id, amount, order_date, year, month FROM partitioned_orders WHERE year = 2026 AND month >= '05' ORDER BY month, order_id;
SELECT month, COUNT(*) AS order_count, SUM(amount) AS total_amount FROM partitioned_orders WHERE year = 2026 GROUP BY month ORDER BY month;
