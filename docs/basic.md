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

> **⚠️ URL 中的 `&` 需要引号保护**：Shell 中 `&` 是后台运行符，含有查询参数的 URL 必须用引号包裹：
> ```bash
> # ❌ 错误：& 被 shell 解析为后台任务
> gsql -s lark://folder/file.sql?app_id=xxx&app_secret=yyy
> # [1] 12345  → 只运行了前半段
>
> # ✅ 正确：单引号保护整个 URL
> gsql -s 'lark://folder/file.sql?app_id=xxx&app_secret=yyy'
>
> # ✅ 双引号也可以
> gsql -s "lark://folder/file.sql?app_id=xxx&app_secret=yyy"
> ```

---

## 模板 SQL（Jinja2）

使用 `-t` 加载 SQL 模板文件，`-a` 传入参数，模板渲染后再执行。

### 基本参数

```bash
# 加载模板文件并用参数填充
gsql -t query.sql -a "min_age=30" -a "city=Beijing"

# 支持 local:// 前缀
gsql -t local://path/to/query.sql -a "key=value"
```

模板文件 `query.sql`：
```sql
SELECT id, name, age
FROM users
WHERE age >= {{ min_age }} AND city = '{{ city }}'
ORDER BY id;
```

### 重复参数自动合并

同一个 key 多次使用 `-a`，会自动合并为逗号分隔的字符串。常用于 `IN` 查询：

```bash
gsql -t export.sql -a "month=2026-05" -a "user=alice" -a "user=bob" -a "user=charlie"
```

模板文件 `export.sql`：
```sql
-- 模板中用 |safe 防止单引号被转义
WHERE month = '{{ month }}' AND username in ('{{ user|safe }}')
```

等价于传入一个参数：`username in('alice','bob','charlie')`

> 注意：模板中引用合并参数时必须加 `|safe` 过滤器，否则 pongo2 会将 `'` 转义为 `&#39;`。

### JSON 数组参数（批量插入）

`-a` 的值如果是 JSON 数组或对象，会自动解析为结构化数据，可在模板中用 `{% for %}` 遍历：

```bash
# 内联 JSON 数组
gsql -t batch_insert.sql -a "rows=[{\"id\":1,\"name\":\"Alice\"},{\"id\":2,\"name\":\"Bob\"}]"

# 从 JSON 文件加载（推荐）
gsql -t batch_insert.sql -a "rows=@data.json" -a "min_age=18"
```

模板文件 `batch_insert.sql`：
```sql
CREATE TABLE users (id INT, name STRING, age INT)
WITH (storage = 'local', format = 'csv', path = 'output');

{% for row in rows %}
INSERT INTO users VALUES ({{ row.id }}, '{{ row.name }}', {{ row.age }});
{% endfor %}

SELECT * FROM users WHERE age >= {{ min_age }};
```

JSON 文件 `data.json`：
```json
[
  {"id": 10, "name": "Alice", "age": 25, "city": "Beijing"},
  {"id": 11, "name": "Bob",   "age": 30, "city": "Shanghai"}
]
```

### JSON 文件引用

通过 `@` 前缀引用外部 JSON 文件：

```bash
gsql -t batch.sql -a "rows=@data.json" -a "config=@config.json"
```

### 条件渲染

支持 Jinja2 的 `{% if %}` 条件判断：

```sql
SELECT * FROM users
{% if min_age %}
WHERE age >= {{ min_age }}
{% endif %}
ORDER BY {{ order_by|default('id') }};
```

### -t 支持的 URL 协议

与 `-s` 相同，支持所有存储后端 URL：

```bash
# 本地文件
gsql -t local://path/to/query.sql -a "x=1"

# 对象存储
gsql -t s3://bucket/template.sql -a "param=value"

# 飞书文档
gsql -t lark://app_token/folder/template.sql -a "env=prod"

# HTTP 远程文件
gsql -t https://example.com/tpl.sql -a "mode=test"

# FTP / SFTP
gsql -t ftp://user:pass@host/path/tpl.sql -a "key=val"
gsql -t sftp://user@host/path/tpl.sql -a "key=val"

# WebDAV / Git LFS
gsql -t webdav://host/path/tpl.sql -a "x=1"
gsql -t gitlfs://host/repo/tpl.sql -a "x=1"
```

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

