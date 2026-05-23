# 代码审查 - 执行摘要和修复优先级

**项目**：gsql - Hive 风格 SQL 查询引擎  
**审查日期**：2026-05-23  
**状态**：完成 ✅

---

## 🎯 一句话总结

**架构优秀、功能完整，但有 6 个 P0 级关键问题需立即修复**（安全+功能），预计 8 小时。

---

## 📊 快速评分表

```
代码可读性 ⭐⭐⭐☆  (3.4/5) - 架构清晰，大函数需拆分
命名规范  ⭐⭐⭐⭐  (4.0/5) - 基本规范
错误处理 ⭐⭐⭐  (3.2/5) - 网络 I/O 缺超时
安全性  ⭐⭐☆  (2.5/5) - 🔴 有严重漏洞
测试覆盖 ⭐⭐⭐  (3.0/5) - engine 78% 很好，plan 20% 偏低
───────────────────────────
总体评分 ⭐⭐⭐  (3.2/5)
```

---

## 🔴 P0 - Critical Issues (6个，8h)

### 1. SSH 密钥验证关闭 ⚠️ **安全漏洞**
- **文件**：pkg/storage/remote.go ~150
- **问题**：`ssh.InsecureIgnoreHostKey()` 导致 MITM 攻击可行
- **修复**：使用 known_hosts 文件验证
- **工时**：2h
```go
// ❌ 当前
HostKeyCallback: ssh.InsecureIgnoreHostKey()

// ✅ 修复
hostKeyCallback, _ := knownhosts.New(filepath.Join(os.Getenv("HOME"), ".ssh/known_hosts"))
HostKeyCallback: hostKeyCallback
```

---

### 2. 网络请求无超时 ⚠️ **可靠性**
- **文件**：pkg/storage/s3.go, remote.go
- **问题**：S3/FTP/SFTP ListObjects/Dial 可无限挂起
- **修复**：使用 `context.WithTimeout`
- **工时**：1.5h
```go
// ❌ 当前
cfg, _ := config.LoadDefaultConfig(context.Background())

// ✅ 修复
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
cfg, _ := config.LoadDefaultConfig(ctx)
```

---

### 3. Window 函数 COUNT 计数错误 ⚠️ **功能异常**
- **文件**：pkg/plan/plan.go:1084-1125
- **问题**：`computeWindowAgg` 中 COUNT 分支和末尾都做 `count++`，导致计数翻倍
- **修复**：仅在 COUNT 分支执行一次 count++
- **工时**：0.5h
```go
// ❌ 当前
case "COUNT":
    if rows[i]["*"] != "" {
        count++  // 第一次
    }
    rows[i][colKey] = strconv.Itoa(count)
// ...
count++  // ❌ 第二次，错误！

// ✅ 修复：仅在 COUNT 分支增加
```

---

### 4. JOIN 节点并发状态问题 ⚠️ **并发不安全**
- **文件**：pkg/plan/plan.go:396-424
- **问题**：`JoinNode.Execute` 直接修改自身 `n.LeftColumn`，defer 恢复不稳定
- **修复**：使用局部变量代替修改 node
- **工时**：1h
```go
// ❌ 当前：修改 node 自身
defer func() { n.LeftColumn, n.RightColumn = ... }()

// ✅ 修复：使用局部变量
leftCol, rightCol := n.LeftColumn, n.RightColumn
if needSwap {
    leftCol, rightCol = rightCol, leftCol
}
// 使用 leftCol/rightCol，不修改 n
```

---

### 5. Git LFS 路径逻辑混淆 ⚠️ **功能异常**
- **文件**：pkg/storage/gitlfs.go:40-80
- **问题**：`repoPath` 和 `lfsPath` 两种配置读取目录错误
- **修复**：清晰定义两种模式逻辑，补充单元测试
- **工时**：1h
```go
// 需明确：当 repoPath 非空时，应读 git_lfs_cache 中 pointer 指向的对象
// 而非尝试读 lfsPath
```

---

### 6. SQL 表达式括号不支持 ⚠️ **功能异常**
- **文件**：pkg/sqlparse/parser.go:882
- **问题**：`parseExpression` 不支持括号，导致 `(a=1 OR b=2) AND c=3` 误解析
- **修复**：实现递归表达式解析，支持括号分组
- **工时**：2h
```go
// ❌ 当前仅支持：comparison (AND|OR) comparison
// ✅ 修复为递归优先级处理（OR < AND）
```

**验证**：执行 samples/query_hive_coverage.sql 应通过

---

