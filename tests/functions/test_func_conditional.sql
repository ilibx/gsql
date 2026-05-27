-- Test conditional functions
-- Uses users(id,name,email,age,city)

-- IF (use simple literal condition instead of expression)
SELECT IF(1, 'true', 'false') AS if_true FROM users LIMIT 1;
SELECT IF(0, 'true', 'false') AS if_false FROM users LIMIT 1;

-- COALESCE (use '' instead of NULL keyword)
SELECT COALESCE('', 'default') AS coalesce_val FROM users LIMIT 1;
SELECT COALESCE(name, 'unknown') AS coalesce_name FROM users LIMIT 1;

-- NVL
SELECT NVL('', 'default') AS nvl_val FROM users LIMIT 1;
SELECT NVL(name, 'unknown') AS nvl_name FROM users LIMIT 1;

-- NULLIF
SELECT NULLIF(5, 5) AS nullif_equal FROM users LIMIT 1;
SELECT NULLIF(5, 3) AS nullif_diff FROM users LIMIT 1;

-- CAST
SELECT CAST(age AS STRING) AS age_str FROM users LIMIT 1;
