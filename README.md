# gsql

`gsql` 是一个基于 `Go` 实现的 Hive SQL 风格数据查询框架，面向本地与远程文件数据源，提供类似 Hive 的 SQL 建表、查询、聚合、分区裁剪和结果导出能力。

## 核心特性

- 基于 `Golang` 实现的 SQL 查询引擎
- Hive SQL 风格的表定义与查询语法
- 支持 `CSV`、`JSON` 文件格式
- 支持多种数据源访问：
  - 本地文件系统
  - `S3`
  - `FTP`
  - `SFTP`
  - `WebDAV`
  - `Git LFS`
- 支持 SQL 查询语法：`WITH`、`SELECT`、`JOIN`、`WHERE`、`GROUP BY`、`ORDER BY`、`LIMIT`
- 支持窗口函数与分组聚合
- 支持查询结果导出为 `CSV`、`JSON`

## 架构设计参考（Hive SQL 风格）

`gsql` 的架构可参考 Hive 的功能模块，并在 Go 语言环境下实现核心能力：

1. 解析层：
   - SQL 解析与语法树生成
   - 支持 `CREATE TABLE ... WITH(...)`、查询语句、CTE、窗口函数
2. 元数据层：
   - 表结构定义与数据源映射
   - 管理表字段、字段类型、分区字段、数据源属性
3. 存储访问层：
   - 文件格式解析：`CSV`、`JSON`
   - 数据源适配：本地、S3、FTP、SFTP、WebDAV、Git LFS
4. 计算层：
   - SQL 执行计划生成
   - 过滤、投影、连接、聚合、排序、窗口计算
   - 分区裁剪与算子下推
5. 导出层：
   - 查询结果输出为文件或标准输出
   - 支持 `CSV`、`JSON` 导出格式

## 功能说明

### 1. 表定义与数据源映射

使用 `CREATE TABLE ... WITH (...)` 语法，声明表结构、字段类型、数据源位置与访问方式。类似 Hive 到外部表的映射方式，但简化为直接指定文件路径与格式。

示例：

```sql
CREATE TABLE users (
  id INT,
  name STRING,
  email STRING,
  region STRING,
  created_at TIMESTAMP
)
WITH (
  storage = 'local',
  format = 'csv',
  location = '/data/users',
  file_pattern = '*.csv'
);
```

### 2. 支持的数据格式

当前支持：

- `CSV`
- `JSON`

### 3. 支持的数据源

支持以下协议与存储类型：

- `local`
- `s3`
- `ftp`
- `sftp`
- `webdav`
- `git lfs`

### 4. 查询能力

支持 Hive SQL 风格查询能力，包括：

- 公共表表达式（CTE）
- 多表关联（`JOIN`）
- 过滤条件（`WHERE`）
- 聚合与分组（`GROUP BY`）
- 排序（`ORDER BY`）
- 限制（`LIMIT`）
- 窗口函数（`OVER`）
- 分区裁剪与分区推理

与 Hive 的关键差异：

- 不依赖 Hive Metastore、YARN 或 Hadoop 生态
- 直接针对文件目录与远程对象存储进行查询
- 重点为轻量数据探索与导出，而非大规模分布式计算

## 示例 SQL

```sql
-- users 表，CSV 本地文件
CREATE TABLE users (
  id INT,
  name STRING,
  email STRING,
  region STRING,
  created_at TIMESTAMP
)
WITH (
  storage = 'local',
  format = 'csv',
  location = '/path/to/users',
  file_pattern = '*.csv'
);

-- orders 表，JSON S3 存储
CREATE TABLE orders (
  order_id INT,
  user_id INT,
  amount FLOAT,
  order_date DATE,
  status STRING
)
WITH (
  storage = 's3',
  format = 'json',
  location = 's3://bucket/orders',
  region = 'us-west-2'
);

-- logs 表，CSV SFTP 存储
CREATE TABLE logs (
  file_path STRING,
  user_id INT,
  event_type STRING,
  event_time TIMESTAMP
)
WITH (
  storage = 'sftp',
  format = 'csv',
  location = 'sftp://example.com/data/logs',
  username = 'readonly',
  password = '***'
);

WITH recent_orders AS (
  SELECT *
  FROM orders
  WHERE order_date >= '2026-01-01'
    AND status = 'completed'
),
user_order_summary AS (
  SELECT
    user_id,
    COUNT(order_id) AS order_count,
    SUM(amount) AS total_amount,
    MAX(order_date) AS last_order_date
  FROM recent_orders
  GROUP BY user_id
),
user_activity AS (
  SELECT
    u.id AS user_id,
    u.region,
    u.name,
    u.email,
    os.order_count,
    os.total_amount,
    os.last_order_date,
    CASE
      WHEN os.order_count >= 10 THEN 'top'
      WHEN os.order_count >= 5 THEN 'active'
      ELSE 'normal'
    END AS customer_segment
  FROM users u
  JOIN user_order_summary os ON u.id = os.user_id
)
SELECT
  ua.region,
  ua.name,
  ua.order_count,
  ua.total_amount,
  ua.last_order_date,
  ua.customer_segment,
  RANK() OVER (PARTITION BY ua.region ORDER BY ua.total_amount DESC) AS region_rank,
  COUNT(l.event_type) AS event_count,
  MAX(l.event_time) AS last_event_time
FROM user_activity ua
LEFT JOIN logs l ON ua.user_id = l.user_id
WHERE ua.region IS NOT NULL
  AND ua.last_order_date >= '2026-01-01'
GROUP BY
  ua.region,
  ua.name,
  ua.order_count,
  ua.total_amount,
  ua.last_order_date,
  ua.customer_segment
ORDER BY ua.region, ua.total_amount DESC
LIMIT 20;
```

