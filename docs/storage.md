# Storage Backends

gsql 支持多种存储后端，可通过传统方式或 URL 方式配置。

> 数据库类型（MySQL / PostgreSQL / SQLite）的用法见 [database.md](database.md)。

---

## 配置方式

### 传统方式

在 `WITH` 子句中直接指定参数：

```sql
CREATE TABLE t (...)
WITH (
  storage = 's3',
  bucket = 'my-bucket',
  region = 'us-east-1',
  format = 'csv'
);
```

### URL 方式（推荐）

通过统一 URL 自动推断存储类型：

```sql
CREATE TABLE t (...)
WITH (
  url = 's3://s3.example.com/my-bucket/data/?region=us-east-1&access_key=xxx&access_secret=yyy',
  format = 'csv'
);
```

### 混合方式

带前缀的配置会覆盖 URL 中对应的参数：

```sql
CREATE TABLE t (...)
WITH (
  url = 'ftp://ftp.example.com/data',
  username = 'custom_user',   -- 覆盖 URL 中的用户名
  password = 'custom_pass',   -- 覆盖 URL 中的密码
  format = 'csv'
);
```

---

## Local（本地文件系统）

```sql
CREATE TABLE users (
  id INT,
  name STRING
)
WITH (
  storage = 'local',
  format = 'csv',
  path = '/tmp/users',
  file_pattern = '*.csv'
);
```

| 参数 | 说明 |
|------|------|
| `path` | 数据目录路径 |
| `file_pattern` | 读取时文件匹配模式（如 `*.csv`） |
| `file_name` | 写入时的输出文件名 |
| `partition_format` | 分区目录格式：`col=value`（默认）或 `value` |

---

## S3 / S3 兼容服务

支持 AWS S3、MinIO、阿里 OSS 等。

### URL 方式

```sql
CREATE TABLE s3_data (
  id INT,
  name STRING
)
WITH (
  url = 's3://s3.example.com/my-bucket/data/?region=us-east-1&access_key=xxx&access_secret=yyy',
  format = 'csv',
  file_pattern = '*.csv'
);
```

URL 格式：`s3://endpoint/bucket/prefix?region=...&access_key=...&access_secret=...`

- host 部分为 endpoint（如 `s3.us-east-1.amazonaws.com`、`play.min.io`）
- 第一个路径段为 bucket，后续为 prefix

### 传统方式

```sql
CREATE TABLE s3_data (
  id INT,
  name STRING
)
WITH (
  storage = 's3',
  bucket = 'my-bucket',
  region = 'us-east-1',
  format = 'csv',
  file_pattern = '*.csv',
  use_path_style = 'true',    -- MinIO 等需要
  retry_mode = 'adaptive'     -- standard / adaptive
);
```

### 配置参数

| 参数 | 说明 |
|------|------|
| `bucket` | S3 存储桶名 |
| `region` | 区域（如 `us-east-1`） |
| `path` | 路径前缀 |
| `endpoint` | 自定义 endpoint（MinIO 等） |
| `access_key` | 访问密钥 ID |
| `access_secret` | 访问密钥 Secret |
| `use_path_style` | 路径式寻址（MinIO 等需要 `true`） |
| `retry_mode` | 重试模式：`standard` 或 `adaptive` |

---

## FTP / SFTP

```sql
CREATE TABLE ftp_data (
  id INT,
  name STRING
)
WITH (
  storage = 'ftp',
  host = 'ftp.example.com',
  port = '21',
  username = 'user',
  password = 'pass',
  path = '/data',
  format = 'csv',
  file_pattern = '*.csv'
);

-- SFTP
CREATE TABLE sftp_data (
  id INT,
  name STRING
)
WITH (
  storage = 'sftp',
  host = 'sftp.example.com',
  port = '22',
  username = 'user',
  password = 'pass',
  path = '/data',
  format = 'csv',
  file_pattern = '*.csv'
);
```

---

## WebDAV

```sql
CREATE TABLE webdav_data (
  id INT,
  name STRING
)
WITH (
  storage = 'webdav',
  url = 'http://webdav.example.com',
  username = 'user',
  password = 'pass',
  path = '/public/data',
  format = 'csv',
  file_pattern = '*.csv'
);
```

---

## Git LFS

```sql
-- 指定 Git 仓库路径
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

-- 或直接指定 LFS 缓存路径
CREATE TABLE gitlfs_cache (
  id INT,
  name STRING
)
WITH (
  storage = 'gitlfs',
  path = '/lfs/objects',
  format = 'csv',
  file_pattern = '*.csv'
);
```

---

## Lark（飞书云文档）

支持读写飞书云文档，并可自动分享到群聊。

### 配置方式

```sql
CREATE TABLE lark_data (
  id INT,
  name STRING,
  score INT
)
WITH (
  storage = 'lark',
  app_id = 'cli_xxxxxxxxxxxx',
  app_secret = 'xxxxxxxxxxxxxxxxxxxxxxxxxxxx',
  path = 'gsql-data',            -- 在网盘根目录自动创建/查找
  chat_id = 'oc_xxxxxxxxxxxxx',    -- 可选：自动分享到群聊
  format = 'csv',
  file_pattern = '*.csv'
);
```

### URL 方式

```sql
CREATE TABLE lark_data (
  id INT,
  name STRING
)
WITH (
  url = 'lark://gsql-data/path?app_id=cli_xxx&app_secret=xxx&chat_id=oc_xxx',
  format = 'csv'
);
```

### 配置参数

| 参数 | 说明 |
|------|------|
| `app_id` / `lark_app_id` | 飞书应用的 App ID |
| `app_secret` / `lark_app_secret` | 飞书应用的 App Secret |
| `path` | 根文件夹名称（推荐，自动创建/查找） |
| `root_token` / `lark_root_token` | 根文件夹 token（与 `folder` 二选一） |
| `chat_id` / `lark_chat_id` | 群聊 ID（可选，自动授权） |
| `format` | 支持 `csv`、`json`、`excel` / `xlsx` |

### 自动授权

当 `chat_id` 配置后，写入文件和新创建的目录会自动授予群聊 `full_access`（完全管理）权限。

### 获取文件夹 token

在飞书云文档中打开目标文件夹，浏览器地址栏 URL 中的 `{token}` 部分即为文件夹 token。

---

## URL 协议参考

| 协议 | 格式 | 说明 |
|------|------|------|
| `local://` | `local:///data/path` | 本地文件系统 |
| `ftp://` | `ftp://user:pass@host:21/path` | FTP 服务器 |
| `sftp://` | `sftp://user:pass@host:22/path` | SFTP 服务器 |
| `s3://` | `s3://endpoint/bucket/prefix?params` | S3 兼容服务 |
| `webdav://` | `webdav://user:pass@host/path` | WebDAV |
| `git://` | `git:///path/to/repo` | Git 仓库 |
| `lark://` | `lark://folder/path?app_id=xxx&app_secret=xxx&chat_id=xxx` | 飞书 Lark |
| `mysql://` | 见 [database.md](database.md) | MySQL 数据库 |
| `postgres://` | 见 [database.md](database.md) | PostgreSQL 数据库 |
| `sqlite://` | 见 [database.md](database.md) | SQLite 数据库 |
## 参数优先级

1. 显式配置的参数（如 `username`、`password`）优先级最高
2. URL 中的参数次之
3. 默认值优先级最低（如 FTP 默认端口 21）
