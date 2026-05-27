-- Test string functions
-- Uses users(id,name,email,age,city)

-- CONCAT
SELECT CONCAT('Hello', ' ', 'World') AS greeting FROM users LIMIT 1;
SELECT CONCAT(name, ' from ', city) AS descr FROM users LIMIT 1;

-- CONCAT_WS
SELECT CONCAT_WS('-', '2026', '01', '15') AS date_str FROM users LIMIT 1;
SELECT CONCAT_WS(', ', name, email) AS info FROM users LIMIT 1;

-- SUBSTRING, SUBSTR
SELECT SUBSTRING('Hello World', 1, 5) AS substr1 FROM users LIMIT 1;
SELECT SUBSTR('Hello World', 7) AS substr2 FROM users LIMIT 1;
SELECT SUBSTRING(name, 1, 2) AS name_prefix FROM users LIMIT 1;

-- UPPER, UCASE
SELECT UPPER('hello') AS u1, UCASE('world') AS u2 FROM users LIMIT 1;
SELECT UPPER(name) AS name_upper FROM users LIMIT 1;

-- LOWER, LCASE
SELECT LOWER('HELLO') AS l1, LCASE('WORLD') AS l2 FROM users LIMIT 1;
SELECT LOWER(email) AS email_lower FROM users LIMIT 1;

-- TRIM
SELECT TRIM('  hello  ') AS trimmed FROM users LIMIT 1;
SELECT TRIM(name) AS name_trim FROM users LIMIT 1;

-- LTRIM
SELECT LTRIM('  hello  ') AS ltrimmed FROM users LIMIT 1;

-- RTRIM
SELECT RTRIM('  hello  ') AS rtrimmed FROM users LIMIT 1;

-- LENGTH
SELECT LENGTH('Hello') AS len FROM users LIMIT 1;
SELECT LENGTH(name) AS name_len FROM users LIMIT 1;

-- REPLACE
SELECT REPLACE('hello world', 'world', 'there') AS replaced FROM users LIMIT 1;

-- REVERSE
SELECT REVERSE('hello') AS rev FROM users LIMIT 1;
SELECT REVERSE(name) AS name_rev FROM users LIMIT 1;

-- LOCATE
SELECT LOCATE('world', 'hello world') AS loc1 FROM users LIMIT 1;
SELECT LOCATE('l', 'hello', 3) AS loc2 FROM users LIMIT 1;

-- INSTR
SELECT INSTR('hello world', 'world') AS instr_val FROM users LIMIT 1;

-- LPAD
SELECT LPAD('hello', 10, '-') AS lpad_val FROM users LIMIT 1;

-- RPAD
SELECT RPAD('hello', 10, '-') AS rpad_val FROM users LIMIT 1;

-- INITCAP
SELECT INITCAP('hello world') AS initcap_val FROM users LIMIT 1;
SELECT INITCAP(name) AS name_proper FROM users LIMIT 1;

-- ASCII
SELECT ASCII('A') AS ascii_val FROM users LIMIT 1;
SELECT ASCII(name) AS name_ascii FROM users LIMIT 1;

-- SPLIT
SELECT SPLIT('a,b,c', ',') AS split_val FROM users LIMIT 1;
