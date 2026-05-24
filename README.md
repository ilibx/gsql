# gsql

gsql 是一个基于 Go 的轻量级 Hive 风格 SQL 查询引擎原型。当前版本聚焦于本地文件数据源、执行计划生成和并行行级执行。

## 核心功能

- `CREATE TABLE ... WITH (...)` 声明本地表结构与文件映射
- `SELECT` 查询、`WITH` CTE、子查询（FROM 子句中的派生表）、`JOIN`（Hash Join 实现，支持表别名）、`WHERE`（支持 `AND`/`OR` 组合）、`GROUP BY`、`HAVING`、`ORDER BY`（支持列别名引用）、`LIMIT`
- `GROUP BY` 与聚合函数：`COUNT`、`SUM`、`AVG`、`MIN`、`MAX`
- `INSERT OVERWRITE TABLE` 将查询结果写回目标表
- 支持本地 `CSV` 与 `JSON` 数据源
- 查询引擎自动将 SQL 转换为执行计划，并对过滤与投影阶段进行并行执行
- 支持 `EXPLAIN` 命令查看逻辑计划、优化后逻辑计划和物理执行计划
- 本地文件读取支持多文件并发加载

## 架构概览

gsql 拥有以下核心层次：

1. 解析层
   - 简化 SQL 解析器，支持 `CREATE TABLE`、`SELECT`、`INSERT OVERWRITE` 和 `WITH` 子句
2. 元数据层
   - 表定义与字段元数据由 `pkg/catalog` 管理
3. 逻辑计划层
   - 将 SQL 查询转换为逻辑计划算子：`LogicalScan → LogicalFilter → LogicalJoin → LogicalAggregate → LogicalSort → LogicalLimit → LogicalProject`
   - 逻辑计划不含执行逻辑，仅描述查询意图
   - 支持规则优化框架（`pkg/plan/optimizer.go`），可扩展谓词下推、列裁剪等优化
4. 物理计划层
   - 逻辑计划经优化后转换为物理算子：`TableScan → Filter → Join → Aggregate → Sort → Limit → Project`
   - 物理算子实现 `Execute()` 接口，运行时对行处理阶段进行多核并行化
   - Join 使用 Hash Join 实现，自动选择小表建哈希
5. 存储层
   - 本地 CSV/JSON 文件读取与写入
   - 支持通过 `WITH` 选项指定 `storage`, `format`, `location`, `file_pattern`, `file_name`
6. 导出层
   - 支持将查询结果导出为 CSV/JSON 文件

## 当前支持

- `CREATE TABLE ... WITH (...)`
- `SELECT ... FROM ...`（支持子查询 `FROM (SELECT ...) AS alias`）
- `WITH` 公共表表达式（CTE）
- `JOIN` 语法：`t1 JOIN t2 ON t1.col = t2.col`（Hash Join 实现，自动选择小表建哈希）
- `WHERE` 基本比较过滤：`=`, `!=`, `<`, `>`, `<=`, `>=`, `LIKE`
- `WHERE` / `HAVING` 复合逻辑：`AND` / `OR` 组合条件
- `IS NULL` / `IS NOT NULL` 空值判断
- `IN` / `NOT IN` 列表匹配
- `DISTINCT` 去重查询
- 算术表达式：`+`, `-`, `*`, `/`（SELECT 列表与 WHERE 条件均支持）
- `GROUP BY` 与聚合函数：`COUNT(*)`, `SUM(col)`, `AVG(col)`, `MIN(col)`, `MAX(col)`
- 窗口函数：`ROW_NUMBER()`, `RANK()`, `DENSE_RANK()`, `SUM() OVER(...)` 等，支持 `PARTITION BY` 与 `ORDER BY`
- `HAVING` 子句（支持聚合结果过滤）
- `ORDER BY`（支持多列排序）
- `LIMIT`
- `INSERT OVERWRITE TABLE ... SELECT ...`
- `PARTITIONED BY (col1, col2, ...)` 分区表定义，写入时自动按分区列值写入子目录
- 分区表读取时自动从目录路径注入分区列值，支持等值和范围（`>`, `<`, `>=`, `<=`）分区裁剪，自动识别 `col=value` 和裸值两种目录格式
- 本地 `CSV` / `JSON` 数据读取与写入
- **云存储与远程数据源支持**：
  - `S3` / S3兼容服务（MinIO、阿里OSS等）
  - `FTP` / `SFTP` 文件服务器
  - `WebDAV` 协议支持
  - `Git LFS` 版本控制大文件访问
