# gsql 用法大全

本文档穷举 gsql 的所有 SQL 语法和内置函数，每个示例均可直接执行。

## 前置：建表（测试数据）

以下所有示例基于三张表：`users`、`products`、`orders`。

```sql
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
  path = 'samples/products',
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
  path = 'samples/orders',
  file_pattern = 'data.csv'
)
PARTITIONED BY (year, month);
```

---

## 一、SELECT 基础

### 1.1 全表查询

```sql
SELECT * FROM users;
SELECT id, name, age FROM users;
```

### 1.2 无 FROM 子句（纯表达式计算）

```sql
SELECT 1 AS id, 'hello' AS msg, 3.14 AS pi;
SELECT 2 + 3 * 4 AS result;              -- 14，乘法优先
SELECT (2 + 3) * 4 AS result;            -- 20
```

### 1.3 DISTINCT

```sql
SELECT DISTINCT city FROM users ORDER BY city;
SELECT COUNT(DISTINCT city) AS city_cnt FROM users;
```

### 1.4 AS 别名

```sql
SELECT name AS user_name, age AS user_age FROM users LIMIT 3;
SELECT COUNT(*) AS total_users FROM users;
```

### 1.5 算术表达式

支持 `+`、`-`、`*`、`/`，可出现在 SELECT、WHERE、函数参数中：

```sql
SELECT name, age + 1 AS next_age FROM users LIMIT 3;
SELECT name, (age + 1) * 2 AS doubled_age FROM users LIMIT 3;
SELECT name, age FROM users WHERE age + 5 > 25 ORDER BY age LIMIT 5;
SELECT ROUND(age * 1.5, 0) AS rounded_age FROM users LIMIT 3;
```

### 1.6 字符串拼接 `||`

PostgreSQL 风格操作符，等效于 `CONCAT()`，支持任意层级嵌套：

```sql
SELECT name || ' from ' || city AS info FROM users;
SELECT 'Hello' || ' ' || 'World' AS greeting;               -- Hello World
SELECT MD5(name || '@' || city) AS hash FROM users;
SELECT CONCAT(UPPER(name), '@', city) AS email_fmt FROM users;
```

> **注意**：`||` 表达式需配合 `AS` 别名使用。

---

## 二、WHERE 条件

### 2.1 比较运算符

```sql
SELECT * FROM users WHERE age > 30;
SELECT * FROM users WHERE age >= 30;
SELECT * FROM users WHERE age < 30;
SELECT * FROM users WHERE age <= 30;
SELECT * FROM users WHERE age = 28;
SELECT * FROM users WHERE age != 28;
SELECT * FROM users WHERE age <> 28;
```

### 2.2 逻辑运算符

```sql
SELECT * FROM users WHERE age >= 25 AND age <= 35;
SELECT * FROM users WHERE city = 'Beijing' OR city = 'Shanghai';
SELECT * FROM users WHERE (city = 'Beijing' OR city = 'Shanghai') AND age >= 30;
```

### 2.3 LIKE 模糊匹配

```sql
SELECT name, email FROM users WHERE email LIKE '%@example.com';
SELECT name FROM users WHERE name LIKE 'A%';
SELECT name FROM users WHERE name LIKE '_o%';
```

### 2.4 IN / NOT IN

```sql
SELECT name, email FROM users WHERE city IN ('Beijing', 'Shanghai') ORDER BY name;
SELECT name FROM users WHERE city NOT IN ('Guangzhou') ORDER BY name;
SELECT name FROM users WHERE id IN (SELECT user_id FROM orders);
```

### 2.5 IS NULL / IS NOT NULL

```sql
SELECT name, email FROM users WHERE email IS NOT NULL AND city = 'Beijing' ORDER BY name;
SELECT name FROM users WHERE email IS NULL;
```

### 2.6 BETWEEN

```sql
SELECT * FROM users WHERE age BETWEEN 25 AND 35;
SELECT * FROM users WHERE age NOT BETWEEN 40 AND 50;
```

### 2.7 EXISTS / NOT EXISTS

```sql
SELECT name FROM users WHERE EXISTS (SELECT * FROM orders);
SELECT name FROM users WHERE NOT EXISTS (SELECT * FROM orders WHERE amount > 10000);
```

