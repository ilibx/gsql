-- Test misc functions
-- Uses users(id,name,email,age,city)

-- REGEXP_REPLACE
SELECT REGEXP_REPLACE('hello 123 world', '\\d+', 'X') AS re_replace FROM users LIMIT 1;
SELECT REGEXP_REPLACE(name, '[aeiou]', '*') AS name_masked FROM users LIMIT 1;

-- REGEXP_EXTRACT
SELECT REGEXP_EXTRACT('hello-123-world', '(\\d+)') AS re_extract FROM users LIMIT 1;
SELECT REGEXP_EXTRACT('hello-123-world', '([a-z]+)-\\d+-([a-z]+)', 2) AS re_extract_group FROM users LIMIT 1;

-- REGEXP_LIKE
SELECT REGEXP_LIKE('hello123', '\\d+') AS re_like_true FROM users LIMIT 1;
SELECT REGEXP_LIKE('hello', '\\d+') AS re_like_false FROM users LIMIT 1;

-- BASE64
SELECT BASE64('hello') AS b64 FROM users LIMIT 1;

-- UNBASE64
SELECT UNBASE64('aGVsbG8=') AS unb64 FROM users LIMIT 1;

-- HEX
SELECT HEX('hello') AS hx FROM users LIMIT 1;

-- UNHEX
SELECT UNHEX('68656c6c6f') AS unhx FROM users LIMIT 1;

-- MD5
SELECT MD5('hello') AS md5_hash FROM users LIMIT 1;

-- SHA1
SELECT SHA1('hello') AS sha1_hash FROM users LIMIT 1;

-- SHA2
SELECT SHA2('hello', '256') AS sha2_256 FROM users LIMIT 1;
SELECT SHA2('hello', '512') AS sha2_512 FROM users LIMIT 1;

-- CRC32
SELECT CRC32('hello') AS crc FROM users LIMIT 1;

-- HASH
SELECT HASH('hello') AS h FROM users LIMIT 1;
SELECT HASH(name, email) AS h_multi FROM users LIMIT 1;

-- ENCODE
SELECT ENCODE('hello', 'UTF-8') AS enc FROM users LIMIT 1;

-- DECODE
SELECT DECODE('hello', 'UTF-8') AS dec FROM users LIMIT 1;

-- GET_JSON_OBJECT
SELECT GET_JSON_OBJECT('{"key": "value", "num": 42}', '$.key') AS json_key FROM users LIMIT 1;
SELECT GET_JSON_OBJECT('{"key": "value", "num": 42}', '$.num') AS json_num FROM users LIMIT 1;

-- JSON_TUPLE
SELECT JSON_TUPLE('{"key": "value", "num": 42}', 'key') AS json_t_key FROM users LIMIT 1;

-- CURRENT_USER
SELECT CURRENT_USER() AS usr FROM users LIMIT 1;

-- CURRENT_DATABASE
SELECT CURRENT_DATABASE() AS db FROM users LIMIT 1;

-- VERSION
SELECT VERSION() AS ver FROM users LIMIT 1;

-- MASK
SELECT MASK('Abcd1234') AS msk FROM users LIMIT 1;
SELECT MASK('Abcd1234', 'U', 'l', 'd') AS msk_custom FROM users LIMIT 1;

-- MASK_FIRST_N
SELECT MASK_FIRST_N('Abcd1234', 3) AS msk_first FROM users LIMIT 1;

-- MASK_LAST_N
SELECT MASK_LAST_N('Abcd1234', 3) AS msk_last FROM users LIMIT 1;

-- MASK_SHOW_FIRST_N
SELECT MASK_SHOW_FIRST_N('Abcd1234', 3) AS msk_show_first FROM users LIMIT 1;

-- MASK_SHOW_LAST_N
SELECT MASK_SHOW_LAST_N('Abcd1234', 3) AS msk_show_last FROM users LIMIT 1;
