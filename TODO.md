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

## 近期开发目标
- [ ] 完善 SQL 解析器
  - [ ] 支持 `JOIN` 语法
  - [ ] 支持 `GROUP BY` 和聚合函数
  - [ ] 支持更复杂的 `WHERE` 表达式（当前支持单个比较谓词）
  - [x] 支持 `SELECT ... FROM ... WHERE ... ORDER BY ... LIMIT ...` 更稳健解析
- [ ] 执行计划优化
  - [ ] 加入逻辑计划与物理计划分离
  - [ ] 实现谓词下推与列裁剪
  - [ ] 增加 `Join` 执行算子与并行连接策略
  - [ ] 支持基于成本的简单计划选择
- [ ] 数据源扩展
  - [ ] 增加 `S3` 访问适配器
  - [ ] 增加 `FTP` / `SFTP` / `WebDAV` 数据源适配
  - [ ] 增加 `Git LFS` 数据访问支持
- [ ] 存储与写入增强
  - [ ] 支持目标表分区目录写入
  - [ ] 增加 `JSON` 写入支持
  - [ ] 支持追加写入与覆盖策略
- [ ] 引擎功能补齐
  - [ ] 支持窗口函数与 `OVER` 计算
  - [ ] 支持子查询与嵌套查询
  - [ ] 支持 `CREATE EXTERNAL TABLE` 语义
- [ ] 测试与文档
  - [ ] 补充单元测试覆盖核心算子与查询计划路径
  - [ ] 更新 `README.md` 与 `TODO.md`，保持文档与实现同步
  - [ ] 增加示例 SQL 与执行说明

## 长期目标
- [ ] 进一步靠近 Hive 风格执行架构
  - [ ] 支持分区裁剪与分区推理
  - [ ] 支持更完整的 SQL 标准特性
  - [ ] 引入更完善的优化器与执行计划 rewrite
  - [ ] 考虑分布式执行或更大规模并行模型