---

## 三、GROUP BY / HAVING

```sql
SELECT city, COUNT(*) AS cnt, AVG(age) AS avg_age
FROM users
GROUP BY city
ORDER BY cnt DESC;

SELECT city, COUNT(*) AS cnt
FROM users
GROUP BY city
HAVING cnt > 1
ORDER BY cnt DESC;

-- 多列分组
SELECT city, category, COUNT(*) AS cnt
FROM users u JOIN orders o ON u.id = o.user_id
JOIN products p ON o.product_id = p.id
GROUP BY city, category
ORDER BY cnt DESC;
```

---

## 四、ORDER BY / LIMIT

```sql
SELECT * FROM users ORDER BY age DESC LIMIT 5;
SELECT * FROM users ORDER BY age ASC, name ASC LIMIT 5;
SELECT name, age FROM users ORDER BY age DESC LIMIT 3;
```

---

## 五、JOIN

### 5.1 INNER JOIN

```sql
SELECT u.name, o.amount, o.order_date
FROM users u
JOIN orders o ON u.id = o.user_id;

SELECT u.name, o.amount
FROM users u
JOIN orders o ON u.id = o.user_id
WHERE o.amount > 1000
ORDER BY o.amount DESC LIMIT 5;
```

### 5.2 LEFT JOIN

```sql
SELECT u.name, o.amount
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
ORDER BY u.id;
```

### 5.3 RIGHT JOIN

```sql
SELECT u.name, o.amount
FROM users u
RIGHT JOIN orders o ON u.id = o.user_id
ORDER BY o.order_id;
```

### 5.4 FULL JOIN

```sql
SELECT u.name, o.amount
FROM users u
FULL JOIN orders o ON u.id = o.user_id;
```

### 5.5 LEFT SEMI JOIN

```sql
SELECT name FROM users
LEFT SEMI JOIN orders ON users.id = orders.user_id
ORDER BY name;
```

### 5.6 CROSS JOIN

```sql
SELECT name, amount
FROM users
CROSS JOIN orders
ORDER BY name, amount LIMIT 10;
```

### 5.7 多表 JOIN

```sql
SELECT u.name, p.name AS product, o.amount
FROM users u
JOIN orders o ON u.id = o.user_id
JOIN products p ON o.product_id = p.id
WHERE o.amount > 500
ORDER BY o.amount DESC LIMIT 10;
```

> **类型感知比较**：JOIN 条件自动处理类型转换，`INT` 列去掉前导零，`DECIMAL` 列去掉尾零，`STRING` 列去除首尾空格。

---

## 六、子查询 / CTE

### 6.1 派生表（FROM 子查询）

```sql
SELECT name, age FROM (SELECT name, age FROM users) AS sub WHERE age > 30 ORDER BY age;
SELECT COUNT(*) AS young_cnt FROM (SELECT id FROM users WHERE age < 30) AS young;
```

### 6.2 CTE（WITH 子句）

```sql
WITH young_users AS (SELECT id, name, age FROM users WHERE age < 30)
SELECT name, age FROM young_users ORDER BY age;

-- 多 CTE
WITH beijing AS (SELECT id, name FROM users WHERE city = 'Beijing'),
     shanghai AS (SELECT id, name FROM users WHERE city = 'Shanghai')
SELECT name FROM beijing
UNION ALL
SELECT name FROM shanghai
ORDER BY name;
```

### 6.3 UNION / UNION ALL

```sql
SELECT name FROM users WHERE city = 'Beijing'
UNION
SELECT name FROM users WHERE city = 'Shanghai'
ORDER BY name;

SELECT name FROM users WHERE city = 'Beijing'
UNION ALL
SELECT name FROM users WHERE city = 'Shanghai'
ORDER BY name;
```

---

## 七、CASE WHEN

```sql
SELECT name,
  CASE WHEN age < 30 THEN 'young'
       WHEN age < 50 THEN 'adult'
       ELSE 'senior'
  END AS age_group
FROM users
ORDER BY age_group, name;

SELECT name,
  CASE WHEN age < 30 THEN age + 1
       ELSE age - 1
  END AS age_adj
FROM users
WHERE age IS NOT NULL
ORDER BY age_adj LIMIT 5;
```