## 🟡 P1 - High Priority Issues (8个，12.5h)

| # | 文件 | 问题 | 修复工时 |
|---|---|---|---|
| 7 | s3.go:100 | `io.ReadAll` 大文件 OOM | 1.5h |
| 8 | local.go:330 | 临时文件写入语义不清 | 1h |
| 9 | plan.go:671 | `computeRank` 排序列比较错误 | 1h |
| 10 | plan.go:291 | `computeAggregate` 需类型感知 | 1.5h |
| 11 | catalog.go | Tables map 无并发保护 | 0.5h |
| 12 | parser.go:1238 | 算术表达式优先级错误 | 2h |
| 13 | parser.go:422 | 字符串转义不支持 | 1h |
| 14 | storage/ 多处 | 并发无 errgroup | 2h |

---

## 🟢 P2 - Medium/Low Issues (6个，14h)

| # | 文件 | 问题 | 工时 |
|---|---|---|---|
| 15 | plan.go:798 | 分区剪枝仅支持 AND | 2h |
| 16 | engine.go:130 | buildLogicalPlan 函数拆分 | 2h |
| 17 | parser.go:1049 | parseSelectItems 函数拆分 | 2h |
| 18 | 全局 | 缺 godoc 文档注释 | 4h |
| 19 | 全局 | 魔法字符串常量化 | 1h |
| 20 | storage/ | 网络错误模拟测试 | 3h |

---

## 📈 修复时间线

```
第1周（P0）：8小时
├─ 修复 SSH 密钥验证 (2h)
├─ 添加网络超时 (1.5h)
├─ 修复 COUNT 计数 (0.5h)
├─ 修复 JOIN 并发 (1h)
├─ 修复 Git LFS 路径 (1h)
└─ 修复表达式括号 (2h)

第2-3周（P1）：12.5小时
├─ 大文件流式读取 (1.5h)
├─ 并发错误聚合 (2h)
├─ 类型感知比较 (1.5h)
├─ 算术优先级 (2h)
├─ 字符转义 (1h)
├─ 其他高优先级 (2.5h)
└─ 单元测试补充 (2h)

第4周+（P2）：14小时
├─ 代码重构与拆分 (6h)
├─ 文档注释补充 (4h)
├─ 测试增强 (3h)
└─ 集成测试 (1h)
```

**总计**：34.5 小时 ≈ 1-2 个工作周（并行处理可缩短）

---

## ✅ 测试验证清单

### P0 修复验证
- [ ] 运行 `go test ./...` 所有测试通过
- [ ] samples/query_hive_coverage.sql 可解析
- [ ] TestEngineWindowRowNumber 结果正确（COUNT 无翻倍）
- [ ] 测试并发执行同一 JOIN 计划节点
- [ ] SFTP 连接使用真实主机密钥

### P1 修复验证
- [ ] 大 S3 文件读取内存占用 < 50MB
- [ ] Git LFS 两种配置都可正确读写
- [ ] 并发读取中任意 goroutine 失败时其他正确取消
- [ ] MIN/MAX 聚合对数值字段正确比较

### 整体验证
- [ ] 代码覆盖率 plan: ≥ 40%, storage: ≥ 30%
- [ ] `go vet ./...` 无警告
- [ ] `gofmt -l` 无格式问题
- [ ] 完整 SQL 集成测试通过

---

## 📄 详细报告位置

完整的深度审查报告：[CODE_REVIEW_FULL_REPORT.md](CODE_REVIEW_FULL_REPORT.md)

包含：
- ✅ 5个模块的完整审查
- 🔴 所有关键问题的具体代码位置和修复方案
- 📊 测试覆盖率分析
- 📝 优化建议清单
- 🎯 优先级排序

---

## 🚀 下一步行动

### 立即行动（今天）
1. [ ] 创建分支修复 P0 问题
2. [ ] 在 SSH 连接处打 TODO 注释（等待 known_hosts 支持）
3. [ ] 修复 COUNT 计数错误
4. [ ] 添加网络超时配置

### 本周内
5. [ ] 修复 JOIN 并发问题
6. [ ] 修复 Git LFS 路径混淆
7. [ ] 实现表达式括号支持
8. [ ] 补充单元测试验证

### 本月内
9. [ ] 修复所有 P1 问题
10. [ ] 增加代码覆盖率到 50%+
11. [ ] 补充 godoc 文档

---

**报告发布人**：GitHub Copilot  
**审查模式**：全量深度审查  
**建议阅读时间**：5-10 分钟（本摘要）+ 30 分钟（完整报告）