### 先建结果表，再插入结果表的示例

以下示例更贴近 Hive 的执行方式：先创建目标结果表，再将查询结果写入该表。

```sql
-- 1. 先声明结果表结构并指定目录存储位置
CREATE EXTERNAL TABLE result_user_summary (
  region STRING,
  name STRING,
  order_count BIGINT,
  total_amount DOUBLE,
  last_order_date DATE,
  customer_segment STRING
)
STORED AS TEXTFILE
LOCATION '/export/results/users_summary';

-- 2. 将查询结果写入结果表
INSERT OVERWRITE TABLE result_user_summary
SELECT
  ua.region,
  ua.name,
  ua.order_count,
  ua.total_amount,
  ua.last_order_date,
  ua.customer_segment
FROM user_activity ua
WHERE ua.order_count > 0
ORDER BY ua.total_amount DESC;
```

```sql
-- gsql 风格：先定义结果表并写入
CREATE TABLE result_user_summary (
  region STRING,
  name STRING,
  order_count INT,
  total_amount FLOAT,
  last_order_date DATE,
  customer_segment STRING
)
WITH (
  storage = 'local',
  format = 'csv',
  location = '/export/results/users_summary',
  file_name = 'users_summary.csv'
);

INSERT OVERWRITE TABLE result_user_summary
SELECT
  ua.region,
  ua.name,
  ua.order_count,
  ua.total_amount,
  ua.last_order_date,
  ua.customer_segment
FROM user_activity ua
WHERE ua.order_count > 0
ORDER BY ua.total_amount DESC;
```

```sql
-- 先定义 JSON 导出结果表，再写入它
CREATE TABLE result_user_activity_json (
  user_id INT,
  region STRING,
  name STRING,
  order_count INT,
  total_amount FLOAT,
  customer_segment STRING
)
WITH (
  storage = 'local',
  format = 'json',
  location = '/export/results/user_activity_json',
  file_name = 'user_activity.json'
);

INSERT OVERWRITE TABLE result_user_activity_json
SELECT
  ua.user_id,
  ua.region,
  ua.name,
  ua.order_count,
  ua.total_amount,
  ua.customer_segment
FROM user_activity ua
WHERE ua.region IS NOT NULL;
```

```sql
-- 复杂示例：使用 WITH 子查询构建中间结果，并将最终结果写入目标表
CREATE EXTERNAL TABLE result_user_order_summary (
  user_id INT,
  region STRING,
  name STRING,
  order_count BIGINT,
  total_amount DOUBLE,
  avg_amount DOUBLE,
  max_order_date DATE,
  customer_segment STRING,
  region_rank BIGINT
)
STORED AS TEXTFILE
LOCATION '/export/results/user_order_summary';

WITH recent_orders AS (
  SELECT
    order_id,
    user_id,
    amount,
    order_date,
    status
  FROM orders
  WHERE order_date >= '2026-01-01'
    AND status = 'completed'
),
user_order_stats AS (
  SELECT
    user_id,
    COUNT(*) AS order_count,
    SUM(amount) AS total_amount,
    AVG(amount) AS avg_amount,
    MAX(order_date) AS max_order_date
  FROM recent_orders
  GROUP BY user_id
),
user_segment AS (
  SELECT
    u.id AS user_id,
    u.region,
    u.name,
    s.order_count,
    s.total_amount,
    s.avg_amount,
    s.max_order_date,
    CASE
      WHEN s.total_amount >= 10000 THEN 'platinum'
      WHEN s.total_amount >= 5000 THEN 'gold'
      WHEN s.order_count >= 20 THEN 'silver'
      ELSE 'bronze'
    END AS customer_segment
  FROM user_order_stats s
  JOIN users u ON u.id = s.user_id
),
ranked_segment AS (
  SELECT
    us.*,
    RANK() OVER (PARTITION BY us.region ORDER BY us.total_amount DESC) AS region_rank
  FROM user_segment us
)

INSERT OVERWRITE TABLE result_user_order_summary
SELECT
  rs.user_id,
  rs.region,
  rs.name,
  rs.order_count,
  rs.total_amount,
  rs.avg_amount,
  rs.max_order_date,
  rs.customer_segment,
  rs.region_rank
FROM ranked_segment rs
WHERE rs.order_count >= 3
  AND rs.region IS NOT NULL
ORDER BY rs.region, rs.total_amount DESC;
```

## 适用场景

- 快速探索本地/远程 CSV、JSON 数据
- 将文件目录映射成可查询的 SQL 表
- 执行跨表聚合、分析和导出
- 以 Go 语言实现轻量级 Hive 风格查询引擎

- 将查询结果导出为标准文件格式用于报表和数据交换

## 项目定位

`gsql` 适用于轻量级的数据查询场景，尤其是本地数据仓库、数据分析辅助、运维数据检索等场景。它不是一个完整的关系型数据库，而是一个面向文件数据的 SQL 查询引擎。
