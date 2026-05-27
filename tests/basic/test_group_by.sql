SELECT city, COUNT(*) AS cnt FROM users GROUP BY city HAVING COUNT(*) > 1;
SELECT category, COUNT(*), AVG(price) FROM products GROUP BY category ORDER BY category;
SELECT name, COUNT(*), SUM(amount) FROM users JOIN orders ON users.id = orders.user_id GROUP BY name;
