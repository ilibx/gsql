CREATE TABLE users (
  id INT,
  name STRING,
  age INT,
  city STRING
)
WITH (
  storage = 'local',
  format = 'csv',
  path = 'samples/users',
  file_pattern = 'users.csv'
);

{% for row in rows %}
INSERT INTO users VALUES ({{ row.id }}, '{{ row.name }}', {{ row.age }}, '{{ row.city }}');
{% endfor %}

SELECT id, name, age, city FROM users WHERE age >= {{ min_age }} ORDER BY id;