- 表别名支持（`FROM users u`、`JOIN orders o`）
- 列别名支持（`COUNT(*) AS cnt`、`ORDER BY cnt`）
- `EXPLAIN` 命令查看逻辑计划、优化后逻辑计划与物理执行计划

## 使用示例

### 创建本地表

```sql
CREATE TABLE users (
  id INT,
  name STRING,
  email STRING
)
WITH (
  storage = 'local',
  format = 'csv',
  location = '/tmp/users',
  file_pattern = '*.csv'
);
```

### 查询示例

```sql
WITH recent_users AS (
  SELECT id, name, email
  FROM users
  WHERE name = 'alice'
)
SELECT id, name
FROM recent_users
ORDER BY id
LIMIT 10;
```

### 创建分区表

```sql
CREATE TABLE events (
  id INT,
  name STRING,
  dt STRING
)
WITH (
  storage = 'local',
  format = 'csv',
  location = '/tmp/events',
  file_name = 'data.csv'
)
PARTITIONED BY (dt);
```

写入时自动按 `dt` 值分组写入子目录，默认使用 `col=value` 格式（如 `dt=2024-01-01/data.csv`）。

可通过 `partition_format` 选项控制目录格式：
- `partition_format = 'col=value'`（默认）：使用 `col=value` 格式目录
- `partition_format = 'value'`：仅使用值作为目录名（如 `2024-01-01/data.csv`）

### Hive 风格分区（读取）

支持读取已按 Hive 风格目录结构组织的数据：

**`col=value` 格式目录**（自动识别）：
```
/data/events/
  year=2024/
    month=01/
      data.csv
    month=02/
      data.csv
```

**裸值格式目录**（自动识别）：
```
samples/orders/2026/
  01/
    data.csv
  02/
    data.csv
```

两种格式自动检测——如果第一级子目录不含 `col=` 前缀则视为裸值格式。

```sql
-- 默认格式：col=value 目录（如 year=2026/summary.csv）
CREATE TABLE monthly_summary (
  month STRING,
  order_count INT,
  total_amount INT
)
WITH (
  storage = 'local',
  format = 'csv',
  location = 'samples/result/monthly',
  file_name = 'summary.csv'
)
PARTITIONED BY (year);

-- 裸值格式：仅使用值作为目录（如 2026/summary.csv）
CREATE TABLE monthly_summary_bare (
  month STRING,
  order_count INT,
  total_amount INT
)
WITH (
  storage = 'local',
  format = 'csv',
  location = 'samples/result/monthly_bare',
  file_name = 'summary.csv',
  partition_format = 'value'
)
PARTITIONED BY (year);

-- 写入数据
INSERT OVERWRITE TABLE monthly_summary_bare
SELECT month, COUNT(*) AS order_count, SUM(amount) AS total_amount, year
FROM partitioned_orders
GROUP BY month, year;
```

查询时支持 **分区列自动注入**——`year` 和 `month` 值从目录路径提取并附加到每行数据：
```sql
-- 等值分区裁剪：只读取匹配的分区目录
SELECT * FROM partitioned_orders WHERE year = 2026 AND month = '03';

-- 范围分区裁剪：支持 >, <, >=, <=
SELECT * FROM partitioned_orders WHERE year >= 2026 AND month >= '05';

-- 分区列可参与聚合和排序
SELECT month, COUNT(*), SUM(amount)
FROM partitioned_orders
WHERE year = 2026
GROUP BY month
ORDER BY month;
```

### 插入覆盖结果表