---

## 八、窗口函数

```sql
-- ROW_NUMBER
SELECT name, age, ROW_NUMBER() OVER (ORDER BY age DESC) AS rn FROM users;
SELECT name, age, city, ROW_NUMBER() OVER (PARTITION BY city ORDER BY age) AS rn_city FROM users;

-- RANK, DENSE_RANK
SELECT name, age, RANK() OVER (ORDER BY age DESC) AS rk FROM users;
SELECT name, age, DENSE_RANK() OVER (ORDER BY age DESC) AS drk FROM users;

-- LEAD, LAG
SELECT name, age,
  LEAD(age, 1) OVER (ORDER BY age) AS next_age,
  LAG(age, 1) OVER (ORDER BY age) AS prev_age
FROM users;

-- NTILE
SELECT name, age, NTILE(4) OVER (ORDER BY age) AS bucket FROM users;

-- FIRST_VALUE, LAST_VALUE
SELECT name, age,
  FIRST_VALUE(name) OVER (ORDER BY age) AS first_name,
  LAST_VALUE(name) OVER (ORDER BY age ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING) AS last_name
FROM users;

-- CUME_DIST, PERCENT_RANK
SELECT name, age,
  CUME_DIST() OVER (ORDER BY age) AS cum_dist,
  PERCENT_RANK() OVER (ORDER BY age) AS pct_rank
FROM users;

-- NTH_VALUE
SELECT name, age,
  NTH_VALUE(name, 2) OVER (ORDER BY age ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING) AS second_name
FROM users;

-- 窗口聚合
SELECT u.name, o.amount,
  SUM(o.amount) OVER (PARTITION BY u.city ORDER BY o.amount DESC) AS city_total_amount
FROM users u JOIN orders o ON u.id = o.user_id
ORDER BY u.city, city_total_amount DESC LIMIT 15;
```

---

## 九、INSERT

### 9.1 INSERT OVERWRITE（覆盖写入）

```sql
CREATE TABLE result_top_users (name STRING, total_amount INT)
WITH (storage = 'local', format = 'csv', path = 'samples/result', file_name = 'top_users.csv');

INSERT OVERWRITE TABLE result_top_users
SELECT u.name, SUM(o.amount) AS total
FROM users u JOIN orders o ON u.id = o.user_id
GROUP BY u.name
ORDER BY total DESC LIMIT 3;

SELECT * FROM result_top_users;
```

### 9.2 INSERT INTO（追加写入）

```sql
CREATE TABLE result_active_users (name STRING, city STRING)
WITH (storage = 'local', format = 'csv', path = 'samples/result', file_name = 'active_users.csv', include_header = 'true');

INSERT INTO TABLE result_active_users
SELECT name, city FROM users WHERE age < 30 ORDER BY name;

SELECT * FROM result_active_users;
```

### 9.3 INSERT ... VALUES

```sql
-- 单行覆盖写入
INSERT OVERWRITE TABLE users VALUES (1, 'Alice', 'alice@example.com', 28, 'Beijing');

-- 多行追加
INSERT INTO TABLE users
VALUES (2, 'Bob', 'bob@example.com', 35, 'Shanghai'),
       (3, 'Charlie', 'charlie@example.com', 42, 'Shenzhen');
```

---

## 十、分区表

```sql
-- 读取全部分区
SELECT order_id, amount, year, month FROM partitioned_orders ORDER BY order_id;

-- 等值分区裁剪
SELECT order_id, amount, order_date FROM partitioned_orders WHERE year = 2026 AND month = '03';

-- 范围分区裁剪
SELECT order_id, amount, order_date, year, month
FROM partitioned_orders
WHERE year = 2026 AND month >= '05'
ORDER BY month, order_id;

-- 分区列参与聚合
SELECT month, COUNT(*) AS order_count, SUM(amount) AS total_amount
FROM partitioned_orders
WHERE year = 2026
GROUP BY month
ORDER BY month;
```

写入时按分区列自动分组：

