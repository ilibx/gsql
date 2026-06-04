CREATE TABLE users (
  id INT,
  name STRING,
  email STRING,
  age INT,
  city STRING
)
WITH (
  storage = 'local',
  format = 'csv',
  path = 'samples/users',
  file_pattern = 'users.csv'
);

SELECT id, name, age, city
FROM users
{% if min_age %}
WHERE age >= {{ min_age }}
{% endif %}
{% if order_by %}
ORDER BY {{ order_by }}
{% endif %}
{% if limit %}
LIMIT {{ limit }}
{% endif %};
