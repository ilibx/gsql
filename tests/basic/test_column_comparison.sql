-- Test column-to-column comparison (WHERE a = b)
CREATE TABLE orders (
  order_id INT,
  user_id INT,
  product_id INT,
  quantity INT,
  amount INT
)
WITH (
  storage = 'local',
  format = 'csv',
  path = '/tmp/orders',
  file_name = 'orders.csv'
);

-- Insert test data
INSERT OVERWRITE TABLE orders VALUES
  (1, 101, 1001, 2, 20),
  (2, 102, 1002, 1, 10),
  (3, 101, 1003, 3, 30),
  (4, 103, 1001, 1, 10),
  (5, 102, 1003, 2, 20);

-- Test WHERE user_id = order_id (should return empty)
SELECT * FROM orders WHERE user_id = order_id;

-- Test WHERE user_id = user_id (should return all rows)
SELECT * FROM orders WHERE user_id = user_id;

-- Test WHERE amount = quantity * 10 (should return all rows)
SELECT * FROM orders WHERE amount = quantity * 10;