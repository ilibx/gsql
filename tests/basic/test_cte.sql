WITH young_users AS (SELECT id, name, age FROM users WHERE age < 30) SELECT name, age FROM young_users ORDER BY age;
