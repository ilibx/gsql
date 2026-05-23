CREATE TABLE partitioned_orders (order_id INT, user_id INT, product_id INT, quantity INT, amount INT, order_date STRING) WITH (storage = 'local', format = 'csv', location = 'samples/orders', file_pattern = 'data.csv') PARTITIONED BY (year, month);
SELECT order_id, amount, year, month FROM partitioned_orders ORDER BY order_id;
SELECT order_id, amount, order_date FROM partitioned_orders WHERE year = 2026 AND month = '03';
SELECT order_id, amount, order_date, year, month FROM partitioned_orders WHERE year = 2026 AND month >= '05' ORDER BY month, order_id;
SELECT month, COUNT(*) AS order_count, SUM(amount) AS total_amount FROM partitioned_orders WHERE year = 2026 GROUP BY month ORDER BY month;
