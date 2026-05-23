SELECT name, age FROM (SELECT name, age FROM users) AS sub WHERE age > 30 ORDER BY age;
SELECT COUNT(*) AS young_cnt FROM (SELECT id FROM users WHERE age < 30) AS young;
