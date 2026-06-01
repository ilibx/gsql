-- Test nested function calls
-- Uses users(id,name,email,age,city)

-- Single-level nesting
SELECT UPPER(SUBSTR(name, 1, 3)) AS name_prefix FROM users LIMIT 1;
SELECT CONCAT(UPPER(name), ' from ', UPPER(city)) AS descr FROM users LIMIT 1;
SELECT LENGTH(UPPER(name)) AS name_len FROM users LIMIT 1;

-- Multi-level nesting
SELECT UPPER(CONCAT(SUBSTR(name, 1, 1), '. ', city)) AS short FROM users LIMIT 1;

-- Nesting with arithmetic arguments
SELECT ROUND(ABS(age * 1.5), 0) AS rounded FROM users LIMIT 1;

-- Nesting with multi-arg inner function
SELECT REPLACE(UPPER(name), 'A', 'X') AS replaced FROM users LIMIT 1;
SELECT CONCAT_WS(', ', UPPER(name), UPPER(email)) AS info FROM users LIMIT 1;
