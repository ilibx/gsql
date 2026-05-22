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
  location = 'samples/users',
  file_pattern = '*.csv'
);

CREATE TABLE products (
  id INT,
  name STRING,
  category STRING,
  price INT,
  stock INT
)
WITH (
  storage = 'local',
  format = 'csv',
  location = 'samples/products',
  file_pattern = '*.csv'
);

CREATE TABLE orders (
  order_id INT,
  user_id INT,
  product_id INT,
  quantity INT,
  amount INT,
  order_date STRING
)
WITH (
  storage = 'local',
  format = 'csv',
  location = 'samples/orders',
  file_pattern = '*.csv'
);
