-- ============================================================
-- JOIN 类型全覆盖示例
-- 数据: samples/join/{customers,orders}.csv
--   customers: id 1-5
--   orders:    customer_id ∈ {1,1,2,9,3}
--   匹配的客户 {1,2,3}; 订单 104 无客户; 客户 4,5 无订单
-- 断言见 samples/join_test.go
-- 运行: ./bin/gsql -s samples/test_all_joins.sql
-- ============================================================

CREATE TABLE customers (
  id INT,
  name STRING,
  city STRING
) WITH (
  storage = 'local', format = 'csv',
  path = 'samples/join', file_pattern = 'customers.csv'
);

CREATE TABLE orders (
  order_id INT,
  customer_id INT,
  amount INT
) WITH (
  storage = 'local', format = 'csv',
  path = 'samples/join', file_pattern = 'orders.csv'
);

-- INNER JOIN: 只保留两侧都匹配的行 (4 行)
SELECT 'inner' AS join_type, COUNT(*) AS n FROM customers c JOIN orders o ON o.customer_id = c.id;

-- LEFT JOIN: 保留左表全部, 无匹配的右侧留空 (6 行)
SELECT 'left' AS join_type, COUNT(*) AS n FROM customers c LEFT JOIN orders o ON o.customer_id = c.id;

-- LEFT OUTER JOIN: 与 LEFT JOIN 等价 (6 行)
SELECT 'left_outer' AS join_type, COUNT(*) AS n FROM customers c LEFT OUTER JOIN orders o ON o.customer_id = c.id;

-- RIGHT JOIN: 保留右表全部, 无匹配的左侧留空 (5 行)
SELECT 'right' AS join_type, COUNT(*) AS n FROM customers c RIGHT JOIN orders o ON o.customer_id = c.id;

-- RIGHT OUTER JOIN: 与 RIGHT JOIN 等价 (5 行)
SELECT 'right_outer' AS join_type, COUNT(*) AS n FROM customers c RIGHT OUTER JOIN orders o ON o.customer_id = c.id;

-- FULL JOIN: 两侧全部保留 (7 行)
SELECT 'full' AS join_type, COUNT(*) AS n FROM customers c FULL JOIN orders o ON o.customer_id = c.id;

-- FULL OUTER JOIN: 与 FULL JOIN 等价 (7 行)
SELECT 'full_outer' AS join_type, COUNT(*) AS n FROM customers c FULL OUTER JOIN orders o ON o.customer_id = c.id;

-- LEFT SEMI JOIN: 左表每行只输出一次, 且仅当在右侧有匹配 (3 行: Alice,Bob,Charlie)
SELECT 'left_semi' AS join_type, COUNT(*) AS n FROM customers c LEFT SEMI JOIN orders o ON o.customer_id = c.id;

-- CROSS JOIN: 笛卡尔积 (5 x 5 = 25 行)
SELECT 'cross' AS join_type, COUNT(*) AS n FROM customers c CROSS JOIN orders o;

-- ============================================================
-- 详细结果演示 (可逐行核对)
-- ============================================================

-- INNER: Alice 两笔订单, Bob/Charlie 各一笔
SELECT c.name, o.order_id, o.amount
FROM customers c JOIN orders o ON o.customer_id = c.id
ORDER BY o.order_id;

-- LEFT: 客户 Diana、Eve 无订单, order_id 为空
SELECT c.name, o.order_id
FROM customers c LEFT JOIN orders o ON o.customer_id = c.id
ORDER BY c.name, o.order_id;

-- RIGHT: 订单 104 (customer_id=9) 无客户, name 为空
SELECT c.name, o.order_id
FROM customers c RIGHT JOIN orders o ON o.customer_id = c.id
ORDER BY o.order_id;

-- FULL: 合并上面两种未匹配 (未匹配行 name 为空字符串, 排在最前)
SELECT c.name, o.order_id
FROM customers c FULL JOIN orders o ON o.customer_id = c.id
ORDER BY c.name, o.order_id;

-- LEFT SEMI: 每个有订单的客户只出现一次 (Alice 不会因两笔订单重复)
SELECT c.name
FROM customers c LEFT SEMI JOIN orders o ON o.customer_id = c.id
ORDER BY c.name;

-- 多表链式 join: customers -> orders -> (id 关联自身做演示)
SELECT c.name, p.city, COUNT(*) AS orders
FROM customers c
JOIN orders o ON o.customer_id = c.id
JOIN customers p ON p.id = c.id
GROUP BY c.name, p.city
ORDER BY c.name;
