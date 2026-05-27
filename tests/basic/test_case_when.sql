SELECT name, CASE WHEN age < 30 THEN 'young' WHEN age < 50 THEN 'adult' ELSE 'senior' END AS age_group FROM users ORDER BY age_group, name;
SELECT name, CASE WHEN age < 30 THEN age + 1 ELSE age - 1 END AS age_adj FROM users WHERE age IS NOT NULL ORDER BY age_adj LIMIT 5;