```sql
CREATE TABLE monthly_summary (month STRING, order_count INT, total_amount INT)
WITH (storage = 'local', format = 'csv', path = 'samples/result/monthly', file_name = 'summary.csv')
PARTITIONED BY (year);

INSERT OVERWRITE TABLE monthly_summary
SELECT month, COUNT(*) AS order_count, SUM(amount) AS total_amount, year
FROM partitioned_orders
GROUP BY month, year;

SELECT month, order_count, total_amount, year FROM monthly_summary ORDER BY month;
```

---

## 十一、VALUES 内联表

```sql
SELECT * FROM (VALUES (1, 'a'), (2, 'b'), (3, 'c')) AS t(id, name) WHERE id > 1;
```

---

## 十二、EXPLAIN

```sql
EXPLAIN SELECT name, age
FROM users
WHERE age > 25
ORDER BY age
LIMIT 5;
```

输出三层计划：Logical Plan → Optimized Logical Plan → Physical Plan。

---

## 十三、字符串函数

```sql
-- 无需 FROM，直接执行
SELECT CONCAT('Hello', ' ', 'World') AS greeting;
SELECT CONCAT_WS('-', '2026', '01', '15') AS date_str;
SELECT 'Hello' || ' ' || 'World' AS greeting;
SELECT SUBSTRING('Hello World', 1, 5) AS sub1;
SELECT SUBSTR('Hello World', 7) AS sub2;
SELECT UPPER('hello') AS u1, UCASE('world') AS u2;
SELECT LOWER('HELLO') AS l1, LCASE('WORLD') AS l2;
SELECT TRIM('  hello  ') AS trimmed;
SELECT LTRIM('  hello  ') AS ltrimmed;
SELECT RTRIM('  hello  ') AS rtrimmed;
SELECT LENGTH('Hello') AS len;
SELECT REPLACE('hello world', 'world', 'there') AS replaced;
SELECT REVERSE('hello') AS rev;
SELECT LOCATE('world', 'hello world') AS loc1;
SELECT LOCATE('l', 'hello', 3) AS loc2;
SELECT INSTR('hello world', 'world') AS instr_val;
SELECT LPAD('hello', 10, '-') AS lpad_val;
SELECT RPAD('hello', 10, '-') AS rpad_val;
SELECT INITCAP('hello world') AS initcap_val;
SELECT ASCII('A') AS ascii_val;
SELECT SPLIT('a,b,c', ',') AS split_val;
```

表字段操作：

```sql
SELECT UPPER(name) AS name_upper FROM users LIMIT 3;
SELECT SUBSTR(name, 1, 3) AS prefix FROM users LIMIT 3;
SELECT LENGTH(name) AS name_len FROM users LIMIT 3;
SELECT REPLACE(name, 'a', 'A') AS replaced FROM users LIMIT 3;
SELECT REVERSE(name) AS name_rev FROM users LIMIT 3;
SELECT CONCAT(name, ' from ', city) AS descr FROM users LIMIT 3;
SELECT CONCAT_WS(', ', UPPER(name), city) AS info FROM users LIMIT 3;
```

---

## 十四、数学函数

```sql
SELECT ROUND(3.14, 2) AS pi;
SELECT ROUND(3.5) AS rounded;
SELECT FLOOR(3.99) AS fl;
SELECT CEIL(3.1) AS cl, CEILING(3.1) AS clg;
SELECT ABS(-5) AS abs_val, ABS(5) AS abs_pos;
SELECT SQRT(100) AS sqrt_val;
SELECT EXP(1) AS exp_val;
SELECT LN(2) AS ln_val;
SELECT LOG10(100) AS log10_val;
SELECT LOG2(8) AS log2_val;
SELECT POWER(2, 3) AS pow1, POW(2, 3) AS pow2;
SELECT MOD(10, 3) AS mod_val;
SELECT SIGN(-5) AS sn, SIGN(0) AS s0, SIGN(5) AS sp;
SELECT RAND() AS r1, RAND(42) AS r2;
SELECT GREATEST(3, 7, 1, 9, 5) AS max_val;
SELECT LEAST(3, 7, 1, 9, 5) AS min_val;
SELECT WIDTH_BUCKET(age, 20, 50, 5) AS bucket FROM users LIMIT 3;
```

