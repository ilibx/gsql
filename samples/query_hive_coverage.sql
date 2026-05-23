-- 20. DISTINCT + ORDER BY
SELECT DISTINCT city FROM users ORDER BY city;

-- 21. 算术表达式 + 列别名
SELECT name, age + 1 AS next_age, email
FROM users
WHERE age + 5 > 25
ORDER BY next_age LIMIT 5;

-- 22. CASE WHEN 表达式
SELECT name,
  CASE
    WHEN age < 30 THEN 'young'
    WHEN age < 50 THEN 'adult'
    ELSE 'senior'
  END AS age_group
FROM users
ORDER BY age_group, name;

-- 23. IN / NOT IN / IS NOT NULL
SELECT name, email
FROM users
WHERE city IN ('Beijing', 'Shanghai')
  AND email IS NOT NULL
ORDER BY name;

SELECT name
FROM users
WHERE city NOT IN ('Guangzhou')
  AND (email IS NULL OR email IS NOT NULL)
ORDER BY name;

-- 24. UNION / UNION ALL
SELECT name FROM users WHERE city = 'Beijing'
UNION
SELECT name FROM users WHERE city = 'Shanghai';

SELECT name FROM users WHERE city = 'Beijing'
UNION ALL
SELECT name FROM users WHERE city = 'Shanghai';

-- 25. JOIN + 窗口函数 + 窗口聚合
SELECT u.name,
       o.amount,
       ROW_NUMBER() OVER (PARTITION BY u.city ORDER BY o.amount DESC) AS rn,
       RANK() OVER (PARTITION BY u.city ORDER BY o.amount DESC) AS rnk,
       DENSE_RANK() OVER (PARTITION BY u.city ORDER BY o.amount DESC) AS drnk,
       SUM(o.amount) OVER (PARTITION BY u.city ORDER BY o.amount DESC) AS city_total_amount
FROM users u
JOIN orders o ON u.id = o.user_id
ORDER BY u.city, rn
LIMIT 15;

-- 26. GROUP BY + HAVING + 别名 ORDER BY
SELECT u.city, COUNT(*) AS orders_count, SUM(o.amount) AS total_amount
FROM users u
JOIN orders o ON u.id = o.user_id
GROUP BY u.city
HAVING COUNT(*) >= 2
ORDER BY orders_count DESC;
