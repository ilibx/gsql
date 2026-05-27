-- Test window functions
-- Uses users(id,name,email,age,city)

-- ROW_NUMBER
SELECT name, age, ROW_NUMBER() OVER (ORDER BY age DESC) AS rn FROM users;
SELECT name, age, city, ROW_NUMBER() OVER (PARTITION BY city ORDER BY age) AS rn_city FROM users;

-- RANK
SELECT name, age, RANK() OVER (ORDER BY age DESC) AS rk FROM users;

-- DENSE_RANK
SELECT name, age, DENSE_RANK() OVER (ORDER BY age DESC) AS drk FROM users;

-- LEAD
SELECT name, age, LEAD(age, 1) OVER (ORDER BY age) AS next_age FROM users;

-- LAG
SELECT name, age, LAG(age, 1) OVER (ORDER BY age) AS prev_age FROM users;

-- NTILE
SELECT name, age, NTILE(4) OVER (ORDER BY age) AS bucket FROM users;

-- FIRST_VALUE
SELECT name, age, FIRST_VALUE(name) OVER (ORDER BY age) AS first_name FROM users;

-- LAST_VALUE
SELECT name, age, LAST_VALUE(name) OVER (ORDER BY age ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING) AS last_name FROM users;

-- CUME_DIST
SELECT name, age, CUME_DIST() OVER (ORDER BY age) AS cum_dist FROM users;

-- PERCENT_RANK
SELECT name, age, PERCENT_RANK() OVER (ORDER BY age) AS pct_rank FROM users;

-- NTH_VALUE
SELECT name, age, NTH_VALUE(name, 2) OVER (ORDER BY age ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING) AS second_name FROM users;