```sql
CREATE TABLE result_users (
  id INT,
  name STRING
)
WITH (
  storage = 'local',
  format = 'csv',
  location = '/tmp/result',
  file_name = 'result.csv'
);

INSERT OVERWRITE TABLE result_users
SELECT id, name
FROM users
WHERE name != 'alice';
```

### S3 / S3兼容服务 (使用 AWS S3)

```sql
CREATE TABLE s3_data (
  id INT,
  name STRING
)
WITH (
  storage = 's3',
  s3_bucket = 'my-bucket',
  s3_region = 'us-east-1',
  s3_prefix = 'data/',
  format = 'csv',
  file_pattern = '*.csv'
);

SELECT * FROM s3_data LIMIT 10;
```

### FTP / SFTP 服务器

```sql
CREATE TABLE ftp_data (
  id INT,
  name STRING
)
WITH (
  storage = 'ftp',
  ftp_host = 'ftp.example.com',
  ftp_port = '21',
  ftp_user = 'username',
  ftp_pass = 'password',
  ftp_path = '/data',
  format = 'csv',
  file_pattern = '*.csv'
);

-- SFTP 类似，使用 storage = 'sftp' 和相应的 sftp_* 参数
```

### WebDAV 存储

```sql
CREATE TABLE webdav_data (
  id INT,
  name STRING
)
WITH (
  storage = 'webdav',
  webdav_url = 'http://webdav.example.com',
  webdav_user = 'username',
  webdav_pass = 'password',
  webdav_path = '/public/data',
  format = 'csv',
  file_pattern = '*.csv'
);
```

### Git LFS 版本控制

```sql
CREATE TABLE gitlfs_data (
  id INT,
  name STRING
)
WITH (
  storage = 'git-lfs',
  git_lfs_repo = '/path/to/git/repo',
  format = 'csv',
  file_pattern = '*.csv'
);

-- 或指定 LFS 缓存路径
CREATE TABLE gitlfs_cache (
  id INT,
  name STRING
)
WITH (
  storage = 'gitlfs',
  git_lfs_path = '/lfs/objects',
  format = 'csv',
  file_pattern = '*.csv'
);
```

## 并行执行与计划生成

当前引擎已将 `SELECT` 查询转换为执行计划节点：

- `TableScan`：读取本地表文件
- `Filter`：并行过滤行，支持单个比较谓词与 `AND`/`OR` 复合逻辑
- `Join`：Hash Join 实现，自动选择较短表构建哈希表
- `Aggregate`：分组聚合计算（`COUNT`/`SUM`/`AVG`/`MIN`/`MAX`）
- `Sort`：排序结果
- `Limit`：截取结果行数
- `Project`：并行投影所需列

使用者可以通过 `EXPLAIN` 命令查看查询的执行计划：

```sql
EXPLAIN SELECT name, age + 1 AS next_age, email
FROM users
WHERE age + 5 > 25
ORDER BY next_age
LIMIT 5;
```

输出包含三层计划：
- **Logical Plan** — 优化前的逻辑计划，反映 SQL 语义结构
- **Optimized Logical Plan** — 经过列裁剪等优化后的逻辑计划
- **Physical Plan** — 最终物理执行计划，展示实际执行的算子

```text
=== Logical Plan ===
LogicalLimit(5)
  LogicalSort((age + 1))
    LogicalProject(name, (age + 1), email)
      LogicalFilter(WHERE (age + 5) > 25)
        LogicalScan(users)

=== Optimized Logical Plan ===
LogicalLimit(5)
  LogicalSort((age + 1))
    LogicalProject(name, (age + 1), email)
      LogicalFilter(WHERE (age + 5) > 25)
        LogicalProject((age + 1), (age + 5), age, email, name)
          LogicalScan(users)

=== Physical Plan ===
Limit(5)
  Sort((age + 1))
    Project(name, (age + 1), email)
      Filter((age + 5) > 25)
        Project((age + 1), (age + 5), age, email, name)
          TableScan(table=users)
```

同时，本地文件读取使用并发方式加载多个文件，提升数据扫描效率。

## 运行测试