表字段操作：

```sql
SELECT ROUND(age * 1.5, 0) AS rounded_age FROM users LIMIT 3;
SELECT ABS(price) AS abs_price FROM products LIMIT 3;
SELECT MOD(price, 100) AS price_mod FROM products LIMIT 3;
```

---

## 十五、日期时间函数

```sql
SELECT CURRENT_DATE() AS today;
SELECT CURRENT_TIMESTAMP() AS now;
SELECT UNIX_TIMESTAMP() AS ts_now;
SELECT UNIX_TIMESTAMP('2026-01-15') AS ts_date;
SELECT UNIX_TIMESTAMP('20260322', 'yyyyMMdd') AS ts_pattern;
SELECT FROM_UNIXTIME(1768492800) AS dt;
SELECT FROM_UNIXTIME(1768492800, 'yyyy-MM-dd') AS dt_fmt;
SELECT TO_DATE('2026-01-15 10:30:00') AS dt;
SELECT YEAR('2026-01-15') AS yr;
SELECT MONTH('2026-01-15') AS mon;
SELECT DAY('2026-01-15') AS dy, DAYOFMONTH('2026-01-15') AS dom;
SELECT HOUR('2026-01-15 10:30:45') AS hr;
SELECT MINUTE('2026-01-15 10:30:45') AS min;
SELECT SECOND('2026-01-15 10:30:45') AS sec;
SELECT WEEKOFYEAR('2026-01-15') AS wk;
SELECT DATEDIFF('2026-01-20', '2026-01-15') AS diff;
SELECT DATE_ADD('2026-01-15', 5) AS added;
SELECT DATE_SUB('2026-01-15', 5) AS subd;
SELECT DATE_FORMAT('2026-01-15', 'yyyy/MM/dd') AS fmt;
SELECT ADD_MONTHS('2026-01-15', 2) AS added_mon;
SELECT ADD_MONTHS('2026-01-31', 1) AS next_mon;
SELECT LAST_DAY('2026-02-15') AS last_day;
SELECT NEXT_DAY('2026-01-15', 'Monday') AS next_mon;
SELECT MONTHS_BETWEEN('2026-03-15', '2026-01-15') AS mons;
SELECT QUARTER('2026-04-15') AS qtr;
SELECT TRUNC('2026-01-15', 'MM') AS trunc_mm;
SELECT TRUNC('2026-01-15', 'YY') AS trunc_yy;
SELECT EXTRACT(YEAR FROM '2026-05-27') AS ext_year;
SELECT EXTRACT(MONTH FROM '2026-05-27') AS ext_month;
SELECT EXTRACT(DAY FROM '2026-05-27') AS ext_day;
SELECT EXTRACT(HOUR FROM '2026-05-27 14:30:45') AS ext_hour;
SELECT EXTRACT(MINUTE FROM '2026-05-27 14:30:45') AS ext_minute;
SELECT EXTRACT(SECOND FROM '2026-05-27 14:30:45') AS ext_second;
SELECT FROM_UTC_TIMESTAMP('2026-01-15 10:00:00', 'Asia/Shanghai') AS utc_to_sh;
SELECT TO_UTC_TIMESTAMP('2026-01-15 18:00:00', 'Asia/Shanghai') AS sh_to_utc;
```

表字段操作：

```sql
SELECT YEAR(order_date) AS ord_year, MONTH(order_date) AS ord_month FROM orders LIMIT 3;
SELECT DATE_FORMAT(order_date, 'yyyy/MM') AS ord_month_fmt FROM orders LIMIT 3;
```

---

## 十六、条件函数

```sql
SELECT IF(1, 'true', 'false') AS if_true;
SELECT IF(0, 'true', 'false') AS if_false;
SELECT COALESCE('', 'default') AS coalesce_val;
SELECT COALESCE(name, 'unknown') AS coalesce_name FROM users LIMIT 1;
SELECT NVL('', 'default') AS nvl_val;
SELECT NVL(name, 'unknown') AS nvl_name FROM users LIMIT 1;
SELECT NULLIF(5, 5) AS nullif_equal;
SELECT NULLIF(5, 3) AS nullif_diff;
SELECT CAST(age AS STRING) AS age_str FROM users LIMIT 1;
SELECT CAST('3.14' AS FLOAT) AS cast_float;
```

