CREATE TABLE mydb (
  greeting STRING,
  target STRING,
  num INT
)
WITH (
  storage = 'sqlite',
  path = ':memory:',
  query = `SELECT 'hello' AS greeting, "world" AS target, 42 AS num`
);

SELECT * FROM mydb;
