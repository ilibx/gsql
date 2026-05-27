# Database Support

gsql 支持将 **MySQL**、**PostgreSQL**、**SQLite** 作为数据源和数据目标，读写时自动建表。

> `pkg/database` 包提供了统一的 `Database` 接口，方便扩展新的数据库类型。

---

## 配置参数

### 通用参数

| 参数 | 说明 |
|------|------|
| `storage` | 数据库类型：`mysql`、`postgres`、`sqlite` |
| `table_name` | 目标数据库中的表名（默认与 gsql 表名相同） |
| `query` | 可选，自定义查询语句作为数据源 |

### 连接方式

支持两种配置方式：

**URL 方式（推荐）**：
```sql
-- MySQL
CREATE TABLE t (...) WITH (
  storage = 'mysql',
  url = 'user:password@tcp(host:3306)/dbname?charset=utf8mb4&parseTime=true'
);

-- PostgreSQL
CREATE TABLE t (...) WITH (
  storage = 'postgres',
  url = 'host=host port=5432 user=postgres password=secret dbname=mydb sslmode=disable'
);

-- SQLite
CREATE TABLE t (...) WITH (
  storage = 'sqlite',
  url = '/path/to/db.sqlite'
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

## SQLite

```sql
CREATE TABLE users (
  id INT,
  name STRING,
  age INT
)
WITH (
  storage = 'sqlite',
  location = '/tmp/mydb.db'
);

-- 写入数据（自动建表）
INSERT OVERWRITE TABLE users
SELECT 1 AS id, 'Alice' AS name, 28 AS age
UNION ALL
SELECT 2, 'Bob', 35;

-- 读取
SELECT * FROM users;

-- 追加
INSERT INTO TABLE users
SELECT 3 AS id, 'Charlie' AS name, 42 AS age;

-- 自定义查询
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

---

## MySQL

```sql
CREATE TABLE remote_users (
  id INT,
  name STRING,
  email STRING
)
WITH (
  storage = 'mysql',
  host = '127.0.0.1',
  port = '3306',
  username = 'root',
  password = 'secret',
  database = 'mydb',
  table_name = 'users'        -- 对应 MySQL 中的 users 表
);

-- 读取远程表
SELECT * FROM remote_users WHERE age > 30;

-- 写入（自动建表，覆盖写入）
INSERT OVERWRITE TABLE remote_users
SELECT id, name, email FROM local_users;

-- 追加
INSERT INTO TABLE remote_users
SELECT 201, 'NewUser', 'new@example.com';
```

---

## PostgreSQL

```sql
CREATE TABLE pg_data (
  id INT,
  value STRING
)
WITH (
  storage = 'postgres',
  host = '127.0.0.1',
  port = '5432',
  username = 'postgres',
  password = 'secret',
  database = 'mydb'
);

INSERT OVERWRITE TABLE pg_data
SELECT 1 AS id, 'hello' AS value;

SELECT * FROM pg_data;
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
  url = 'root:secret@tcp(127.0.0.1:3306)/shop',
  query = '''
    SELECT DATE_FORMAT(order_date, '%Y-%m') AS month,
           COUNT(*) AS order_count,
           SUM(amount) AS total_amount
    FROM orders
    WHERE order_date >= '2024-01-01'
    GROUP BY DATE_FORMAT(order_date, '%Y-%m')
  '''
);

SELECT * FROM monthly_summary WHERE order_count > 10 ORDER BY month;

-- 跨表 JOIN
CREATE TABLE user_orders (
  user_id INT,
  user_name STRING,
  total_spent FLOAT
)
WITH (
  storage = 'mysql',
  url = 'root:secret@tcp(127.0.0.1:3306)/shop',
  query = '''
    SELECT u.id AS user_id, u.name AS user_name,
           SUM(o.amount) AS total_spent
    FROM users u
    JOIN orders o ON u.id = o.user_id
    GROUP BY u.id, u.name
  '''
);

SELECT * FROM user_orders WHERE total_spent > 100;
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

`pkg/database` 提供了统一的 `Database` 接口，添加新数据库只需：

1. 在 `pkg/database/database.go` 中导入新驱动
2. 在 `driverFor()` 中添加驱动名称映射
3. 在 `dataSourceName()` 中添加 DSN 构造
4. 在 `IsDatabase()` 中添加类型名

```go
// 示例：添加 Oracle 支持
func driverFor(storageType string) string {
    switch storageType {
    case "oracle":
        return "oracle"  // 对应 driver 注册名
    }
}

func dataSourceName(tbl *catalog.Table) string {
    switch storageType {
    case "oracle":
        return fmt.Sprintf("oracle://%s:%s@%s:%s/%s", ...)
    }
}
```