---

## 十七、聚合函数

```sql
SELECT COUNT(*) AS total FROM users;
SELECT COUNT(DISTINCT city) AS distinct_cities FROM users;
SELECT SUM(age) AS sum_age, AVG(age) AS avg_age, MIN(age) AS min_age, MAX(age) AS max_age FROM users;
SELECT STDDEV(age) AS stddev_age, STDDEV_POP(age) AS stddev_pop, STDDEV_SAMP(age) AS stddev_samp FROM users;
SELECT VARIANCE(age) AS var_age, VAR_POP(age) AS var_pop, VAR_SAMP(age) AS var_samp FROM users;
SELECT CORR(price, stock) AS corr, COVAR_POP(price, stock) AS covar_pop, COVAR_SAMP(price, stock) AS covar_samp FROM products;
SELECT COLLECT_LIST(city) AS city_list FROM users;
SELECT COLLECT_SET(city) AS city_set FROM users;
SELECT PERCENTILE(age, '0.5') AS median_age FROM users;
SELECT PERCENTILE_APPROX(age, '0.5') AS approx_median FROM users;
SELECT PERCENTILE_APPROX(age, '0.5', '100') AS approx_median_prec FROM users;
SELECT HISTOGRAM_NUMERIC(age, 3) AS age_histogram FROM users;
```

---

## 十八、正则表达式函数

```sql
SELECT REGEXP_REPLACE('hello 123 world', '\\d+', 'X') AS re_replace;
SELECT REGEXP_REPLACE(name, '[aeiou]', '*') AS name_masked FROM users LIMIT 1;
SELECT REGEXP_EXTRACT('hello-123-world', '(\\d+)') AS re_extract;
SELECT REGEXP_EXTRACT('hello-123-world', '([a-z]+)-\\d+-([a-z]+)', 2) AS re_extract_group;
SELECT REGEXP_LIKE('hello123', '\\d+') AS has_digit;
SELECT REGEXP_LIKE('hello', '\\d+') AS no_digit;
```

---

## 十九、编解码函数

```sql
SELECT BASE64('hello') AS b64;
SELECT UNBASE64('aGVsbG8=') AS txt;
SELECT HEX('hello') AS hx;
SELECT UNHEX('68656c6c6f') AS unhx;
SELECT ENCODE('hello', 'UTF-8') AS enc;
SELECT DECODE('hello', 'UTF-8') AS dec;
```

---

## 二十、哈希函数

```sql
SELECT MD5('hello') AS md5_hash;
SELECT SHA1('hello') AS sha1_hash;
SELECT SHA2('hello', '256') AS sha2_256;
SELECT SHA2('hello', '512') AS sha2_512;
SELECT CRC32('hello') AS crc;
SELECT HASH('hello') AS h;
SELECT HASH(name, email) AS h_multi FROM users LIMIT 1;
SELECT MD5(CONCAT(name, '@', city)) AS user_hash FROM users LIMIT 3;
```

---

## 二十一、JSON 函数

```sql
SELECT GET_JSON_OBJECT('{"key": "value", "num": 42}', '$.key') AS json_key;
SELECT GET_JSON_OBJECT('{"key": "value", "num": 42}', '$.num') AS json_num;
SELECT JSON_TUPLE('{"key": "value", "num": 42}', 'key') AS json_t_key;
```

---

## 二十二、数据脱敏函数

```sql
SELECT MASK('Abcd1234') AS msk;
SELECT MASK('Abcd1234', 'U', 'l', 'd') AS msk_custom;
SELECT MASK_FIRST_N('Abcd1234', 3) AS msk_first;
SELECT MASK_LAST_N('Abcd1234', 3) AS msk_last;
SELECT MASK_SHOW_FIRST_N('Abcd1234', 3) AS show_first;
SELECT MASK_SHOW_LAST_N('Abcd1234', 3) AS show_last;
```

---

## 二十三、元信息函数

```sql
SELECT CURRENT_USER() AS usr, CURRENT_DATABASE() AS db, VERSION() AS ver;
```

