# Built-in Functions

gsql 提供 90+ 个 Hive 风格的内置函数，涵盖聚合、窗口、数学、字符串、日期时间、条件判断及杂项函数。

> 函数名不区分大小写。以下所有示例均可直接执行。

---

## 目录

- [聚合函数](#聚合函数)
- [窗口函数](#窗口函数)
- [数学函数](#数学函数)
- [字符串函数](#字符串函数)
- [日期时间函数](#日期时间函数)
- [条件函数](#条件函数)
- [杂项函数](#杂项函数)
  - [正则表达式](#正则表达式)
  - [编解码](#编解码)
  - [哈希/加密](#哈希加密)
  - [JSON](#json)
  - [数据脱敏](#数据脱敏)
  - [元信息](#元信息)

---

## 聚合函数

需要 `FROM users` / `FROM products` / `FROM orders` 表。

| 函数 | 参数 | 说明 |
|------|------|------|
| `COUNT(*)` / `COUNT(col)` / `COUNT(DISTINCT col)` | 1 | 统计行数或非空值数量 |
| `SUM(col)` | 1 | 计算数值列的总和 |
| `AVG(col)` | 1 | 计算数值列的平均值 |
| `MIN(col)` | 1 | 返回最小值（支持数值和字符串） |
| `MAX(col)` | 1 | 返回最大值（支持数值和字符串） |
| `STDDEV(col)` / `STDDEV_POP(col)` | 1 | 总体标准差 |
| `STDDEV_SAMP(col)` | 1 | 样本标准差（N-1） |
| `VARIANCE(col)` / `VAR_POP(col)` | 1 | 总体方差 |
| `VAR_SAMP(col)` | 1 | 样本方差（N-1） |
| `CORR(col1, col2)` | 2 | 皮尔逊相关系数 |
| `COVAR_POP(col1, col2)` | 2 | 总体协方差 |
| `COVAR_SAMP(col1, col2)` | 2 | 样本协方差 |
| `COLLECT_LIST(col)` | 1 | 收集非空值到有序数组 `[val1,val2,...]` |
| `COLLECT_SET(col)` | 1 | 收集去重非空值到有序数组 |
| `PERCENTILE(col, p)` | 2 | 精确百分位数（插值法）。`p` 为字符串如 `'0.5'` |
| `PERCENTILE_APPROX(col, p [, precision])` | 2-3 | 近似百分位数 |
| `HISTOGRAM_NUMERIC(col, buckets)` | 2 | 数值直方图，返回分桶范围和计数 |

```sql
-- COUNT
SELECT COUNT(*) AS cnt, COUNT(DISTINCT city) AS distinct_cities FROM users;
-- SUM, AVG, MIN, MAX
SELECT SUM(age) AS sum_age, AVG(age) AS avg_age, MIN(age) AS min_age, MAX(age) AS max_age FROM users;
-- STDDEV, VARIANCE
SELECT STDDEV(age) AS stddev_age, STDDEV_POP(age) AS stddev_pop, STDDEV_SAMP(age) AS stddev_samp FROM users;
SELECT VARIANCE(age) AS var_age, VAR_POP(age) AS var_pop, VAR_SAMP(age) AS var_samp FROM users;
-- CORR, COVAR
SELECT CORR(price, stock) AS corr, COVAR_POP(price, stock) AS covar_pop, COVAR_SAMP(price, stock) AS covar_samp FROM products;
-- COLLECT
SELECT COLLECT_LIST(city) AS city_list, COLLECT_SET(city) AS city_set FROM users;
-- PERCENTILE
SELECT PERCENTILE(age, '0.5') AS median_age FROM users;
SELECT PERCENTILE_APPROX(age, '0.5') AS approx_median FROM users;
-- HISTOGRAM
SELECT HISTOGRAM_NUMERIC(age, 3) AS age_histogram FROM users;
```

---

## 窗口函数

需要 `FROM users` 表，配合 `OVER()` 子句使用。

| 函数 | 参数 | 说明 |
|------|------|------|
| `ROW_NUMBER()` | 0 | 行号（1-based） |
| `RANK()` | 0 | 排名（并列留空） |
| `DENSE_RANK()` | 0 | 排名（并列不留空） |
| `LEAD(col [, offset=1 [, default]])` | 1-3 | 访问后续行值 |
| `LAG(col [, offset=1 [, default]])` | 1-3 | 访问前一行值 |
| `NTILE(n)` | 1 | 分桶 |
| `FIRST_VALUE(col)` | 1 | 窗口内第一个值 |
| `LAST_VALUE(col)` | 1 | 窗口内最后一个值（需指定 frame） |
| `NTH_VALUE(col, n)` | 2 | 窗口内第 n 个值 |
| `CUME_DIST()` | 0 | 累积分布 |
| `PERCENT_RANK()` | 0 | 百分比排名 |

```sql
-- ROW_NUMBER, RANK, DENSE_RANK
SELECT name, age,
  ROW_NUMBER() OVER (ORDER BY age DESC) AS rn,
  RANK() OVER (ORDER BY age DESC) AS rk,
  DENSE_RANK() OVER (ORDER BY age DESC) AS drk
FROM users;
-- LEAD, LAG
SELECT name, age,
  LEAD(age, 1) OVER (ORDER BY age) AS next_age,
  LAG(age, 1) OVER (ORDER BY age) AS prev_age
FROM users;
-- NTILE
SELECT name, age, NTILE(4) OVER (ORDER BY age) AS bucket FROM users;
-- FIRST_VALUE, LAST_VALUE, NTH_VALUE
SELECT name, age,
  FIRST_VALUE(name) OVER (ORDER BY age) AS first_name,
  LAST_VALUE(name) OVER (ORDER BY age ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING) AS last_name,
  NTH_VALUE(name, 2) OVER (ORDER BY age ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING) AS second_name
FROM users;
-- CUME_DIST, PERCENT_RANK
SELECT name, age,
  CUME_DIST() OVER (ORDER BY age) AS cum_dist,
  PERCENT_RANK() OVER (ORDER BY age) AS pct_rank
FROM users;
```

---

## 数学函数

无需 FROM 子句，可直接执行。

| 函数 | 参数 | 说明 |
|------|------|------|
| `ROUND(x [, d])` | 1-2 | 四舍五入到 d 位小数 |
| `FLOOR(x)` | 1 | 向下取整 |
| `CEIL(x)` / `CEILING(x)` | 1 | 向上取整 |
| `ABS(x)` | 1 | 绝对值 |
| `SQRT(x)` | 1 | 平方根 |
| `EXP(x)` | 1 | e^x |
| `LN(x)` | 1 | 自然对数 |
| `LOG10(x)` | 1 | 以 10 为底的对数 |
| `LOG2(x)` | 1 | 以 2 为底的对数 |
| `POWER(x, y)` / `POW(x, y)` | 2 | x^y |
| `MOD(x, y)` | 2 | 取模（余数） |
| `SIGN(x)` | 1 | 返回符号：正数 1，负数 -1，零 0 |
| `RAND([seed])` | 0-1 | 返回 [0,1) 随机数 |
| `GREATEST(v1, v2, ...)` | 2+ | 返回最大值 |
| `LEAST(v1, v2, ...)` | 2+ | 返回最小值 |
| `WIDTH_BUCKET(expr, min, max, buckets)` | 4 | 将值分配到直方图桶 |

```sql
SELECT ROUND(3.14159, 2) AS pi;             -- 3.14
SELECT ROUND(3.5) AS rounded;                -- 4
SELECT FLOOR(3.99) AS fl;                    -- 3
SELECT CEIL(3.14) AS cl, CEILING(3.14) AS clg; -- 4, 4
SELECT ABS(-5) AS abs_val, ABS(5) AS abs_pos;  -- 5, 5
SELECT SQRT(100) AS sqrt_val;                -- 10
SELECT EXP(1) AS exp_val;                    -- 2.718...
SELECT LN(2) AS ln_val;                      -- 0.693...
SELECT LOG10(100) AS log10_val;              -- 2
SELECT LOG2(8) AS log2_val;                  -- 3
SELECT POWER(2, 3) AS pow1, POW(2, 3) AS pow2; -- 8, 8
SELECT MOD(10, 3) AS mod_val;                -- 1
SELECT SIGN(0) AS s0, SIGN(5) AS sp, SIGN(-3) AS sn; -- 0, 1, -1
SELECT RAND() AS r1, RAND(42) AS r2;         -- 随机数
SELECT GREATEST(3, 7, 1, 9, 5) AS max_val;   -- 9
SELECT LEAST(3, 7, 1, 9, 5) AS min_val;      -- 1
SELECT WIDTH_BUCKET(25, 20, 50, 5) AS bucket; -- 1（20-26 为第 1 桶，每桶跨度 6）
```

---

## 字符串函数

无需 FROM 子句，可直接执行。

| 函数 | 参数 | 说明 |
|------|------|------|
| `CONCAT(str1, str2, ...)` | 1+ | 拼接字符串（无分隔符） |
| `CONCAT_WS(sep, str1, str2, ...)` | 2+ | 带分隔符拼接 |
| `SUBSTRING(str, pos [, len])` | 2-3 | 截取子串，位置从 1 开始 |
| `SUBSTR(str, pos [, len])` | 2-3 | SUBSTRING 别名 |
| `UPPER(str)` / `UCASE(str)` | 1 | 转大写 |
| `LOWER(str)` / `LCASE(str)` | 1 | 转小写 |
| `TRIM(str)` | 1 | 去除首尾空白 |
| `LTRIM(str)` | 1 | 去除左侧空白 |
| `RTRIM(str)` | 1 | 去除右侧空白 |
| `LENGTH(str)` | 1 | 字符数 |
| `REPLACE(str, from, to)` | 3 | 替换子串 |
| `REVERSE(str)` | 1 | 反转字符串 |
| `LOCATE(substr, str [, pos])` | 2-3 | 查找子串位置（1-based），未找到返回 0 |
| `INSTR(str, substr)` | 2 | LOCATE 别名 |
| `LPAD(str, len, pad)` | 3 | 左侧填充至指定长度 |
| `RPAD(str, len, pad)` | 3 | 右侧填充至指定长度 |
| `INITCAP(str)` | 1 | 首字母大写 |
| `ASCII(str)` | 1 | 首字符的 Unicode 码点 |
| `SPLIT(str, delimiter)` | 2 | 按分隔符切分，返回数组 |

```sql
SELECT CONCAT('Hello', ' ', 'World') AS greeting;            -- Hello World
SELECT CONCAT_WS('-', '2026', '01', '15') AS date_str;       -- 2026-01-15
SELECT SUBSTRING('Hello World', 1, 5) AS sub1;               -- Hello
SELECT SUBSTR('Hello World', 7) AS sub2;                     -- World
SELECT UPPER('hello') AS u1, UCASE('world') AS u2;           -- HELLO, WORLD
SELECT LOWER('HELLO') AS l1, LCASE('WORLD') AS l2;           -- hello, world
SELECT TRIM('  hello  ') AS trimmed;                          -- hello
SELECT LTRIM('  hello  ') AS ltrimmed;                        -- "hello  "
SELECT RTRIM('  hello  ') AS rtrimmed;                        -- "  hello"
SELECT LENGTH('Hello') AS len;                                -- 5
SELECT REPLACE('hello world', 'world', 'there') AS replaced;  -- hello there
SELECT REVERSE('hello') AS rev;                               -- olleh
SELECT LOCATE('world', 'hello world') AS loc1;                -- 7
SELECT LOCATE('l', 'hello', 3) AS loc2;                       -- 3
SELECT INSTR('hello world', 'world') AS instr_val;            -- 7
SELECT LPAD('hello', 10, '-') AS lpad_val;                    -- -----hello
SELECT RPAD('hello', 10, '-') AS rpad_val;                    -- hello-----
SELECT INITCAP('hello world') AS initcap_val;                  -- Hello World
SELECT ASCII('A') AS ascii_val;                               -- 65
SELECT SPLIT('a,b,c', ',') AS split_val;                      -- [a,b,c]
```

---

## 日期时间函数

无需 FROM 子句，可直接执行。

支持的日期解析格式：
- `yyyy-MM-dd`（如 `2026-01-15`）
- `yyyy-MM-dd HH:mm:ss`（如 `2026-01-15 10:30:00`）
- `yyyy-MM-ddTHH:mm:ssZ`（ISO 8601）
- `yyyy/MM/dd`
- `MM/dd/yyyy`
- RFC3339

| 函数 | 参数 | 说明 |
|------|------|------|
| `CURRENT_DATE()` | 0 | 当前日期 `YYYY-MM-DD` |
| `CURRENT_TIMESTAMP()` | 0 | 当前时间戳 `YYYY-MM-DD HH:MM:SS` |
| `UNIX_TIMESTAMP()` | 0 | 当前 Unix 时间戳（秒） |
| `UNIX_TIMESTAMP(str)` | 1 | 将日期字符串解析为时间戳 |
| `UNIX_TIMESTAMP(str, fmt)` | 2 | 按自定义格式解析日期为时间戳 |
| `FROM_UNIXTIME(ts)` | 1 | 时间戳转日期字符串 |
| `FROM_UNIXTIME(ts, fmt)` | 2 | 时间戳按格式输出 |
| `TO_DATE(str)` | 1 | 提取日期部分 `YYYY-MM-DD` |
| `YEAR(date)` | 1 | 提取年份 |
| `MONTH(date)` | 1 | 提取月份 (1-12) |
| `DAY(date)` / `DAYOFMONTH(date)` | 1 | 提取天数 (1-31) |
| `HOUR(ts)` | 1 | 提取小时 (0-23) |
| `MINUTE(ts)` | 1 | 提取分钟 (0-59) |
| `SECOND(ts)` | 1 | 提取秒 (0-59) |
| `WEEKOFYEAR(date)` | 1 | ISO 周数 |
| `DATEDIFF(end, start)` | 2 | 日期差（天） |
| `DATE_ADD(date, n)` | 2 | 日期加 n 天 |
| `DATE_SUB(date, n)` | 2 | 日期减 n 天 |
| `DATE_FORMAT(date, fmt)` | 2 | 日期格式化输出 |
| `ADD_MONTHS(date, n)` | 2 | 加 n 个月（处理月末截断） |
| `LAST_DAY(date)` | 1 | 当月最后一天 |
| `NEXT_DAY(date, day_name)` | 2 | 下一个指定星期几 |
| `MONTHS_BETWEEN(end, start)` | 2 | 月份差（含小数天数） |
| `QUARTER(date)` | 1 | 季度 (1-4) |
| `TRUNC(date [, unit])` | 1-2 | 日期截断到指定单位 |
| `FROM_UTC_TIMESTAMP(ts, tz)` | 2 | UTC 时间戳转目标时区 |
| `TO_UTC_TIMESTAMP(ts, tz)` | 2 | 源时区时间戳转 UTC |

**Pattern 格式**（用于 `UNIX_TIMESTAMP`、`FROM_UNIXTIME`、`DATE_FORMAT`）：
| 符号 | 含义 |
|------|------|
| `yyyy` | 四位年份 |
| `yy` | 两位年份 |
| `MM` | 两位月份 |
| `M` | 月份（不补零） |
| `dd` | 两位日期 |
| `d` | 日期（不补零） |
| `HH` | 24小时制 |
| `mm` | 分钟 |
| `ss` | 秒 |
| `SSS` | 毫秒 |

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
SELECT DATEDIFF('2026-01-20', '2026-01-15') AS diff;       -- 5
SELECT DATE_ADD('2026-01-15', 5) AS added;                  -- 2026-01-20
SELECT DATE_SUB('2026-01-15', 5) AS subd;                   -- 2026-01-10
SELECT DATE_FORMAT('2026-01-15', 'yyyy/MM/dd') AS fmt;      -- 2026/01/15
SELECT ADD_MONTHS('2026-01-15', 2) AS added_mon;            -- 2026-03-15
SELECT ADD_MONTHS('2026-01-31', 1) AS next_mon;             -- 2026-02-28（月末截断）
SELECT LAST_DAY('2026-02-15') AS last_day;                  -- 2026-02-28
SELECT NEXT_DAY('2026-01-15', 'Monday') AS next_mon;        -- 2026-01-19
SELECT MONTHS_BETWEEN('2026-03-15', '2026-01-15') AS mons;  -- 2.0
SELECT QUARTER('2026-04-15') AS qtr;                        -- 2
SELECT TRUNC('2026-01-15', 'MM') AS trunc_mm;               -- 2026-01-01
SELECT TRUNC('2026-01-15', 'YY') AS trunc_yy;               -- 2026-01-01
SELECT FROM_UTC_TIMESTAMP('2026-01-15 10:00:00', 'Asia/Shanghai') AS utc_to_sh;
SELECT TO_UTC_TIMESTAMP('2026-01-15 18:00:00', 'Asia/Shanghai') AS sh_to_utc;
```

---

## 条件函数

无需 FROM 子句（除 `CAST(col FROM table)` 外）。

| 函数 | 参数 | 说明 |
|------|------|------|
| `IF(cond, true_val, false_val)` | 3 | 条件判断。`cond` 为 `true`/`TRUE`/`1`/非零数字时返回 `true_val`，否则返回 `false_val` |
| `COALESCE(val1, val2, ...)` | 1+ | 返回第一个非空（非空字符串）的值 |
| `NVL(val, default)` | 2 | 值为空时返回默认值 |
| `NULLIF(val1, val2)` | 2 | 两值相等返回空，否则返回第一个值 |
| `CAST(val, type)` | 2 | 类型转换。支持 `INT`/`INTEGER`、`BIGINT`、`FLOAT`/`DOUBLE`、`STRING`/`VARCHAR`/`CHAR`、`BOOLEAN` |

```sql
SELECT IF(1, 'true', 'false') AS if_true;             -- true
SELECT IF(0, 'true', 'false') AS if_false;            -- false
SELECT COALESCE('', 'default') AS coalesce_val;       -- default
SELECT NVL('', 'default') AS nvl_val;                 -- default
SELECT NULLIF(5, 5) AS nullif_equal;                  -- (空)
SELECT NULLIF(5, 3) AS nullif_diff;                   -- 5
SELECT CAST(123 AS STRING) AS cast_val;               -- 123
SELECT CAST('3.14' AS FLOAT) AS cast_float;           -- 3.14
```

---

## 杂项函数

### 正则表达式

| 函数 | 参数 | 说明 |
|------|------|------|
| `REGEXP_REPLACE(str, pattern, replacement)` | 3 | 正则替换 |
| `REGEXP_EXTRACT(str, pattern [, group=1])` | 2-3 | 正则提取捕获组 |
| `REGEXP_LIKE(str, pattern)` | 2 | 正则匹配测试，返回 `"true"`/`"false"` |

```sql
SELECT REGEXP_REPLACE('hello 123 world', '\\d+', 'X') AS repl;   -- hello X world
SELECT REGEXP_EXTRACT('hello-123-world', '(\\d+)') AS num;        -- 123
SELECT REGEXP_EXTRACT('hello-123-world', '([a-z]+)-\\d+-([a-z]+)', 2) AS grp2; -- world
SELECT REGEXP_LIKE('hello123', '\\d+') AS has_digit;              -- true
SELECT REGEXP_LIKE('hello', '\\d+') AS no_digit;                  -- false
```

### 编解码

| 函数 | 参数 | 说明 |
|------|------|------|
| `BASE64(str)` | 1 | Base64 编码 |
| `UNBASE64(str)` | 1 | Base64 解码 |
| `HEX(str)` | 1 | 十六进制编码 |
| `UNHEX(str)` | 1 | 十六进制解码 |
| `ENCODE(str, charset)` | 2 | 按字符集编码（非 UTF-8 返回十六进制） |
| `DECODE(str, charset)` | 2 | 按字符集解码 |

```sql
SELECT BASE64('hello') AS b64;              -- aGVsbG8=
SELECT UNBASE64('aGVsbG8=') AS txt;         -- hello
SELECT HEX('hello') AS hx;                  -- 68656c6c6f
SELECT UNHEX('68656c6c6f') AS unhx;         -- hello
SELECT ENCODE('hello', 'UTF-8') AS enc;
SELECT DECODE('hello', 'UTF-8') AS dec;
```

### 哈希/加密

| 函数 | 参数 | 说明 |
|------|------|------|
| `MD5(str)` | 1 | MD5 哈希 |
| `SHA1(str)` | 1 | SHA-1 哈希 |
| `SHA2(str, bit_len)` | 2 | SHA-2 哈希（224/256/384/512） |
| `CRC32(str)` | 1 | CRC32 校验和 |
| `HASH(str1 [, str2, ...])` | 1+ | 简单哈希（字符串 char code 累加 * 31） |

```sql
SELECT MD5('hello') AS md5_hash;
SELECT SHA1('hello') AS sha1_hash;
SELECT SHA2('hello', '256') AS sha2_256;
SELECT SHA2('hello', '512') AS sha2_512;
SELECT CRC32('hello') AS crc;
SELECT HASH('hello') AS h;
SELECT HASH(name, email) AS h_multi FROM users;
```

### JSON

| 函数 | 参数 | 说明 |
|------|------|------|
| `GET_JSON_OBJECT(json_str, path)` | 2 | 按 JSONPath 提取值。路径格式 `$.key` 或 `$.key.subkey` |
| `JSON_TUPLE(json_str, key1 [, key2, ...])` | 2+ | 提取指定 key 的值（简化版，返回第一个请求的 key） |

```sql
SELECT GET_JSON_OBJECT('{"key": "value", "num": 42}', '$.key') AS val;   -- value
SELECT GET_JSON_OBJECT('{"key": "value", "num": 42}', '$.num') AS num;   -- 42
SELECT JSON_TUPLE('{"key": "value", "num": 42}', 'key') AS tuple_val;    -- value
```

### 数据脱敏

| 函数 | 参数 | 说明 |
|------|------|------|
| `MASK(str [, upper, lower, digit])` | 1-4 | 脱敏。默认大写→X，小写→x，数字→n |
| `MASK_FIRST_N(str, n)` | 2 | 仅脱敏前 n 个字符 |
| `MASK_LAST_N(str, n)` | 2 | 仅脱敏后 n 个字符 |
| `MASK_SHOW_FIRST_N(str, n)` | 2 | 保留前 n 个字符不脱敏 |
| `MASK_SHOW_LAST_N(str, n)` | 2 | 保留后 n 个字符不脱敏 |

```sql
SELECT MASK('Abcd1234') AS msk;                          -- Xxxxnnnn
SELECT MASK('Abcd1234', 'U', 'l', 'd') AS msk_custom;    -- Ullddddd
SELECT MASK_FIRST_N('Abcd1234', 3) AS msk_first;         -- XXXd1234
SELECT MASK_LAST_N('Abcd1234', 3) AS msk_last;           -- Abcd1XXX
SELECT MASK_SHOW_FIRST_N('Abcd1234', 3) AS show_first;   -- Abcnnnnn
SELECT MASK_SHOW_LAST_N('Abcd1234', 3) AS show_last;     -- Xxxx234
```

### 元信息

| 函数 | 参数 | 说明 |
|------|------|------|
| `CURRENT_USER()` | 0 | 返回当前用户名（`gsql_user`） |
| `CURRENT_DATABASE()` | 0 | 返回当前数据库（`default`） |
| `VERSION()` | 0 | 返回引擎版本（`gsql 0.1.0`） |

```sql
SELECT CURRENT_USER() AS usr, CURRENT_DATABASE() AS db, VERSION() AS ver;
```
