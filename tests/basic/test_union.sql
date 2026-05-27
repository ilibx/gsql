SELECT name FROM users WHERE city = 'Beijing' UNION SELECT name FROM users WHERE city = 'Shanghai';
SELECT name FROM users WHERE city = 'Beijing' UNION ALL SELECT name FROM users WHERE city = 'Shanghai';
