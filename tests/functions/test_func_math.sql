-- Test math functions
-- Uses users(id,name,email,age,city), products(id,name,category,price,stock)

-- ROUND (use integer args instead of decimal literals)
SELECT ROUND(3, 2) AS rnd FROM users LIMIT 1;
SELECT ROUND(age, 1) AS rnd_age FROM users LIMIT 1;

-- FLOOR
SELECT FLOOR(3) AS fl FROM users LIMIT 1;
SELECT FLOOR(price) AS fl_price FROM products LIMIT 1;

-- CEIL, CEILING
SELECT CEIL(3) AS cl, CEILING(3) AS clg FROM users LIMIT 1;

-- ABS
SELECT ABS(5) AS abs_pos FROM users LIMIT 1;
SELECT ABS(price) AS abs_price FROM products LIMIT 1;

-- SQRT
SELECT SQRT(100) AS sqrt_val FROM users LIMIT 1;

-- EXP
SELECT EXP(1) AS exp_val FROM users LIMIT 1;

-- LN
SELECT LN(2) AS ln_val FROM users LIMIT 1;

-- LOG10
SELECT LOG10(100) AS log10_val FROM users LIMIT 1;

-- LOG2
SELECT LOG2(8) AS log2_val FROM users LIMIT 1;

-- POWER, POW
SELECT POWER(2, 3) AS pow1, POW(2, 3) AS pow2 FROM users LIMIT 1;

-- MOD
SELECT MOD(10, 3) AS mod_val FROM users LIMIT 1;

-- SIGN
SELECT SIGN(0) AS sign_zero, SIGN(5) AS sign_pos FROM users LIMIT 1;

-- RAND
SELECT RAND() AS rand_val FROM users LIMIT 1;
SELECT RAND(42) AS rand_seeded FROM users LIMIT 1;

-- GREATEST
SELECT GREATEST(3, 7, 1, 9, 5) AS greatest_val FROM users LIMIT 1;

-- LEAST
SELECT LEAST(3, 7, 1, 9, 5) AS least_val FROM users LIMIT 1;

-- WIDTH_BUCKET
SELECT WIDTH_BUCKET(age, 20, 50, 5) AS age_bucket FROM users LIMIT 1;
