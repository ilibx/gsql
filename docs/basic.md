# Basic Usage

## 命令行

```bash
# 执行 SQL 文件
gsql -s query.sql

# 从远程存储读取 SQL 文件
gsql -s s3://mybucket/queries/query.sql
gsql -s lark://app_token/folder/query.sql
gsql -s ftp://user:pass@host/path/query.sql
gsql -s https://example.com/query.sql

# 执行内联 SQL 语句
gsql -e "SELECT 1 AS id, 'hello' AS msg"

# 组合使用（先执行 setup.sql，再执行查询）
gsql -s setup.sql -s query.sql

# 调试模式
gsql -v -s query.sql        # 调试级别 1
gsql -vvvvvv -s query.sql   # 调试级别 6
gsql desc -s query.sql      # desc 等价于 -v

# 查看表元信息
gsql desc
```

`-v` 可置于命令任意位置，每增加一个 `v` 提升一级调试级别。

---

## 建表

```sql
CREATE TABLE table_name (
  col1 TYPE,
  col2 TYPE,
  ...
)
WITH (
  storage = 'local',         -- 存储后端
  format = 'csv',            -- 文件格式
  location = '/path/to/dir', -- 数据目录
  file_pattern = '*.csv',    -- 文件匹配模式
  file_name = 'data.csv'     -- 输出文件名（写入时）
);
```

支持的列类型：`INT`、`BIGINT`、`FLOAT`、`DOUBLE`、`STRING`、`BOOLEAN`。

---

## 查询

### 基本查询

```sql
SELECT * FROM users;
SELECT id, name, age FROM users;
SELECT DISTINCT city FROM users;
```

### WHERE 过滤

```sql
SELECT * FROM users WHERE age > 30;
SELECT * FROM users WHERE age >= 25 AND age <= 40;
SELECT * FROM users WHERE city = 'Beijing' OR city = 'Shanghai';
SELECT * FROM users WHERE name LIKE 'A%';
SELECT * FROM users WHERE age IN (28, 31, 35);
SELECT * FROM users WHERE email IS NOT NULL;
```

### 函数调用

SELECT 列表和 WHERE 中支持标量函数调用，且函数可以任意嵌套：

```sql
SELECT UPPER(name) AS name_upper FROM users;
SELECT SUBSTR(name, 1, 3) AS prefix FROM users;
SELECT UPPER(SUBSTR(name, 1, 3)) AS upper_prefix FROM users;
SELECT CONCAT(UPPER(name), ' from ', city) AS descr FROM users;
SELECT ROUND(ABS(age * 1.5), 0) AS rounded FROM users;
```

### 别名

```sql
SELECT u.name AS user_name, u.age FROM users AS u;
SELECT COUNT(*) AS cnt FROM users;
```

### 排序与限制

```sql
SELECT * FROM users ORDER BY age DESC;
SELECT * FROM users ORDER BY age DESC, name ASC;
SELECT * FROM users ORDER BY age LIMIT 5;
```

### 聚合与分组

```sql
SELECT city, COUNT(*) AS cnt, AVG(age) AS avg_age
FROM users
GROUP BY city;

SELECT city, COUNT(*) AS cnt
FROM users
GROUP BY city
HAVING cnt > 1;
```

### JOIN

```sql
SELECT u.name, o.amount
FROM users u
JOIN orders o ON u.id = o.user_id;
```

支持 Hash Join，自动选择小表建哈希。

### 子查询

```sql
SELECT * FROM (
  SELECT id, name, age FROM users WHERE age > 30
) AS adult_users
WHERE name LIKE 'A%';
```

### CTE (WITH 子句)

```sql
WITH adult_users AS (
  SELECT id, name, age FROM users WHERE age > 30
)
SELECT name, age FROM adult_users ORDER BY age;
```

### 窗口函数

```sql
SELECT name, age,
  ROW_NUMBER() OVER (ORDER BY age DESC) AS rn,
  RANK() OVER (PARTITION BY city ORDER BY age) AS rk
FROM users;
```

### UNION ALL

```sql
SELECT name FROM users WHERE id < 10
UNION ALL
SELECT name FROM admin_users;
```

### 无 FROM 查询

```sql
SELECT 1 AS id, 'hello' AS msg, 3.14 AS pi;
```

### VALUES 内联表

```sql
SELECT * FROM (VALUES (1, 'a'), (2, 'b'), (3, 'c')) AS t(id, name)
WHERE id > 1;
```

---

## 插入数据

### INSERT OVERWRITE（覆盖写入）

```sql
INSERT OVERWRITE TABLE result_users
SELECT id, name FROM users WHERE age > 30;
```

### INSERT INTO（追加）

```sql
INSERT INTO TABLE result_users
SELECT id, name FROM users WHERE age <= 30;
```

追加模式会读取已有文件内容，合并后重新写入。

---

## EXPLAIN 执行计划

```sql
EXPLAIN SELECT name, age
FROM users
WHERE age > 25
ORDER BY age
LIMIT 5;
```

输出三层计划：
- **Logical Plan** — 优化前的逻辑计划
- **Optimized Logical Plan** — 经过列裁剪优化后的计划
- **Physical Plan** — 实际执行的物理算子

---

## 分区表

### 建表

```sql
CREATE TABLE events (
  id INT,
  name STRING,
  event_date STRING
)
WITH (
  storage = 'local',
  format = 'csv',
  location = '/tmp/events',
  file_name = 'data.csv'
)
PARTITIONED BY (year, month);
```

### 目录格式

默认使用 `col=value` 格式（如 `year=2024/month=01/data.csv`），可通过 `partition_format = 'value'` 切换为裸值格式（如 `2024/01/data.csv`）。

### 分区裁剪

```sql
-- 等值裁剪
SELECT * FROM partitioned_orders WHERE year = 2026 AND month = '03';

-- 范围裁剪（支持 >, <, >=, <=）
SELECT * FROM partitioned_orders WHERE year >= 2026;

-- 分区列可参与聚合
SELECT month, COUNT(*) FROM partitioned_orders
WHERE year = 2026 GROUP BY month ORDER BY month;
```

写入时自动按分区列值分组写入对应子目录。

---

## Makefile

```bash
make build          # 构建 bin/gsql
make run-sql        # 执行 SQL 文件（默认 tests/check.sql）
make check-sql      # 执行并校验输出
make test           # 运行全部测试
```

## 完整测试

```bash
go test ./...
```

测试分类：
- `tests/*.sql` — 端到端 SQL 集成测试
- `tests/functions/*.sql` — 内置函数 SQL 测试
- `pkg/plan/builtin_test.go` — 函数 Go 单元测试
- `pkg/parser/parser_test.go` — SQL 解析器测试
- `pkg/storage/*_test.go` — 存储适配器测试
