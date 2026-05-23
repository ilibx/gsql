-- 30. CREATE EXTERNAL TABLE
CREATE EXTERNAL TABLE external_users (
  id INT,
  name STRING,
  email STRING,
  age INT,
  city STRING
)
WITH (
  storage = 'local',
  format = 'csv',
  location = 'samples/users',
  file_pattern = '*.csv'
);

-- 31. PARTITIONED TABLE 定义
CREATE TABLE partitioned_orders (
  order_id INT,
  user_id INT,
  product_id INT,
  quantity INT,
  amount INT,
  order_date STRING
)
WITH (
  storage = 'local',
  format = 'csv',
  location = 'samples/orders',
  file_pattern = '*.csv'
)
PARTITIONED BY (order_date);

-- 32. UNION ALL 语法
SELECT city FROM users WHERE city = 'Beijing'
UNION ALL
SELECT city FROM users WHERE city = 'Shanghai';

-- 33. CASE WHEN + 算术表达式 + ORDER BY
SELECT name,
  CASE
    WHEN age < 30 THEN age + 1
    ELSE age - 1
  END AS age_adj
FROM users
WHERE age IS NOT NULL
ORDER BY age_adj
LIMIT 5;
