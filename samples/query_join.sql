-- 10. JOIN 查询：用户 + 订单
SELECT name, amount, order_date FROM users JOIN orders ON users.id = orders.user_id;

-- 11. JOIN + WHERE
SELECT name, amount FROM users JOIN orders ON users.id = orders.user_id WHERE amount > 1000;

-- 12. JOIN + ORDER BY
SELECT name, amount FROM users JOIN orders ON users.id = orders.user_id ORDER BY amount DESC LIMIT 5;

-- 13. JOIN + 聚合
SELECT name, COUNT(*), SUM(amount) FROM users JOIN orders ON users.id = orders.user_id GROUP BY name;

-- 14. JOIN + HAVING
SELECT name, COUNT(*) FROM users JOIN orders ON users.id = orders.user_id GROUP BY name HAVING COUNT(*) >= 2;

-- 15. 三表 JOIN
SELECT u.name, p.name, o.amount, o.order_date
FROM users u
JOIN orders o ON u.id = o.user_id
JOIN products p ON o.product_id = p.id
WHERE o.amount > 500
ORDER BY o.amount DESC;
