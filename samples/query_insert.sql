-- 18. INSERT OVERWRITE
CREATE TABLE result_top_users (
  name STRING,
  total_amount INT
)
WITH (
  storage = 'local',
  format = 'csv',
  location = 'samples/result',
  file_name = 'top_users.csv'
);

INSERT OVERWRITE TABLE result_top_users
SELECT u.name, SUM(o.amount) as total
FROM users u
JOIN orders o ON u.id = o.user_id
GROUP BY u.name
ORDER BY total DESC
LIMIT 3;
