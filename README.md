# gsql

gsql 是一个基于 Go 的轻量级 Hive 风格 SQL 查询引擎原型。当前版本聚焦于本地文件数据源、执行计划生成和并行行级执行。

## 核心功能

- `CREATE TABLE ... WITH (...)` 声明本地表结构与文件映射
- `SELECT` 查询、`WITH` CTE、`WHERE`、`ORDER BY`、`LIMIT`
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
3. 存储层
   - 本地 CSV/JSON 文件读取与写入
   - 支持通过 `WITH` 选项指定 `storage`, `format`, `location`, `file_pattern`, `file_name`
4. 执行计划层
   - 将查询构建为计划算子：`TableScan -> Filter -> Sort -> Limit -> Project`
   - 运行时对行处理阶段进行多核并行化
5. 导出层
   - 支持将查询结果导出为 CSV/JSON 文件

## 当前支持

- `CREATE TABLE ... WITH (...)`
- `SELECT ... FROM ...`
- `WITH` 公共表表达式（CTE）
- `WHERE` 简单等值过滤
- `ORDER BY`
- `LIMIT`
- `INSERT OVERWRITE TABLE ... SELECT ...`
- 本地 `CSV` / `JSON` 数据读取与写入

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
WHERE email LIKE '%@example.com';
```

## 并行执行与计划生成

当前引擎已将 `SELECT` 查询转换为执行计划节点：

- `TableScan`：读取本地表文件
- `Filter`：并行过滤行
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
- 添加 `JOIN`、`GROUP BY`、聚合函数和窗口函数支持
- 引入更完善的 SQL 解析器与优化器
- 进一步扩展执行计划算子与分区裁剪逻辑
