# File Formats

gsql 支持 CSV、JSON、Excel (.xlsx) 三种文件格式的读取和写入。

---

## CSV

CSV 为默认格式，当 `format` 未指定时默认使用 CSV。

### 读取

```sql
CREATE TABLE csv_data (
  id INT,
  name STRING,
  score INT
)
WITH (
  storage = 'local',
  format = 'csv',
  location = '/tmp/csv_data',
  file_pattern = '*.csv'
);
```

### 写入

```sql
CREATE TABLE csv_output (
  id INT,
  name STRING
)
WITH (
  storage = 'local',
  format = 'csv',
  location = '/tmp/output',
  file_name = 'result.csv'
);

INSERT OVERWRITE TABLE csv_output
SELECT 1 AS id, 'Alice' AS name;
```

### CSV 选项

```sql
CREATE TABLE csv_opts (
  id INT,
  name STRING
)
WITH (
  storage = 'local',
  format = 'csv',
  location = '/tmp/csv_opts',
  file_name = 'data.csv',
  delimiter = '|',           -- 字段分隔符，默认 ','
  skip_lines = '1',          -- 读取时跳过文件首 N 行（如表头）
  include_header = 'true',   -- 写入时输出列名作为首行
  quote_char = '"',          -- 引号字符
  escape_char = '"'          -- 转义字符
);

INSERT OVERWRITE TABLE csv_opts
SELECT 1 AS id, 'Alice' AS name;
-- 输出: id|name
--       1|Alice

INSERT INTO TABLE csv_opts
SELECT 2 AS id, 'Bob' AS name;
-- 追加后: id|name
--         1|Alice
--         2|Bob
```

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `delimiter` | `,` | 字段分隔符 |
| `skip_lines` | `0` | 读取时跳过的行数（如表头） |
| `include_header` | `false` | 写入时是否输出列名首行 |
| `quote_char` | `"` | 引号字符 |
| `escape_char` | `"` | 转义字符 |

---

## JSON

每行一个 JSON 对象（JSON Lines 格式）。

### 读取

```sql
CREATE TABLE json_data (
  id INT,
  name STRING,
  score INT
)
WITH (
  storage = 'local',
  format = 'json',
  location = '/tmp/json_data',
  file_pattern = '*.json'
);
```

输入文件格式：
```json
{"id": 1, "name": "Alice", "score": 95}
{"id": 2, "name": "Bob", "score": 87}
```

### 写入

```sql
CREATE TABLE json_output (
  id INT,
  name STRING
)
WITH (
  storage = 'local',
  format = 'json',
  location = '/tmp/output',
  file_name = 'result.json'
);

INSERT OVERWRITE TABLE json_output
SELECT 1 AS id, 'Alice' AS name;
-- 输出: {"id":"1","name":"Alice"}
```

---

## Excel (.xlsx)

### 读取

```sql
CREATE TABLE excel_data (
  id INT,
  name STRING,
  score INT
)
WITH (
  storage = 'local',
  format = 'excel',        -- 或 'xlsx'
  location = '/tmp/excel_data',
  file_pattern = '*.xlsx'
);
```

### 写入

```sql
CREATE TABLE excel_output (
  id INT,
  name STRING,
  score INT
)
WITH (
  storage = 'local',
  format = 'excel',            -- 或 'xlsx'
  location = '/tmp/excel_output',
  file_name = 'result.xlsx'
);

INSERT OVERWRITE TABLE excel_output
SELECT 1 AS id, 'Alice' AS name, 95 AS score
UNION ALL
SELECT 2, 'Bob', 87;
```

### 写入表头

设置 `include_header = 'true'` 可在 Excel 文件首行写入列名：

```sql
WITH (
  ...
  include_header = 'true'
)
```

### 指定工作表

通过 `sheet` 选项指定工作表名称（默认 "Sheet1"）：

```sql
WITH (
  ...
  sheet = 'Students'   -- 指定工作表名，读取时也生效
)
```

写入示例：
```sql
CREATE TABLE excel_sheet (
  id INT,
  name STRING,
  score INT
)
WITH (
  storage = 'local',
  format = 'xlsx',
  location = '/tmp/sheet_data',
  file_name = 'grades.xlsx',
  sheet = 'Students'
);

INSERT OVERWRITE TABLE excel_sheet
SELECT 1 AS id, 'Alice' AS name, 95 AS score;
-- 输出到 Students 工作表

-- 读取时指定相同 sheet 名
SELECT * FROM excel_sheet;
```

输出效果：
| id | name | score |
|----|------|-------|
| 1  | Alice | 95   |

---

## 格式选择总结

| 格式 | `format` 值 | 特点 |
|------|------------|------|
| CSV | `csv` 或不指定 | 默认格式，轻量，可读性好，支持丰富选项 |
| JSON | `json` | JSON Lines 格式，每行一个 JSON 对象 |
| Excel | `excel` 或 `xlsx` | 二进制格式，支持写入表头，适合报表导出 |
