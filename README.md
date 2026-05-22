# gsql

gsql 是一个基于 Go 的轻量级 Hive 风格 SQL 查询引擎原型。当前版本聚焦于本地文件数据源、执行计划生成和并行行级执行。

## 核心功能

- `CREATE TABLE ... WITH (...)` 声明本地表结构与文件映射
- `SELECT` 查询、`WITH` CTE、子查询（FROM 子句中的派生表）、`JOIN`（Hash Join 实现，支持表别名）、`WHERE`（支持 `AND`/`OR` 组合）、`GROUP BY`、`HAVING`、`ORDER BY`（支持列别名引用）、`LIMIT`
- `GROUP BY` 与聚合函数：`COUNT`、`SUM`、`AVG`、`MIN`、`MAX`
- `INSERT OVERWRITE TABLE` 将查询结果写回目标表
- 支持本地 `CSV` 与 `JSON` 数据源
- 查询引擎自动将 SQL 转换为执行计划，并对过滤与投影阶段进行并行执行
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
- 本地 `CSV` / `JSON` 数据读取与写入
- 表别名支持（`FROM users u`、`JOIN orders o`）
- 列别名支持（`COUNT(*) AS cnt`、`ORDER BY cnt`）

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

写入时自动按 `dt` 值分组写入子目录，如 `dt=2024-01-01/data.csv`。

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

## 并行执行与计划生成

当前引擎已将 `SELECT` 查询转换为执行计划节点：

- `TableScan`：读取本地表文件
- `Filter`：并行过滤行，支持单个比较谓词与 `AND`/`OR` 复合逻辑
- `Join`：Hash Join 实现，自动选择较短表构建哈希表
- `Aggregate`：分组聚合计算（`COUNT`/`SUM`/`AVG`/`MIN`/`MAX`）
- `Sort`：排序结果
- `Limit`：截取结果行数
- `Project`：并行投影所需列

同时，本地文件读取使用并发方式加载多个文件，提升数据扫描效率。

## 运行测试

```bash
go test ./...
```

## 未来扩展方向

- 支持更多数据源（S3 / FTP / SFTP / WebDAV / Git LFS）
- 添加更多 SQL 标准特性
- 引入更完善的 SQL 解析器与优化器
- 进一步扩展执行计划算子与分区裁剪逻辑