### 列默认值（DEFAULT）

列定义支持可选的 `DEFAULT` 子句，字面量可以是字符串、数字（可带正负号）、`NULL` 或裸词（如 `true`）：

```sql
CREATE TABLE metrics (
  id      INT DEFAULT 0,
  name    STRING DEFAULT 'unknown',
  amount  INT DEFAULT -1,
  active  BOOLEAN DEFAULT true,
  note    STRING DEFAULT NULL
)
WITH (storage = 'local', format = 'csv', path = '/data/metrics');
```

默认值在以下场景生效：

- **读取数据**：CSV/JSON/Excel 行缺少该列时填入默认值（已存在但为空的单元格视为显式空值，不替换）
- **INSERT ... VALUES**：VALUES 行的列数少于表列数时，缺失列用默认值填充；缺失列没有 `DEFAULT` 或列数超出表列数，均报列数不匹配
- **INSERT ... SELECT**：SELECT 产出的列少于目标表列数时，未覆盖的目标列写入默认值
- **外连接填充**：LEFT/RIGHT/FULL/SEMI 未匹配行的缺失侧优先使用列 `DEFAULT`，未声明时按类型回退（数值→`0`，布尔→`false`，其他→空）

### WITH 选项值

选项值支持三种定界符：

| 定界符 | 示例 | 说明 |
|--------|------|------|
| 单引号 | `storage = 'local'` | 常规选项，不含引号的简短值 |
| 双引号 | `format = "csv"` | 同单引号，值内含单引号时可用 |
| 反引号 | `` query = `SELECT * FROM t WHERE name = 'foo'` `` | 多行或值内含单/双引号时使用 |

反引号内部可以自由使用 `'`、`"` 和换行，无需转义。

### SSH 跳板（SSH 隧道）

数据库和远程存储（FTP、S3）可以通过 SSH 跳板机访问：

```sql
CREATE TABLE mydb (id INT, name STRING)
WITH (
  storage = 'mysql',
  host = '192.168.1.100',
  port = '3306',
  username = 'root',
  password = 'secret',
  database = 'mydb',
  ssh_host = 'jump.example.com',    -- SSH 跳板机地址
  ssh_port = '22',                   -- SSH 端口（默认 22）
  ssh_user = 'jumper',               -- SSH 用户名
  ssh_password = 'ssh_pass',         -- SSH 密码（与 ssh_key 二选一）
  ssh_key = '/path/to/id_rsa',      -- SSH 密钥路径
  ssh_key_passphrase = ''           -- 密钥口令（可选）
);
```

URL 方式同样支持：

```sql
CREATE TABLE mydb (id INT, name STRING)
WITH (
  url = 'mysql://root:secret@192.168.1.100:3306/mydb',
  ssh_host = 'jump.example.com',
  ssh_user = 'jumper',
  ssh_key = '/path/to/id_rsa'
);
```

SSH 隧道会将数据库或存储的连接通过跳板机转发到目标内网地址。

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

### 字符串拼接

除了 `CONCAT()` 函数外，gsql 还支持 PostgreSQL 风格的 `||` 操作符：

```sql
SELECT name || ' from ' || city AS descr FROM users;
SELECT 'Hello' || ' ' || 'World' AS greeting;              -- Hello World
SELECT MD5(name || '@' || city) AS hash FROM users;
```

`||` 可与算术运算符混用，支持嵌套在函数中：
```sql
SELECT CONCAT(UPPER(name), '@', city) AS email FROM users;
SELECT MD5(CONCAT(name, '-', city)) FROM users;
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

DISTINCT 聚合：
```sql
SELECT COUNT(DISTINCT city) AS city_cnt FROM users;
SELECT COUNT(DISTINCT city), SUM(age) FROM users;
```

### JOIN

```sql
SELECT u.name, o.amount
FROM users u
JOIN orders o ON u.id = o.user_id;
```

支持所有 JOIN 类型：`INNER JOIN`（可省略 `INNER`）、`LEFT [OUTER] JOIN`、`RIGHT [OUTER] JOIN`、`FULL [OUTER] JOIN`、`LEFT SEMI JOIN`、`CROSS JOIN`。

gsql 自动选择小表构建哈希表（Hash Join），且 ON 列方向可自动检测并交换：

```sql
-- LEFT JOIN（匹配不到时右表列为空）
SELECT u.name, o.amount FROM users u LEFT JOIN orders o ON u.id = o.user_id;

