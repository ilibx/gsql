SELECT u.name, p.name AS product, o.amount, o.order_date FROM users u JOIN orders o ON u.id = o.user_id JOIN products p ON o.product_id = p.id WHERE o.amount > 500 ORDER BY o.amount DESC;
