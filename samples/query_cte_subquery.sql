-- 16. CTE 简单示例
WITH young_users AS (
  SELECT id, name, age FROM users WHERE age < 30
)
SELECT name, age FROM young_users ORDER BY age;

-- 17. 子查询 FROM 子句
SELECT name, age FROM (
  SELECT name, age FROM users
) AS sub
WHERE age > 30
ORDER BY age;