---

## 二十四、嵌套函数

所有标量函数支持任意层级嵌套：

```sql
SELECT UPPER(SUBSTR(name, 1, 3)) AS prefix FROM users LIMIT 1;
SELECT CONCAT(UPPER(name), ' from ', city) AS descr FROM users LIMIT 1;
SELECT REPLACE(UPPER(name), 'A', 'X') AS replaced FROM users LIMIT 1;
SELECT ROUND(ABS(age * 1.5), 0) AS rounded FROM users LIMIT 1;
SELECT UPPER(CONCAT(SUBSTR(name, 1, 1), '. ', city)) AS short FROM users LIMIT 1;
SELECT LENGTH(UPPER(name)) AS name_len FROM users LIMIT 1;
SELECT CONCAT_WS(', ', UPPER(name), city) AS info FROM users LIMIT 1;
```

---

## 二十五、创建表

### 25.1 基本建表

```sql
CREATE TABLE my_table (
  id INT,
  name STRING,
  score DOUBLE,
  active BOOLEAN
)
WITH (
  storage = 'local',
  format = 'csv',
  path = '/tmp/my_table',
  file_pattern = '*.csv'
);
```

支持的列类型：`INT`、`BIGINT`、`FLOAT`、`DOUBLE`、`STRING`、`BOOLEAN`、`DECIMAL(m,n)`。

### 25.2 外部表

```sql
CREATE EXTERNAL TABLE external_users (
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
  file_pattern = '*.csv'
);
```

### 25.3 带分区建表

```sql
CREATE TABLE events (
  id INT,
  name STRING,
  event_date STRING
)
WITH (
  storage = 'local',
  format = 'csv',
  path = '/tmp/events',
  file_name = 'data.csv'
)
PARTITIONED BY (year, month);
```

### 25.4 URL 方式建表

`url` 方式可自动推断存储类型，无需指定 `storage`。带前缀的参数可覆盖 URL 中的值（混合模式）。

```sql
-- Local（本地文件系统）
CREATE TABLE local_data (id INT, name STRING)
WITH (url = 'local:///tmp/data', format = 'csv');

-- S3 / MinIO / 阿里 OSS
CREATE TABLE s3_data (id INT, name STRING)
WITH (url = 's3://s3.example.com/my-bucket/data?region=us-east-1&access_key=xxx&access_secret=yyy', format = 'csv');

-- FTP
CREATE TABLE ftp_data (id INT, name STRING)
WITH (url = 'ftp://user:pass@ftp.example.com:21/data', format = 'csv');

-- SFTP
CREATE TABLE sftp_data (id INT, name STRING)
WITH (url = 'sftp://user:pass@sftp.example.com:22/data', format = 'csv');

-- WebDAV
CREATE TABLE webdav_data (id INT, name STRING)
WITH (url = 'webdav://user:pass@webdav.example.com/data', format = 'csv');

-- Git LFS
CREATE TABLE gitlfs_data (id INT, name STRING)
WITH (url = 'gitlfs:///path/to/git/repo', format = 'csv');

-- 飞书 Lark（app_id/app_secret 放在 query 中）
CREATE TABLE lark_data (id INT, name STRING)
WITH (url = 'lark://folder/path?app_id=cli_xxx&app_secret=xxx&chat_id=oc_xxx', format = 'csv');

-- MySQL
CREATE TABLE mysql_data (id INT, name STRING)
WITH (url = 'mysql://root:secret@127.0.0.1:3306/mydb', table_name = 'users');

-- PostgreSQL
CREATE TABLE pg_data (id INT, name STRING)
WITH (url = 'postgres://postgres:secret@127.0.0.1:5432/mydb?sslmode=disable', table_name = 'users');

-- SQLite
CREATE TABLE sqlite_data (id INT, name STRING)
WITH (url = 'sqlite:///path/to/db.sqlite', table_name = 'users');

-- 混合模式：URL + 参数覆盖
CREATE TABLE ftp_mixed (id INT, name STRING)
WITH (
  url = 'ftp://olduser:oldpass@ftp.example.com:21/test',
  username = 'custom_user',
  password = 'custom_pass',
  format = 'csv'
);
```

