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
- [x] 支持分区裁剪与分区推理
- [ ] 支持更完整的 SQL 标准特性
  - [x] 数值类型 ORDER BY 转为数值比较
  - [x] `SELECT *` 展开为具体列
  - [x] `COUNT(DISTINCT col)` 支持
  - [x] `LIMIT 0` 语义正确（返回空集而非全量）
  - [x] SELECT 中支持常量表达式（`'2026' AS year`）
  - [x] 子查询支持更复杂表达式（IN/EXISTS 等）
  - [ ] `INSERT INTO ... VALUES` 支持
  - [ ] 列与列比较支持（`WHERE a = b`，当前仅支持 `WHERE a = literal`）
- [ ] 引入更完善的优化器与执行计划 rewrite
  - [ ] 谓词下推到扫描层已实现，可扩展更多规则
  - [ ] 列裁剪已实现
  - [ ] Join 重排序（基于代价）已实现
- [ ] 考虑分布式执行或更大规模并行模型

## 函数补齐计划（Hive SQL Built-in）

### P0：基础设施 — 统一函数注册系统
- [x] 新建 `pkg/plan/builtin.go`：函数注册表 + `FuncDef` 结构体定义
- [x] 拆分 `computeAggregate()` switch-case 到 `pkg/plan/builtin_agg.go`
- [x] 新建 `pkg/plan/builtin_math.go` — 数学函数注册
- [x] 新建 `pkg/plan/builtin_string.go` — 字符串函数注册
- [x] 新建 `pkg/plan/builtin_datetime.go` — 日期函数注册
- [x] 新建 `pkg/plan/builtin_window.go` — 窗口函数注册
- [x] `computeAggregate()` / `computePartitionWindow()` 改为从注册表查找
- [x] 新建 `pkg/plan/builtin_conditional.go` — 条件函数注册
- [x] 新建 `pkg/plan/builtin_test.go` — 函数注册单元测试

### P1：条件 & 类型转换
- [x] `IF(cond, true_val, false_val)` — 最常用三目运算
- [x] `COALESCE(v1, v2, ...)` — 返回第一个非 NULL
- [x] `NVL(val, default)` — Oracle 风格 COALESCE
- [x] `NULLIF(a, b)` — 相等则返回 NULL
- [x] `CAST(x AS type)` — 类型转换（parser + 执行，支持列引用和字面量）

### P1：标量函数调用支持（SELECT / WHERE）
- [x] 新增 `FuncCallExpr` AST 节点
- [x] parser `parseComparison()` 支持标量函数调用
- [x] engine 在 SELECT 中自动区分聚合 vs 标量函数
- [x] `evaluateExpressionValue()` 支持 `FuncCallExpr` 求值
- [x] `evaluateExpression()` 支持 `FuncCallExpr` 布尔求值
- [ ] WHERE 中多参函数调用（如 `SUBSTRING(col,1,2)`）完整支持

### P1：字符串函数
- [x] `CONCAT(str1, str2, ...)` — 字符串拼接
- [x] `CONCAT_WS(sep, str1, str2, ...)` — 带分隔符拼接
- [x] `SUBSTRING(s, pos[, len])` / `SUBSTR` — 子串截取
- [x] `UPPER(s)` / `UCASE` — 转大写
- [x] `LOWER(s)` / `LCASE` — 转小写
- [x] `TRIM(s)` — 去首尾空格
- [x] `LTRIM(s)` — 去左侧空格
- [x] `RTRIM(s)` — 去右侧空格
- [x] `LENGTH(s)` — 字符串长度
- [x] `REPLACE(s, from, to)` — 字符串替换
- [x] `REVERSE(s)` — 字符串反转
- [x] `LOCATE(sub, s[, pos])` / `INSTR` — 查找子串位置
- [x] `LPAD(s, len, pad)` — 左填充
- [x] `RPAD(s, len, pad)` — 右填充
- [x] `SPLIT(s, regex)` — 字符串分割（需配合 EXPLODE）
- [x] `INITCAP(s)` — 首字母大写
- [x] `ASCII(c)` — 返回 ASCII 码

### P1：数学函数
- [x] `ROUND(x[, d])` — 四舍五入
- [x] `FLOOR(x)` — 向下取整
- [x] `CEIL(x)` / `CEILING` — 向上取整
- [x] `ABS(x)` — 绝对值
- [x] `SQRT(x)` — 平方根
- [x] `EXP(x)` — e 的 x 次幂
- [x] `LN(x)` — 自然对数
- [x] `LOG10(x)` — 以 10 为底对数
- [x] `LOG2(x)` — 以 2 为底对数
- [x] `POWER(x, y)` / `POW` — 幂运算
- [x] `MOD(x, y)` — 取模
- [x] `SIGN(x)` — 返回符号
- [x] `RAND([seed])` — 随机数
- [x] `GREATEST(v1, v2, ...)` — 最大值
- [x] `LEAST(v1, v2, ...)` — 最小值
- [x] `WIDTH_BUCKET(x, min, max, num)` — 等宽分桶

