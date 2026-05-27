# Database Support

gsql 支持将 **MySQL**、**PostgreSQL**、**SQLite** 作为数据源和数据目标，读写时自动建表。

> `pkg/database` 包提供了统一的 `Database` 接口，方便扩展新的数据库类型。

---

## 完整示例

### SQLite

```sql
-- 建表
CREATE TABLE users (
  id INT,
  name STRING,
  age INT,
  email STRING,
  score INT
)
WITH (
  storage = 'sqlite',
  location = '/tmp/mydb.db'
); -- 默认表名与 gsql 表名一致（users），可不写 table_name

-- 写入数据（自动建表）
INSERT OVERWRITE TABLE users
SELECT 1 AS id, 'Alice' AS name, 28 AS age, 'alice@example.com' AS email, 95 AS score
UNION ALL
SELECT 2, 'Bob', 35, 'bob@example.com', 87
UNION ALL
SELECT 3, 'Charlie', 42, 'charlie@example.com', 72;

-- 查询
SELECT * FROM users WHERE age >= 30 ORDER BY age;

-- 追加数据
INSERT INTO TABLE users
SELECT 4 AS id, 'Diana' AS name, 31 AS age, 'diana@example.com' AS email, 88 AS score;

-- 自定义查询作为数据源
CREATE TABLE adult_users (
  id INT,
  name STRING
)
WITH (
  storage = 'sqlite',
  location = '/tmp/mydb.db',
  query = 'SELECT id, name FROM users WHERE age >= 30'
);

SELECT * FROM adult_users;
```

### MySQL

```sql
-- URL 方式（推荐，storage 自动推断）
CREATE TABLE users (
  id INT,
  name STRING,
  age INT,
  email STRING
)
WITH (
  url = 'mysql://root:secret@127.0.0.1:3306/mydb',
  table_name = 'users'     -- 对应 MySQL 中的真实表名
);

-- 读取远程表
SELECT * FROM users WHERE age > 30;

-- 写入（自动建表，覆盖写入）
INSERT OVERWRITE TABLE users
SELECT 1 AS id, 'Alice' AS name, 28 AS age, 'alice@example.com' AS email;

-- 追加
INSERT INTO TABLE users
SELECT 2 AS id, 'Bob' AS name, 35 AS age, 'bob@example.com' AS email;

-- 传统方式（显式配置连接参数）
CREATE TABLE products (
  id INT,
  name STRING,
  price INT
)
WITH (
  storage = 'mysql',
  host = '127.0.0.1',
  port = '3306',
  username = 'root',
  password = 'secret',
  database = 'mydb',
  table_name = 'products'
);

INSERT OVERWRITE TABLE products
SELECT 1 AS id, 'Laptop' AS name, 6999 AS price
UNION ALL
SELECT 2, 'Mouse', 99;

SELECT * FROM products WHERE price > 100;
```

### PostgreSQL

```sql
-- URL 方式
CREATE TABLE users (
  id INT,
  name STRING,
  age INT
)
WITH (
  url = 'postgres://postgres:secret@127.0.0.1:5432/mydb?sslmode=disable',
  table_name = 'users'      -- 对应 PostgreSQL 中的真实表名
);

INSERT OVERWRITE TABLE users
SELECT 1 AS id, 'Alice' AS name, 28 AS age;

SELECT * FROM users ORDER BY id;

-- 传统方式
CREATE TABLE logs (
  id INT,
  message STRING,
  created_at STRING
)
WITH (
  storage = 'postgres',
  host = '127.0.0.1',
  port = '5432',
  username = 'postgres',
  password = 'secret',
  database = 'mydb',
  table_name = 'logs',
  sslmode = 'disable'
);

INSERT OVERWRITE TABLE logs
SELECT 1 AS id, 'startup' AS message, '2026-01-15 10:00:00' AS created_at;

SELECT * FROM logs;
```

---

## 配置参数

### 通用参数

| 参数 | 说明 |
|------|------|
| `storage` | 数据库类型：`mysql`、`postgres`、`sqlite`（可省略，从 `url` 自动推断） |
| `table_name` | 目标数据库中的表名（默认与 gsql 表名相同） |
| `query` | 可选，自定义查询语句作为数据源 |

