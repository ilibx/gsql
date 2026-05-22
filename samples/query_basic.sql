-- 1. 简单查询
SELECT name, email FROM users;

-- 2. 带 WHERE 条件
SELECT name, age FROM users WHERE age >= 30;

-- 3. WHERE + AND/OR 复合条件
SELECT name, city FROM users WHERE city = 'Beijing' AND age > 30;

-- 4. LIKE 模糊查询
SELECT name, email FROM users WHERE email LIKE '%@example.com';

-- 5. ORDER BY + LIMIT
SELECT name, age FROM users ORDER BY age DESC LIMIT 3;

-- 6. 聚合：COUNT
SELECT COUNT(*) FROM users;

-- 7. 聚合：SUM, AVG, MIN, MAX
SELECT SUM(price), AVG(price), MIN(price), MAX(price) FROM products;

-- 8. GROUP BY + 聚合 + HAVING
SELECT city, COUNT(*) as cnt FROM users GROUP BY city HAVING COUNT(*) > 1;

-- 9. GROUP BY + ORDER BY
SELECT category, COUNT(*), AVG(price) FROM products GROUP BY category ORDER BY category;
