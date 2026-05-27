# Built-in Functions

gsql 提供 90+ 个 Hive 风格的内置函数，涵盖聚合、窗口、数学、字符串、日期时间、条件判断及杂项函数。

> 函数名不区分大小写。

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
SELECT COUNT(*) AS cnt, SUM(age) AS sum_age, AVG(age) AS avg_age FROM users;
SELECT MIN(price), MAX(price) FROM products;
```

---

## 窗口函数

窗口函数需配合 `OVER()` 子句使用，支持 `PARTITION BY` 和 `ORDER BY`。

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
SELECT name, age, ROW_NUMBER() OVER (ORDER BY age DESC) AS rn FROM users;
SELECT name, age, RANK() OVER (PARTITION BY city ORDER BY age) AS rk FROM users;
SELECT name, age, LAG(age, 1) OVER (ORDER BY age) AS prev_age FROM users;
```

---

## 数学函数

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
| `WIDTH_BUCKET(expr, min, max, buckets)` | 4 | 将值分配到直方图桶。低于 min 返回 0，>= max 返回 buckets+1 |

```sql
SELECT ROUND(3.14159, 2) AS pi;    -- 3.14
SELECT ABS(-5) AS abs_val;          -- 5
SELECT POWER(2, 3) AS cubed;        -- 8
SELECT GREATEST(3, 7, 1, 9) AS max_val;  -- 9
```

---

## 字符串函数

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
| `SPLIT(str, delimiter)` | 2 | 按分隔符切分，返回数组 `[part1,part2,...]` |

```sql
SELECT CONCAT('Hello', ' ', 'World') AS greeting;         -- Hello World
SELECT SUBSTRING('Hello World', 1, 5) AS sub;              -- Hello
SELECT UPPER('hello') AS up;                                -- HELLO
SELECT LENGTH('Hello') AS len;                              -- 5
SELECT REPLACE('hello world', 'world', 'there') AS replaced; -- hello there
SELECT LPAD('hello', 10, '-') AS padded;                    -- -----hello
```

---

## 日期时间函数

支持的日期格式：
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
| `TRUNC(date [, unit])` | 1-2 | 日期截断到指定单位（YEAR/MM/DD/HOUR/MINUTE） |
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
SELECT UNIX_TIMESTAMP('2026-01-15') AS ts;
SELECT UNIX_TIMESTAMP('20260322', 'yyyyMMdd') AS ts_pattern;  -- 自定义格式解析
SELECT FROM_UNIXTIME(1768492800, 'yyyy-MM-dd') AS dt;
SELECT DATE_FORMAT('2026-01-15', 'yyyy/MM/dd') AS fmt;
SELECT YEAR('2026-01-15') AS yr, MONTH('2026-01-15') AS mon, DAY('2026-01-15') AS dy;
SELECT DATEDIFF('2026-01-20', '2026-01-15') AS diff;  -- 5
SELECT DATE_ADD('2026-01-15', 5) AS added;              -- 2026-01-20
SELECT ADD_MONTHS('2026-01-31', 1) AS next_mon;         -- 2026-02-28（月末截断）
SELECT TRUNC('2026-01-15', 'MM') AS month_start;        -- 2026-01-01
```

---

## 条件函数

| 函数 | 参数 | 说明 |
|------|------|------|
| `IF(cond, true_val, false_val)` | 3 | 条件判断。`cond` 为 `true`/`TRUE`/`1`/非零数字时返回 `true_val`，否则返回 `false_val` |
| `COALESCE(val1, val2, ...)` | 1+ | 返回第一个非空（非空字符串）的值 |
| `NVL(val, default)` | 2 | 值为空时返回默认值 |
| `NULLIF(val1, val2)` | 2 | 两值相等返回空，否则返回第一个值 |
| `CAST(val, type)` | 2 | 类型转换。支持 `INT`/`INTEGER`、`BIGINT`、`FLOAT`/`DOUBLE`、`STRING`/`VARCHAR`/`CHAR`、`BOOLEAN` |

```sql
SELECT IF(1, 'true', 'false') AS if_true;           -- true
SELECT COALESCE('', 'default') AS val;               -- default
SELECT COALESCE(name, 'unknown') AS name FROM users;
SELECT NVL('', 'default') AS val;                     -- default
SELECT NULLIF(5, 5) AS eq;                            -- (空)
SELECT NULLIF(5, 3) AS neq;                           -- 5
SELECT CAST(age AS STRING) AS age_str FROM users;
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
SELECT REGEXP_REPLACE('hello 123 world', '\\d+', 'X') AS repl;  -- hello X world
SELECT REGEXP_EXTRACT('hello-123-world', '(\\d+)') AS num;       -- 123
SELECT REGEXP_LIKE('hello123', '\\d+') AS has_digit;             -- true
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
SELECT BASE64('hello') AS b64;        -- aGVsbG8=
SELECT UNBASE64('aGVsbG8=') AS txt;   -- hello
SELECT HEX('hello') AS hx;            -- 68656c6c6f
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
SELECT SHA2('hello', '256') AS sha256;
```

### JSON

| 函数 | 参数 | 说明 |
|------|------|------|
| `GET_JSON_OBJECT(json_str, path)` | 2 | 按 JSONPath 提取值。路径格式 `$.key` 或 `$.key.subkey` |
| `JSON_TUPLE(json_str, key1 [, key2, ...])` | 2+ | 提取指定 key 的值（简化版，返回第一个请求的 key） |

```sql
SELECT GET_JSON_OBJECT('{"key": "value", "num": 42}', '$.key') AS val;  -- value
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
SELECT MASK('Abcd1234') AS msk;                     -- Xxxxnnnn
SELECT MASK('Abcd1234', 'U', 'l', 'd') AS custom;   -- Ullddddd
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