### P1：日期时间函数
- [x] `CURRENT_DATE` — 当前日期
- [x] `CURRENT_TIMESTAMP` — 当前时间戳
- [x] `UNIX_TIMESTAMP([date])` — 时间戳转 unix epoch
- [x] `FROM_UNIXTIME(unixtime[, fmt])` — unix epoch 转日期串
- [x] `TO_DATE(ts)` — 提取日期部分
- [x] `YEAR(date)` — 提取年份
- [x] `MONTH(date)` — 提取月份
- [x] `DAY(date)` / `DAYOFMONTH` — 提取日
- [x] `HOUR(ts)` — 提取小时
- [x] `MINUTE(ts)` — 提取分钟
- [x] `SECOND(ts)` — 提取秒
- [x] `WEEKOFYEAR(date)` — 年内第几周
- [x] `DATEDIFF(end, start)` — 日期差（天）
- [x] `DATE_ADD(date, n)` — 日期加法
- [x] `DATE_SUB(date, n)` — 日期减法
- [x] `DATE_FORMAT(date, fmt)` — 日期格式化
- [x] `ADD_MONTHS(date, n)` — 月份加减
- [x] `LAST_DAY(date)` — 月末最后一天
- [x] `NEXT_DAY(date, day_of_week)` — 下个星期几
- [x] `MONTHS_BETWEEN(end, start)` — 月份差
- [x] `QUARTER(date)` — 提取季度
- [x] `TRUNC(date[, fmt])` — 日期截断
- [x] `FROM_UTC_TIMESTAMP(ts, tz)` — UTC 转本地
- [x] `TO_UTC_TIMESTAMP(ts, tz)` — 本地转 UTC

### P1：聚合函数（补充）
- [x] `STDDEV(expr)` / `STDDEV_POP` — 总体标准差
- [x] `STDDEV_SAMP` — 样本标准差
- [x] `VARIANCE(expr)` / `VAR_POP` — 总体方差
- [x] `VAR_SAMP` — 样本方差
- [x] `CORR(expr1, expr2)` — 相关系数
- [x] `COVAR_POP(expr1, expr2)` — 总体协方差
- [x] `COVAR_SAMP(expr1, expr2)` — 样本协方差
- [x] `PERCENTILE(col, p)` — 百分位数
- [x] `PERCENTILE_APPROX(col, p[, B])` — 近似百分位数
- [x] `COLLECT_LIST(col)` — 收集为列表
- [x] `COLLECT_SET(col)` — 收集为集合（去重）
- [x] `HISTOGRAM_NUMERIC(col, num)` — 数值直方图

### P1：窗口函数（补充）
- [x] `NTILE(n) OVER(...)` — 分桶编号
- [x] `LEAD(col[, offset, default]) OVER(...)` — 下一行
- [x] `LAG(col[, offset, default]) OVER(...)` — 上一行
- [x] `FIRST_VALUE(col) OVER(...)` — 窗口首行
- [x] `LAST_VALUE(col) OVER(...)` — 窗口末行
- [x] `CUME_DIST() OVER(...)` — 累积分布
- [x] `PERCENT_RANK() OVER(...)` — 百分比排名
- [x] `NTH_VALUE(col, n) OVER(...)` — 窗口内第 n 行

### P2：窗口帧支持
- [x] parser 识别 `ROWS BETWEEN ... AND ...` 语法
- [x] parser 识别 `RANGE BETWEEN ... AND ...` 语法
- [x] `WindowExpr` 增加 `Frame` 字段（含 `FrameType`、`Start`、`End`）
- [x] `computeWindowAggFrame()` 根据帧定义计算滑动窗口
- [x] 支持帧边界：`UNBOUNDED PRECEDING`、`n PRECEDING`、`CURRENT ROW`、`n FOLLOWING`、`UNBOUNDED FOLLOWING`
- [x] RANGE 帧基于 ORDER BY 值等值分组（当前支持数值类型 PRECEDING/FOLLOWING 阈值）

### P3：表生成函数 + LATERAL VIEW
- [x] `EXPLODE(arr)` — 数组展开为多行（逗号分隔字符串）
- [x] `EXPLODE(map)` — Map 展开为多行 (k, v)（k/v 成对读取）
- [x] `POSEXPLODE(arr)` — 带索引的展开
- [x] `LATERAL VIEW EXPLODE(...) tbl AS col` — 语法解析
- [x] `LATERAL VIEW OUTER EXPLODE(...)` — 空数组保留父行
- [x] SelectQuery 增加 LateralView 列表字段
- [x] engine/plan 展开 LATERAL VIEW 生成多行（`LateralViewExplodeNode`）

### P4：函数全量补全
- [x] 正则函数：`REGEXP_REPLACE`, `REGEXP_EXTRACT`, `REGEXP_LIKE`
- [x] 编码函数：`BASE64`, `UNBASE64`, `HEX`, `UNHEX`
- [x] 编码函数：`ENCODE`, `DECODE`
- [x] HASH 函数：`MD5`, `SHA1`, `SHA2`, `CRC32`, `HASH`
- [x] JSON 函数：`GET_JSON_OBJECT`
- [x] JSON 函数：`JSON_TUPLE`
- [x] 杂项：`CURRENT_USER`, `CURRENT_DATABASE`, `VERSION`
- [x] 掩码函数：`MASK`, `MASK_FIRST_N`, `MASK_LAST_N`, `MASK_SHOW_FIRST_N`, `MASK_SHOW_LAST_N`
- [x] `POSEXPLODE` — 带索引的数组展开（LATERAL VIEW 框架已支持）