```bash
go test ./...
```

## Makefile 使用

```bash
make build
make run-sql SQL=samples/query_check.sql
make check-sql
make test
```

- `make build`：构建可执行文件 `bin/gsql`
- `make run-sql`：执行指定 SQL 文件
- `make check-sql`：执行 `samples/query_check.sql` 并校验输出是否与 `samples/query_check.expected` 一致
- `make test`：运行全部 Go 测试

## 存储适配器详情

gsql 支持多种存储后端，提供两种配置方式：

### 1. 传统方式（带前缀配置）

| 存储类型 | 说明 | 配置参数 |
|---------|------|---------|
| `local` | 本地文件系统（支持分区表目录结构自动检测） | `location`, `file_pattern`, `file_name`, `partition_format` |
| `s3` | AWS S3 或 S3兼容服务 | `s3_bucket`, `s3_region`, `s3_prefix`, `s3_endpoint`(可选) |
| `ftp` | FTP 服务器 | `ftp_host`, `ftp_port`, `ftp_user`, `ftp_pass`, `ftp_path` |
| `sftp` | SFTP 服务器 | `sftp_host`, `sftp_port`, `sftp_user`, `sftp_pass`, `sftp_path` |
| `webdav` | WebDAV 服务 | `webdav_url`, `webdav_user`, `webdav_pass`, `webdav_path` |
| `git-lfs` / `gitlfs` | Git LFS 版本控制 | `git_lfs_repo` 或 `git_lfs_path` |

### 2. URL 方式（推荐）

通过 `url` 参数统一配置存储信息，自动推断 `storage` 类型：

```sql
CREATE TABLE table_name (
  ...
)
WITH (
  url = 'scheme://user:pass@host:port/path?query_params',  -- 自动推断 storage 类型
  format = 'csv'
);
```

### URL 协议格式

通过统一的 URL 格式配置存储信息，自动推断 `storage` 类型和参数：
| 协议       | 格式示例                                                                 | 说明                                                                                     |
|------------|--------------------------------------------------------------------------|------------------------------------------------------------------------------------------|
| `local://` | `local:///data/path`                                                     | 本地文件系统，路径为绝对路径                                                             |
| `ftp://`   | `ftp://user:pass@ftp.example.com:21/data`                                | FTP 服务器，支持用户名密码和端口                                                         |
| `sftp://`  | `sftp://user:pass@sftp.example.com/data`                                 | SFTP 服务器，默认端口 22                                                                |
| `s3://`    | `s3://bucket/path?region=us-east-1&endpoint=s3.example.com`              | S3 或兼容服务（如 MinIO），支持 region 和 endpoint 参数                                |
| `hdfs://`  | `hdfs://namenode:8020/data`                                              | HDFS 文件系统，支持指定 namenode 和端口                                                 |
| `webdav://`| `webdav://user:pass@webdav.example.com/data`                             | WebDAV 服务，支持用户名密码                                                             |
| `git://`   | `git:///path/to/repo` 或 `git://user:pass@git.example.com/repo.git`      | Git 仓库（支持 Git LFS），可以是本地路径或远程仓库 URL

### 参数优先级

1. 显式配置的参数（如 `username`、`password`）优先级最高
2. URL 中的参数（如 `ftp://user:pass@host`）次之
3. 默认值（如 FTP 默认端口 21）优先级最低

### 3. 混合方式

可以同时使用 `url` 和带前缀的配置，带前缀的配置会覆盖 `url` 中的对应参数：

```sql
CREATE TABLE ftp_data (
  ...
)
WITH (
  url = 'ftp://ftp.example.com/data',  -- 自动推断 storage=ftp
  ftp_user = 'custom_user',            -- 覆盖 URL 中的用户名
  ftp_pass = 'custom_pass',            -- 覆盖 URL 中的密码
  format = 'csv'
);
```

## 未来扩展方向

- 支持更多 SQL 标准特性（UNION、INTERSECT、EXCEPT等）
- 添加更完善的 SQL 解析器与优化器
- 支持分布式执行或更大规模并行模型
