SELECT name, amount, order_date FROM users JOIN orders ON users.id = orders.user_id;
SELECT name, amount FROM users JOIN orders ON users.id = orders.user_id WHERE amount > 1000;
SELECT name, amount FROM users JOIN orders ON users.id = orders.user_id ORDER BY amount DESC LIMIT 5;