-- RIGHT JOIN
SELECT u.name, o.amount FROM users u RIGHT JOIN orders o ON u.id = o.user_id;

-- FULL JOIN
SELECT u.name, o.amount FROM users u FULL JOIN orders o ON u.id = o.user_id;

-- LEFT SEMI JOIN（只返回左表行，不重复）
SELECT name FROM users LEFT SEMI JOIN orders ON users.id = orders.user_id;

-- CROSS JOIN（笛卡尔积）
SELECT name, amount FROM users CROSS JOIN orders;

-- 多表 JOIN
SELECT u.name, p.name AS product, o.amount
FROM users u
JOIN orders o ON u.id = o.user_id
JOIN products p ON o.product_id = p.id
WHERE o.amount > 500;
```

> **类型感知比较**：JOIN 的 ON 条件自动处理类型转换——`INT` 列去掉前导零（`001` 匹配 `1`），`DECIMAL` 去掉尾零（`10.50` 匹配 `10.5`），`STRING` 去除首尾空格。

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

### BETWEEN

```sql
SELECT * FROM users WHERE age BETWEEN 25 AND 35;
SELECT * FROM users WHERE age NOT BETWEEN 40 AND 50;
```

### INSERT ... VALUES

```sql
INSERT OVERWRITE TABLE users VALUES (1, 'Alice', 'alice@example.com', 28, 'Beijing');

INSERT INTO TABLE users
VALUES (2, 'Bob', 'bob@example.com', 35, 'Shanghai'),
       (3, 'Charlie', 'charlie@example.com', 42, 'Shenzhen');
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

## 内置函数

详见 [functions.md](functions.md)。常用汇总：

| 类别 | 函数 |
|------|------|
| 字符串 | `CONCAT`, `CONCAT_WS`, `SUBSTRING`, `UPPER`, `LOWER`, `TRIM`, `REPLACE`, `REVERSE`, `LPAD`, `RPAD`, `INITCAP`, `SPLIT` |
| 数学 | `ROUND`, `FLOOR`, `CEIL`, `ABS`, `SQRT`, `POWER`, `MOD`, `RAND`, `GREATEST`, `LEAST` |
| 日期 | `CURRENT_DATE`, `UNIX_TIMESTAMP`, `FROM_UNIXTIME`, `DATEDIFF`, `DATE_ADD`, `DATE_FORMAT`, `EXTRACT` |
| 条件 | `IF`, `COALESCE`, `NVL`, `NULLIF`, `CAST` |
| 聚合 | `COUNT`, `SUM`, `AVG`, `MIN`, `MAX`, `STDDEV`, `VARIANCE`, `COLLECT_LIST`, `PERCENTILE` |
| 窗口 | `ROW_NUMBER`, `RANK`, `DENSE_RANK`, `LEAD`, `LAG`, `NTILE`, `FIRST_VALUE`, `LAST_VALUE` |
| 正则 | `REGEXP_REPLACE`, `REGEXP_EXTRACT`, `REGEXP_LIKE` |
| 编解码 | `BASE64`, `UNBASE64`, `HEX`, `UNHEX`, `ENCODE`, `DECODE` |
| 哈希 | `MD5`, `SHA1`, `SHA2`, `CRC32`, `HASH` |
| JSON | `GET_JSON_OBJECT`, `JSON_TUPLE` |
| 脱敏 | `MASK`, `MASK_FIRST_N`, `MASK_LAST_N`, `MASK_SHOW_FIRST_N`, `MASK_SHOW_LAST_N` |
| 杂项 | `CURRENT_USER`, `CURRENT_DATABASE`, `VERSION` |

字符串拼接还支持 PostgreSQL 风格操作符：

```sql
SELECT name || ' from ' || city AS descr FROM users;
SELECT MD5(CONCAT(name, '@', city)) AS hash FROM users;
```

所有函数支持任意层级嵌套：

```sql
SELECT UPPER(CONCAT(SUBSTR(name, 1, 1), '. ', city)) FROM users LIMIT 1;
SELECT ROUND(ABS(age * 1.5), 0) FROM users LIMIT 1;
```

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
