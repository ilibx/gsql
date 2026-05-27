-- Test aggregate functions
-- Uses users(id,name,email,age,city), products(id,name,category,price,stock), orders(order_id,user_id,product_id,quantity,amount,order_date)

-- COUNT
SELECT COUNT(*) AS cnt FROM users;

-- SUM, AVG, MIN, MAX
SELECT SUM(age) AS sum_age, AVG(age) AS avg_age, MIN(age) AS min_age, MAX(age) AS max_age FROM users;

-- STDDEV, STDDEV_POP, STDDEV_SAMP
SELECT STDDEV(age) AS stddev_age, STDDEV_POP(age) AS stddev_pop_age, STDDEV_SAMP(age) AS stddev_samp_age FROM users;

-- VARIANCE, VAR_POP, VAR_SAMP
SELECT VARIANCE(age) AS var_age, VAR_POP(age) AS var_pop_age, VAR_SAMP(age) AS var_samp_age FROM users;

-- COLLECT_LIST, COLLECT_SET
SELECT COLLECT_LIST(city) AS city_list FROM users;
SELECT COLLECT_SET(city) AS city_set FROM users;

-- CORR, COVAR_POP, COVAR_SAMP
SELECT CORR(price, stock) AS corr_price_stock FROM products;
SELECT COVAR_POP(price, stock) AS covar_pop_price_stock FROM products;
SELECT COVAR_SAMP(price, stock) AS covar_samp_price_stock FROM products;

-- PERCENTILE (use string for decimal)
SELECT PERCENTILE(age, '0.5') AS median_age FROM users;

-- PERCENTILE_APPROX (use string for decimal)
SELECT PERCENTILE_APPROX(age, '0.5') AS approx_median_age FROM users;
SELECT PERCENTILE_APPROX(age, '0.5', '100') AS approx_median_age_prec FROM users;

-- HISTOGRAM_NUMERIC
SELECT HISTOGRAM_NUMERIC(age, 3) AS age_histogram FROM users;
