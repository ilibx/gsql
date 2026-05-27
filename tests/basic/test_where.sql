SELECT name, age FROM users WHERE age >= 30;
SELECT name, city FROM users WHERE city = 'Beijing' AND age > 30;
SELECT name, city FROM users WHERE city != 'Beijing' ORDER BY name;
SELECT name, age FROM users WHERE age < 30 OR age > 40 ORDER BY age;
SELECT name, age, city FROM users WHERE (city = 'Beijing' OR city = 'Shanghai') AND age >= 30 ORDER BY age;
