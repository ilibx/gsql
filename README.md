# gsql

gsql 是一个基于 Go 的轻量级 Hive 风格 SQL 查询引擎原型，支持本地文件、云存储和远程数据源。

## 快速开始

```bash
# 构建
make build

# 执行 SQL
./bin/gsql -e "SELECT 1 AS id, 'hello' AS msg"
./bin/gsql -s query.sql

# 运行测试
make test
```

## 文档

| 文档 | 说明 |
|------|------|
| [基本用法](docs/basic.md) | 命令行、建表、查询、插入、分区表、EXPLAIN |
| [内置函数](docs/functions.md) | 100+ Hive 风格函数：聚合、窗口、数学、字符串、日期、条件、杂项，支持嵌套调用 |
| [存储后端](docs/storage.md) | Local、S3、FTP/SFTP、WebDAV、Git LFS、飞书 Lark |
| [数据库](docs/database.md) | MySQL、PostgreSQL、SQLite 作为数据源/目标 |
| [文件格式](docs/format.md) | CSV、JSON、Excel (.xlsx) 读写及选项 |

## 核心特性

- **SQL 语法**：`SELECT` / `WHERE` / `JOIN` / `GROUP BY` / `HAVING` / `ORDER BY` / `LIMIT` / `DISTINCT` / `UNION ALL` / `CTE (WITH)` / 子查询
- **表达式**：算术运算、比较、`IN` / `NOT IN`、`LIKE`、`IS NULL`、`CASE WHEN`
- **函数**：100+ Hive 兼容内置函数（聚合、窗口、数学、字符串、日期、条件、正则、JSON、哈希、脱敏等），支持嵌套函数调用
- **INSERT**：`INSERT OVERWRITE` 覆盖写入、`INSERT INTO` 追加
- **分区表**：`PARTITIONED BY` 定义，自动分区裁剪（等值+范围），支持 `col=value` 和裸值两种目录格式
- **文件格式**：CSV（支持自定义分隔符、引号、表头等）、JSON（每行一个对象）、Excel (.xlsx)
- **存储后端**：Local、S3 / MinIO / 阿里 OSS、FTP、SFTP、WebDAV、Git LFS、飞书 Lark、MySQL、PostgreSQL、SQLite
- **执行计划**：`EXPLAIN` 命令查看逻辑计划、优化后计划、物理执行计划
- **并行执行**：Filter 和 Project 阶段多核并行，本地文件读取并发加载

## 架构

```
SQL → Parser → Logical Plan → Optimizer → Physical Plan → Execute
```

- **解析层**：简化 SQL 解析器，支持 CREATE TABLE、SELECT、INSERT 及 WITH 子句
- **元数据层**：`pkg/catalog` 管理表定义与字段元数据
- **逻辑计划层**：`LogicalScan → Filter → Join → Aggregate → Sort → Limit → Project`
- **物理计划层**：物理算子实现 `Execute()` 接口，Filter/Project 并行执行，Hash Join 自动选择小表建哈希
- **存储层**：可插拔存储适配器，支持本地及多种远程存储

## Go 库使用

```go
import "github.com/ilibx/gsql"

db := gsql.Open()
defer db.Close()

db.Exec(`CREATE TABLE users (id INT, name STRING) WITH (
  storage = 'local', format = 'csv', location = '/tmp/users', file_pattern = '*.csv'
)`)
db.Exec(`INSERT OVERWRITE TABLE users SELECT 1 AS id, 'alice' AS name`)
rows, _ := db.Exec(`SELECT * FROM users`)
for _, row := range rows {
    fmt.Println(row["id"], row["name"])
}
```
