SELECT name, email FROM users WHERE city IN ('Beijing', 'Shanghai') AND email IS NOT NULL ORDER BY name;
SELECT name FROM users WHERE city NOT IN ('Guangzhou') AND (email IS NULL OR email IS NOT NULL) ORDER BY name;
