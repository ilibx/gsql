SELECT name, COUNT(*) FROM users JOIN orders ON users.id = orders.user_id GROUP BY name HAVING COUNT(*) >= 2;
SELECT SUM(amount) AS total FROM orders HAVING total > 0;
