CREATE EXTERNAL TABLE external_users (id INT, name STRING, email STRING, age INT, city STRING) WITH (storage = 'local', format = 'csv', location = 'samples/users', file_pattern = '*.csv');
