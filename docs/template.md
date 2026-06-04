# Template SQL (Jinja2)

gsql 使用 [pongo2](https://github.com/flosch/pongo2)（Go 实现的 Jinja2 引擎）渲染 SQL 模板。通过 `-t` 指定模板文件，`-a` 传入参数。

## 基本用法

```bash
gsql -t query.sql -a "city=Beijing" -a "min_age=18"
```

模板文件 `query.sql`：
```sql
SELECT id, name, age
FROM users
WHERE city = '{{ city }}' AND age >= {{ min_age }}
ORDER BY id;
```

渲染后：
```sql
SELECT id, name, age
FROM users
WHERE city = 'Beijing' AND age >= 18
ORDER BY id;
```

## 参数类型

### 字符串

默认值为字符串类型，模板中按需加引号：

```bash
gsql -t q.sql -a "name=Alice"
```

```sql
SELECT * FROM users WHERE name = '{{ name }}';
```

### 数值

无需引号：

```bash
gsql -t q.sql -a "limit=10"
```

```sql
SELECT * FROM users LIMIT {{ limit }};
```

### JSON 数组 / 对象

值以 `[` 或 `{` 开头时自动解析为结构化数据：

```bash
gsql -t q.sql -a "ids=[1,2,3]"
```

```sql
SELECT * FROM users WHERE id IN ({{ ids|join:"," }});
```

### JSON 文件引用

通过 `@` 前缀引用外部 JSON 文件：

```bash
gsql -t q.sql -a "rows=@data.json"
```

文件 `data.json`：
```json
[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]
```

模板 `q.sql`：
```sql
{% for row in rows %}
INSERT INTO users VALUES ({{ row.id }}, '{{ row.name }}');
{% endfor %}
```

## 重复参数自动合并

同一个 key 多次 `-a` 自动合并为逗号分隔字符串：

```bash
gsql -t q.sql -a "month=2026-05" -a "user=alice" -a "user=bob"
```

```sql
-- 必须加 |safe 防止 ' 被转义为 &#39;
SELECT * FROM logs WHERE month = '{{ month }}'
  AND username IN ('{{ user|safe }}')
```

渲染后：
```sql
SELECT * FROM logs WHERE month = '2026-05'
  AND username IN ('alice','bob')
```

## 常用过滤器

| 过滤器 | 说明 | 示例 |
|--------|------|------|
| `default(val)` | 变量为空时使用默认值 | `{{ order_by\|default('id') }}` |
| `safe` | 关闭 HTML 转义 | `{{ user\|safe }}` |
| `join(sep)` | 数组用分隔符连接 | `{{ ids\|join:"," }}` |
| `upper` / `lower` | 大小写转换 | `{{ name\|upper }}` |
| `length` | 获取长度 | `{{ items\|length }}` |

## 条件渲染

```sql
SELECT * FROM users
{% if min_age %}
WHERE age >= {{ min_age }}
{% endif %}
ORDER BY id;
```

```bash
# 有 min_age 时带 WHERE，否则全表
gsql -t q.sql -a "min_age=18"
```

## 循环渲染

搭配 JSON 数组参数实现批量操作：

```bash
gsql -t q.sql -a "users=[{\"name\":\"Alice\",\"age\":25},{\"name\":\"Bob\",\"age\":30}]"
```

```sql
CREATE TABLE users (name STRING, age INT)
WITH (storage='local', format='csv', path='output');

{% for u in users %}
INSERT INTO users VALUES ('{{ u.name }}', {{ u.age }});
{% endfor %}

SELECT * FROM users;
```

## 实际场景示例

### 动态 IN 查询

```bash
gsql -t q.sql -a "status=active" -a "dept=eng" -a "dept=product"
```

```sql
SELECT name, email
FROM employees
WHERE status = '{{ status }}'
  AND department IN ('{{ dept|safe }}')
ORDER BY name;
```

### 时间范围查询

```bash
gsql -t q.sql -a "start=2026-01-01" -a "end=2026-06-30"
```

```sql
SELECT date, amount
FROM sales
WHERE date >= '{{ start }}' AND date < '{{ end }}'
ORDER BY date;
```

### 动态 ORDER BY

```bash
gsql -t q.sql -a "sort=amount" -a "order=DESC"
```

```sql
SELECT * FROM transactions
ORDER BY {{ sort }} {{ order }}
LIMIT 100;
```

### WITH 子句模板

```bash
gsql -t q.sql -a "month=2026-05" -a "dept=sales"
```

```sql
WITH filtered AS (
  SELECT * FROM orders
  WHERE to_char(created_at, 'YYYY-MM') = '{{ month }}'
    AND department = '{{ dept }}'
)
SELECT department, COUNT(*) AS cnt, SUM(amount) AS total
FROM filtered
GROUP BY department;
```

### 动态表名

```bash
gsql -t q.sql -a "table=orders_202605"
```

```sql
SELECT * FROM {{ table }} WHERE status = 'pending';
```

## 模板来源

`-t` 支持所有存储后端，与 `-s` 相同：

```bash
# 本地文件
gsql -t local://path/to/query.sql -a "x=1"

# 对象存储
gsql -t s3://bucket/template.sql -a "env=prod"

# 飞书文档
gsql -t lark://app_token/folder/template.sql -a "env=prod"

# HTTP
gsql -t https://example.com/tpl.sql -a "mode=test"

# FTP
gsql -t ftp://user:pass@host/path/tpl.sql -a "key=val"
gsql -t sftp://user@host/path/tpl.sql -a "key=val"

# WebDAV / Git LFS
gsql -t webdav://host/path/tpl.sql -a "x=1"
gsql -t gitlfs://host/repo/tpl.sql -a "x=1"
```

## 调试

加 `-v` 查看渲染后的 SQL：

```bash
gsql -v -t query.sql -a "city=Beijing"
--- rendered SQL ---
SELECT * FROM users WHERE city = 'Beijing';
---
```
