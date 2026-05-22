# gsql 开发计划 TODO

## 当前已实现
- [x] `CREATE TABLE ... WITH (...)` 元数据声明
- [x] `SELECT ... FROM ...` 查询
- [x] `WITH` 公共表表达式（CTE）支持
- [x] 简单 `WHERE` 比较过滤（支持 LIKE）
- [x] `ORDER BY`
- [x] `LIMIT`
- [x] `INSERT OVERWRITE TABLE ... SELECT ...`
- [x] 本地 `CSV` / `JSON` 读写
- [x] 执行计划层：`TableScan -> Filter -> Sort -> Limit -> Project`
- [x] 并行化执行：多文件读取、过滤和投影阶段并行处理
- [x] 支持 `SELECT ... FROM ... WHERE ... ORDER BY ... LIMIT ...` 更稳健解析
- [x] `JSON` 写入支持
- [x] 复杂 `WHERE` 表达式（AND/OR 组合逻辑）
- [x] `GROUP BY` 与聚合函数（COUNT / SUM / AVG / MIN / MAX）
- [x] `HAVING` 子句（聚合后过滤）
- [x] `JOIN` 语法（Hash Join 实现，自动选择小表建哈希）
- [x] 子查询与嵌套查询（FROM 子句中的派生表）
- [x] 表别名支持（`FROM users u`、`JOIN orders o ON u.id = o.user_id`）
- [x] 列别名支持（`COUNT(*) AS cnt`，ORDER BY 可引用别名）
- [x] `IS NULL` / `IS NOT NULL` 空值判断
- [x] `IN` / `NOT IN` 列表匹配
- [x] `DISTINCT` 去重查询
- [x] 算术表达式（`+`, `-`, `*`, `/` 四则运算）
- [x] `AND`/`OR` 组合 WHERE/HAVING 表达式修复
- [x] 集成测试：覆盖基本查询、JOIN、CTE、子查询、INSERT 全流程
- [x] 边缘情况测试：空 CSV、空字段、引号字段、缺失列
- [x] 逻辑计划与物理计划分离：`LogicalNode` + `LogicalToPhysical()` 转换器 + 优化器框架

## 短期开发目标
- [x] 加入逻辑计划与物理计划分离
  - 逻辑计划：`pkg/plan/logical.go` — 纯数据描述，不含执行逻辑
  - 物理计划：`pkg/plan/plan.go` — 实现 `Execute()` 执行接口
  - 转换器：`LogicalToPhysical()` 将逻辑计划转为物理计划
  - 优化器框架：`pkg/plan/optimizer.go` — 规则式优化框架，可扩展谓词下推等
- [x] 优化器规则：`MergeFilters`（合并相邻 Filter）
- [x] 列裁剪：`ColumnPruning`（在 Scan 上自动插入 Project 裁剪无用列）
- [x] 支持基于成本的简单计划选择（`EstimateRows` + `CostBasedJoinReorder` 规则，利用 `Table.EstimatedRows` 统计信息自动将小表放在哈希连接的构建侧）
- [x] 数据源扩展
  - [x] 增加 `S3` 访问适配器（支持 AWS S3 和 S3兼容服务）
  - [x] 增加 `FTP` / `SFTP` / `WebDAV` 数据源适配
  - [x] 增加 `Git LFS` 数据访问支持
- [x] 存储与写入增强
  - [x] 支持目标表分区目录写入
  - [x] 支持追加写入与覆盖策略（`INSERT OVERWRITE` 和 `INSERT INTO`）
- [x] 引擎功能补齐
  - [x] 支持窗口函数与 `OVER` 计算（ROW_NUMBER、RANK、DENSE_RANK 及聚合窗口函数）
  - [x] 支持 `CREATE EXTERNAL TABLE` 语义
  - [x] `UNION` 和 `UNION ALL` 支持
  - [x] `CASE WHEN` 表达式支持
- [x] 测试阶段：使用 samples 数据执行 gsql 命令，并校验输出结果

## 长期目标
- [ ] 进一步靠近 Hive 风格执行架构
  - [ ] 支持分区裁剪与分区推理
  - [ ] 支持更完整的 SQL 标准特性
  - [ ] 引入更完善的优化器与执行计划 rewrite
  - [ ] 考虑分布式执行或更大规模并行模型