### 25.5 反引号包裹复杂选项值

```sql
CREATE TABLE complex (id INT, name STRING)
WITH (
  query = `SELECT id, name FROM source WHERE city = 'Beijing' AND age > 25`
);
```

反引号内部可自由使用单引号、双引号和换行。

---

## 二十六、文件读写

### 26.1 CSV 读写

```sql
CREATE TABLE csv_out (id INT, name STRING)
WITH (storage = 'local', format = 'csv', path = 'samples/result', file_name = 'csv_out.csv', include_header = 'true');

INSERT OVERWRITE TABLE csv_out
SELECT 1 AS id, 'Alice' AS name
UNION ALL
SELECT 2, 'Bob';

SELECT * FROM csv_out;
```

自定义分隔符：

```sql
CREATE TABLE pipe_users (id INT, name STRING, email STRING)
WITH (
  storage = 'local',
  format = 'csv',
  path = 'samples/csv_opts',
  file_pattern = 'pipe_delim.csv',
  delimiter = '|'
);

SELECT name FROM pipe_users ORDER BY id;
```

跳过表头行：

```sql
CREATE TABLE header_scores (id INT, name STRING, score INT)
WITH (
  storage = 'local',
  format = 'csv',
  path = 'samples/csv_opts',
  file_pattern = 'with_header.csv',
  delimiter = '|',
  skip_lines = '2'
);

SELECT name, score FROM header_scores ORDER BY id;
```

### 26.2 JSON 读写

```sql
CREATE TABLE json_out (id INT, name STRING)
WITH (storage = 'local', format = 'json', path = 'samples/result', file_name = 'json_out.json');

INSERT OVERWRITE TABLE json_out
SELECT 1 AS id, 'Alice' AS name;

SELECT * FROM json_out;
```

### 26.3 Excel (.xlsx) 读写

```sql
CREATE TABLE excel_out (id INT, name STRING, score INT)
WITH (
  storage = 'local',
  format = 'xlsx',
  path = 'samples/result',
  file_name = 'excel_out.xlsx',
  include_header = 'true',
  sheet = 'Sheet1'
);

INSERT OVERWRITE TABLE excel_out
SELECT 1 AS id, 'Alice' AS name, 95 AS score
UNION ALL
SELECT 2, 'Bob', 87;

SELECT * FROM excel_out;
```

---

## 二十七、命令行

```bash
# 执行 SQL 文件
gsql -s query.sql

# 从远程存储读取
gsql -s s3://bucket/queries/query.sql
gsql -s lark://token/folder/query.sql
gsql -s https://example.com/query.sql

# 执行内联 SQL
gsql -e "SELECT 1 AS id, 'hello' AS msg"

# 调试模式
gsql -v -s query.sql        # 调试级别 1
gsql -vvvvvv -s query.sql   # 调试级别 6
gsql desc -s query.sql      # desc 等价于 -v

# 查看表元信息
gsql desc
```

---

## 二十八、模板 SQL（Jinja2）

```bash
# 加载模板文件并用参数填充
gsql -t query.sql -a "min_age=30" -a "city=Beijing"

# 重复参数自动合并为逗号分隔字符串
gsql -t query.sql -a "month=2026-05" -a "user=alice" -a "user=bob"
```

模板文件：
```sql
SELECT id, name, age
FROM users
WHERE age >= {{ min_age }} AND city = '{{ city }}'
ORDER BY id;
```

循环渲染（批量插入）：
```bash
gsql -t batch_insert.sql -a "rows=[{\"id\":1,\"name\":\"Alice\"},{\"id\":2,\"name\":\"Bob\"}]"
```

模板文件：
```sql
CREATE TABLE users (id INT, name STRING)
WITH (storage='local', format='csv', path='output');

{% for row in rows %}
INSERT INTO users VALUES ({{ row.id }}, '{{ row.name }}');
{% endfor %}

SELECT * FROM users;
```

条件渲染：
```sql
SELECT * FROM users
{% if min_age %}
WHERE age >= {{ min_age }}
{% endif %}
ORDER BY id;
```

JSON 文件引用（`@` 前缀）：
```bash
gsql -t batch.sql -a "rows=@data.json"
```