### 连接方式

支持两种配置方式：

**URL 方式（推荐）**—— `storage` 可省略，自动从 URL 推断：

```sql
-- MySQL
CREATE TABLE t (...) WITH (
  url = 'mysql://user:password@host:3306/dbname'
);

-- PostgreSQL
CREATE TABLE t (...) WITH (
  url = 'postgres://user:password@host:5432/dbname?sslmode=disable'
);

-- SQLite
CREATE TABLE t (...) WITH (
  url = 'sqlite:///path/to/db.sqlite'
  -- 或直接传路径
  -- url = '/path/to/db.sqlite'
);
```

**传统方式**：
```sql
-- MySQL
CREATE TABLE t (...) WITH (
  storage = 'mysql',
  host = '127.0.0.1',
  port = '3306',
  username = 'root',
  password = 'secret',
  database = 'mydb'
);

-- PostgreSQL
CREATE TABLE t (...) WITH (
  storage = 'postgres',
  host = '127.0.0.1',
  port = '5432',
  username = 'postgres',
  password = 'secret',
  database = 'mydb',
  sslmode = 'disable'
);

-- SQLite
CREATE TABLE t (...) WITH (
  storage = 'sqlite',
  location = '/tmp/mydb.db'
);
```

---

## 自定义查询作为数据源

通过 `query` 参数，可以将任意查询结果映射为 gsql 表，支持复杂 JOIN、聚合和子查询：

```sql
-- MySQL 复杂查询
CREATE TABLE monthly_summary (
  month STRING,
  order_count INT,
  total_amount FLOAT
)
WITH (
  storage = 'mysql',
  url = 'mysql://root:secret@127.0.0.1:3306/shop',
  query = 'SELECT DATE_FORMAT(order_date, '%%Y-%%m') AS month, COUNT(*) AS order_count, SUM(amount) AS total_amount FROM orders WHERE order_date >= "2024-01-01" GROUP BY DATE_FORMAT(order_date, '%%Y-%%m')'
);

SELECT * FROM monthly_summary WHERE order_count > 10 ORDER BY month;

-- PostgreSQL 查询
CREATE TABLE user_orders (
  user_id INT,
  user_name STRING,
  total_spent FLOAT
)
WITH (
  url = 'postgres://postgres:secret@127.0.0.1:5432/shop?sslmode=disable',
  query = 'SELECT u.id AS user_id, u.name AS user_name, SUM(o.amount) AS total_spent FROM users u JOIN orders o ON u.id = o.user_id GROUP BY u.id, u.name'
);

SELECT * FROM user_orders WHERE total_spent > 100;

-- SQLite 查询
CREATE TABLE top_scores (
  id INT,
  name STRING,
  score INT
)
WITH (
  storage = 'sqlite',
  location = '/tmp/db.sqlite',
  query = 'SELECT id, name, score FROM students WHERE score >= 80 ORDER BY score DESC'
);

SELECT * FROM top_scores LIMIT 10;
```

---

## 跨数据库读写

可以将不同数据库类型的表组合使用：

```sql
-- 从 MySQL 读取，写入 SQLite
INSERT OVERWRITE TABLE sqlite_backup
SELECT * FROM mysql_source;
```

---

## 扩展新的数据库

`pkg/database` 提供了统一的 `Database` 接口，添加新数据库只需在 `drivers.go` 中注册并新建独立的 `.go` 文件：

```go
// oracle.go
package database

import (
    "fmt"
    "net/url"
    "strings"
    _ "github.com/some/oracle/driver" // 导入驱动
)

func init() {
    for _, name := range []string{"oracle"} {
        RegisterDriver(name, driverInfo{
            driverName: "oracle",
            buildDSN:   buildOracleDSN,
            parseURL:   parseOracleURL,
        })
    }
}

func buildOracleDSN(tbl *catalog.Table) string {
    // 从 tbl.Option 构建 DSN
    return fmt.Sprintf("oracle://%s:%s@%s:%s/%s", ...)
}

func parseOracleURL(u *url.URL) string {
    // 从标准 URL 转 DSN
    return u.String()
}
```
