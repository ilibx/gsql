SELECT name, email FROM users WHERE id IN (SELECT user_id FROM orders);
