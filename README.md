# gsql

`gsql` 是一个基于 `Go` 实现的本地数据查询服务，面向快速查询本地和远程文件数据源，支持结构化查询、建表、聚合、过滤、导出等功能。

## 核心特性

- 基于 `Golang` 实现的快速数据查询服务
- 支持基于创建语句的建表操作，类似 `hsql` 的建表体验
- 支持 `CSV`、`JSON` 等常见文件格式
- 支持多种文件系统访问方式：
  - 本地文件系统
  - `S3`
  - `FTP`
  - `SFTP`
  - `WebDAV`
  - `Git LFS`
- 支持多表关联、聚合、过滤、排序等 SQL 查询能力
- 支持结果导出为 `CSV`、`JSON` 等格式

## 功能说明

### 1. 建表与数据源映射

通过类似 SQL 的创建语句，将指定目录下的数据文件映射成表结构，支持对文件路径和格式进行声明。数据源相关属性（如 `format`、`location`、`storage` 等）应在 `CREATE TABLE ... WITH (...)` 语法内进行设定。这样可以像查询关系型数据库一样查询本地或远程文件数据。

### 2. 支持多种格式数据

- `CSV` 文件
- `JSON` 文件

### 3. 支持多文件系统

- `local` 本地文件
- `s3` 对象存储
- `ftp` 服务器
- `sftp` 服务器
- `webdav` 服务器
- `git lfs`

数据源可通过统一的路径协议方式进行访问；同时支持将同目录或不同数据源文件映射为多张表。

### 4. 多表聚合与过滤

支持标准 SQL 语法的查询操作，包括 Hive 风格的查询能力：

- `WITH` 公共表表达式（CTE）
- `SELECT` 投影、表达式、聚合
- `FROM ... JOIN ...` 多表关联（包括内连接、外连接）
- `WHERE` 过滤条件与布尔逻辑
- `GROUP BY` 聚合分组
- `ORDER BY` 排序
- `LIMIT` 结果限制
- `PARTITION` 分区裁剪与分区查询语义
- `OVER` 子句与窗口函数支持（如 `RANK() OVER (...)`、`ROW_NUMBER() OVER (...)`）
- 常见聚合函数（如 `COUNT()`、`SUM()`）

可实现跨表聚合、分区处理、预查询、筛选和汇总。

> 与 Hive SQL 的差异：
> - 采用更简化的 `CREATE TABLE ... WITH (...)` 语法，直接映射目录/文件数据源
> - 只支持 `CSV`、`JSON` 文件格式，不同于 Hive 的 ORC/PARQUET/AVRO 等列式存储
> - 面向本地和远程文件访问，不依赖 Hive Metastore
> - 重点是快速数据探索与导出，而不是完整的分布式数据仓库计算

### 5. 数据导出能力

查询结果支持导出为：

- `CSV`
- `JSON`

导出功能适用于查询结果持久化、分析报表或下游系统消费。

## 示例

```sql
-- 创建 users 表，示例 local 存储 + CSV 格式 + file_pattern
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

-- 创建 orders 表，示例 S3 存储 + JSON 格式
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

-- 创建 logs 表，示例 SFTP 存储 + CSV 格式
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

-- 创建 docs 表，示例 WebDAV 存储 + CSV 格式
CREATE TABLE docs (
  id STRING,
  title STRING,
  updated_at TIMESTAMP
)
WITH (
  storage = 'webdav',
  format = 'csv',
  location = 'https://webdav.example.com/data/docs',
  auth_token = '***'
);

-- 使用 CTE recent_orders 过滤完成订单，演示 WITH 子句与 WHERE 过滤
WITH recent_orders AS (
  SELECT *
  FROM orders
  WHERE order_date >= '2026-01-01'
    AND status = 'completed'
),
-- 使用 user_order_summary 聚合订单数据，演示 GROUP BY 和聚合函数
user_order_summary AS (
  SELECT
    o.user_id,
    COUNT(o.order_id) AS order_count,
    SUM(o.amount) AS total_amount,
    MAX(o.order_date) AS last_order_date
  FROM recent_orders o
  GROUP BY o.user_id
),
-- 使用 user_activity 关联用户信息并生成客户分层，演示 JOIN 和 CASE 表达式
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
-- 最终查询：演示 LEFT JOIN、窗口函数、聚合、排序与 LIMIT
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

## 目标场景

- 快速对本地 CSV/JSON 数据进行探索与分析
- 将目录级别的数据文件映射为 SQL 表执行查询
- 通过 SQL 实现跨表聚合和复杂过滤
- 将查询结果导出为标准文件格式用于报表和数据交换

## 项目定位

`gsql` 适用于轻量级的数据查询场景，尤其是本地数据仓库、数据分析辅助、运维数据检索等场景。它不是一个完整的关系型数据库，而是一个面向文件数据的 SQL 查询引擎。
