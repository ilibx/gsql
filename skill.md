# gsql Skill

gsql 是一个基于 Go 的轻量级 Hive 风格 SQL 查询引擎，支持本地文件、云存储、远程数据库。

## 目录结构

```
├── cmd/gsql/          入口 main.go
├── pkg/
│   ├── catalog/       表定义与元数据管理
│   ├── database/      数据库适配器 (MySQL/PG/SQLite)，独立的 Database 接口
│   ├── engine/        引擎入口，SQL 执行调度
│   ├── parser/        SQL 解析器
│   ├── plan/          逻辑计划 / 物理计划 / 优化器 / 内置函数
│   ├── serde/         序列化 (CSV/JSON/Excel 读写)
│   └── storage/       Storage 接口 + 文件系统/S3/FTP/WebDAV/GitLFS/Lark 实现
├── tests/
│   ├── basic/         SQL 基础功能测试
│   ├── functions/     内置函数测试
│   ├── storage/       存储后端测试 (分区/外部表/INSERT/Lark)
│   └── format/        文件格式测试 (CSV 分隔符/跳表头)
├── docs/
│   ├── basic.md       基本用法
│   ├── functions.md   内置函数手册（每个函数有可执行示例）
│   ├── storage.md     存储后端
│   ├── database.md    数据库支持
│   └── format.md      文件格式
├── go.mod / go.sum
├── Makefile
└── README.md
```

## 构建与运行

```bash
make build          # go build -o bin/gsql ./cmd/gsql
make test           # go test ./...
make run-sql        # 执行 tests/check.sql

# 直接执行 SQL
./bin/gsql -e "SELECT 1 AS id"
./bin/gsql -s file.sql
./bin/gsql -s setup.sql -s query.sql    # 组合文件
```

## 测试

```bash
go test ./tests/...          # 全部集成测试
go test ./pkg/plan/...       # 计划/函数 单元测试
go test ./pkg/engine/...     # 引擎测试
go test ./pkg/parser/...     # 解析器测试
go test ./pkg/serde/...      # 序列化测试

# 运行指定测试
go test ./tests/... -run TestFuncMath -v
go test ./pkg/plan/... -run TestScalarAbs -v
```

## 代码约定

### 新增内置函数

1. 在 `pkg/plan/builtin_*.go` 中实现函数逻辑
2. 在对应的 `register*Builtins()` 中注册函数（函数名大写）
3. 在 `tests/functions/test_func_*.sql` 中添加测试 SQL
4. 在 `docs/functions.md` 中添加文档和可执行示例

### 新增存储后端

1. 在 `pkg/storage/` 下创建实现文件，实现 `Storage` 接口
2. 在 `pkg/storage/storage.go` 的 `GetStorage` 中添加分支
3. 在 `docs/storage.md` 中添加文档

### 新增数据库支持

1. 在 `pkg/database/database.go` 中添加驱动注册、`driverFor()`、`dataSourceName()`、`IsDatabase()`
2. 在 `docs/database.md` 中添加文档

### SQL 测试文件规范

- SQL 文件放在 `tests/` 下按分类目录存放
- 每条 SQL 语句应当可独立执行
- 使用 `setup.sql` 中的预置表（users/products/orders/partitioned_orders）
- 函数测试使用 `FROM users LIMIT 1` 确保单行结果

### 文档规范

- 文档放在 `docs/` 目录，使用 Markdown
- 每个函数的文档示例必须可直接复制执行
- 代码块使用 sql 标注

## 核心设计

### 执行流程

```
SQL → Parser → Logical Plan → Optimizer → Physical Plan → Execute
```

### 内置函数分类

| 分类 | 数量 | 文件 |
|------|------|------|
| 聚合 | 19 | `builtin_agg.go` |
| 窗口 | 11 | `builtin_window.go` |
| 数学 | 16 | `builtin_math.go` |
| 字符串 | 21 | `builtin_string.go` |
| 日期时间 | 25 | `builtin_datetime.go` |
| 条件 | 5 | `builtin_conditional.go` |
| 杂项 | 24 | `builtin_misc.go` |

函数注册在 `pkg/plan/builtin.go:init()` 中，所有函数名不区分大小写。

### 存储后端

`Storage` 接口定义在 `pkg/storage/storage.go`，实现文件按类型独立：

| 后端 | 文件 |
|------|------|
| Local | `local.go` |
| S3 | `s3.go` |
| FTP | `remote.go` |
| SFTP | `remote.go` |
| WebDAV | `remote.go` |
| Git LFS | `gitlfs.go` |
| Lark | `lark.go` |
| MySQL/PG/SQLite | `pkg/database/database.go` |

### 文件格式

`SerdeOptions` 定义在 `pkg/serde/serde.go`，支持 CSV/JSON/Excel 三种格式。

## 选项命名规则

WITH 选项名只能使用字母、数字和下划线，不能使用连字符 `-`。
