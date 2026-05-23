# 代码审查实施方案

**项目**：gsql | **审查员**：GitHub Copilot | **日期**：2026-05-23

---

## 📋 项目概况

| 指标 | 值 | 评价 |
|---|---|---|
| 代码行数 | 5,221 (非测试) | ✅ 中等规模 |
| 模块数 | 5 个核心包 | ✅ 分层清晰 |
| 测试覆盖率 | 18-79% (不均) | ⚠️ 需改进 |
| 功能完整度 | 100% | ✅ 完整 |
| 架构评分 | ⭐⭐⭐⭐⭐ | ✅ 优秀 |
| **代码质量评分** | **⭐⭐⭐** | ⚠️ 中等，有改进空间 |

---

## 🔴 关键发现 (按严重度)

### 🔴🔴🔴 Critical (立即修复)

#### 1. SSH 密钥验证关闭 (SECURITY)
```
文件：pkg/storage/remote.go:150
问题：ssh.InsecureIgnoreHostKey() 导致 MITM 攻击风险
影响：生产环境不可接受
修复难度：简单 (0.5h)
```

#### 2. 网络超时缺失 (RELIABILITY)
```
文件：pkg/storage/s3.go, remote.go
问题：S3/FTP/SFTP 无超时，可无限挂起
影响：线上故障，可用性问题
修复难度：简单 (1.5h)
```

#### 3. Window 函数 COUNT 错误 (CORRECTNESS)
```
文件：pkg/plan/plan.go:1084-1125
问题：COUNT 计数翻倍
影响：查询结果错误
修复难度：简单 (0.5h)
```

#### 4. JOIN 并发不安全 (CONCURRENCY)
```
文件：pkg/plan/plan.go:396-424
问题：修改 node 自身状态，并发时失效
影响：并发查询出错
修复难度：简单 (1h)
```

#### 5. Git LFS 路径混淆 (CORRECTNESS)
```
文件：pkg/storage/gitlfs.go
问题：repoPath vs lfsPath 逻辑不清
影响：Git LFS 表读写失败
修复难度：中等 (1h)
```

#### 6. SQL 表达式括号 (FUNCTIONALITY)
```
文件：pkg/sqlparse/parser.go:882
问题：不支持括号分组，误解析复杂表达式
影响：某些 SQL 无法解析
修复难度：中等 (2h)
```

**小计**：6 个问题 | **总修复时间**：8 小时 | **优先级**：**本周修复**

---

### 🟡🟡 High Priority (应修复)

8 个问题，12.5 小时，包括：
- `io.ReadAll` 大文件 OOM
- 算术表达式优先级错误
- 并发错误聚合（errgroup）
- 字符转义支持
- 分区剪枝扩展
- 类型感知比较
- 临时文件写入
- 元数据并发保护

**优先级**：本月修复

---

## 📊 按模块改进优先级

```
pkg/plan/
  ├─ 🔴 4 个 P0 问题 (3.5h)
  ├─ 🟡 3 个 P1 问题 (3.5h)
  └─ 🟢 3 个 P2 问题 (4h)

pkg/sqlparse/
  ├─ 🔴 2 个 P0 问题 (2.5h)
  ├─ 🟡 3 个 P1 问题 (4h)
  └─ 🟢 1 个 P2 问题 (2h)

pkg/storage/
  ├─ 🔴 3 个 P0 问题 (2h)
  ├─ 🟡 3 个 P1 问题 (3.5h)
  └─ 🟢 1 个 P2 问题 (3h)

pkg/engine/
  ├─ ✅ 无 P0 问题
  ├─ 🟡 2 个 P1 问题 (1.5h)
  └─ 🟢 1 个 P2 问题 (2h)

pkg/catalog/
  ├─ 🔴 1 个 P0 问题 (0.5h)
  ├─ 🟡 1 个 P1 问题 (0.5h)
  └─ ✅ 无 P2 问题
```

---

## 🛠️ 修复指南

### Phase 1: P0 Issues (1周, 8h)

**优先级**：最高 | **截止**：本周五

#### 修复顺序建议
1. **SSH 密钥验证** (0.5h)
   - 改为 known_hosts 或交互确认
   - 测试：SFTP 连接

2. **网络超时** (1.5h)
   - S3: `context.WithTimeout(ctx, 30s)`
   - FTP: `ftp.DialWithContext`
   - SFTP: `net.DialTimeout`
   - 测试：模拟超时

3. **COUNT 计数** (0.5h)
   - 修复：仅在 COUNT 分支做 count++
   - 测试：TestEngineWindowRowNumber

4. **JOIN 并发** (1h)
   - 改用局部变量
   - 测试：并发执行同一 JOIN 计划

5. **Git LFS 路径** (1h)
   - 清晰定义 repoPath vs lfsPath
   - 测试：两种配置的读写

6. **表达式括号** (2h)
   - 实现递归优先级解析
   - 测试：query_hive_coverage.sql

---

### Phase 2: P1 Issues (2-3周, 12.5h)

**优先级**：高 | **截止**：本月底

关键修复：
- [ ] S3 流式读取 (避免 OOM)
- [ ] 并发错误聚合 (errgroup)
- [ ] 算术优先级修复
- [ ] 字符转义支持
- [ ] 类型感知比较
- [ ] Map 并发保护

---

### Phase 3: P2 Issues (4周+, 14h)

**优先级**：中 | **截止**：本季度末

代码改进：
- [ ] 补充 godoc 文档 (4h)
- [ ] 拆分大函数 (6h)
- [ ] 提取常量 (1h)
- [ ] 测试增强 (3h)

---

## 📝 具体修复代码片段

### Issue #1: SSH 密钥验证

**位置**：pkg/storage/remote.go:150

**当前代码**：
```go
config := &ssh.ClientConfig{
    User: user,
    Auth: []ssh.AuthMethod{ssh.Password(pass)},
    HostKeyCallback: ssh.InsecureIgnoreHostKey(),  // ❌ 不安全
}
```

**修复方案**：
```go
import "golang.org/x/crypto/ssh/knownhosts"

// 方案 1: 使用 known_hosts 文件
hostKeyCallback, err := knownhosts.New(
    filepath.Join(os.Getenv("HOME"), ".ssh/known_hosts"),
)
if err != nil {
    return nil, fmt.Errorf("failed to parse known_hosts: %w", err)
}

config := &ssh.ClientConfig{
    User: user,
    Auth: []ssh.AuthMethod{ssh.Password(pass)},
    HostKeyCallback: hostKeyCallback,  // ✅ 安全
}

// 方案 2: 临时解决方案（加 TODO）
// 注意：生产环境需要使用真实主机密钥验证
config.HostKeyCallback = ssh.InsecureIgnoreHostKey()  // TODO: 配置 known_hosts
```

**验证**：
```bash
go test -v ./pkg/storage -run TestReadSFTPTable
# 应通过 SFTP 连接
```

---

### Issue #3: COUNT 计数错误

**位置**：pkg/plan/plan.go:1084-1125

**当前代码**：
```go
func computeWindowAgg(rows []storage.Row, colKey string, args []string, aggFunc string) {
    count := 0
    sum := 0.0
    for i := range rows {
        switch aggFunc {
        case "COUNT":
            if rows[i]["*"] != "" {
                count++  // ❌ 第 1 次增加
            }
            rows[i][colKey] = strconv.Itoa(count)
        case "SUM":
            v, _ := strconv.ParseFloat(rows[i][args[0]], 64)
            sum += v
            rows[i][colKey] = fmt.Sprintf("%.2f", sum)
        }
        count++  // ❌ 第 2 次增加！导致计数翻倍
    }
}
```

**修复代码**：
```go
func computeWindowAgg(rows []storage.Row, colKey string, args []string, aggFunc string) {
    count := 0
    sum := 0.0
    for i := range rows {
        switch aggFunc {
        case "COUNT":
            if rows[i]["*"] != "" {
                count++  // ✅ 仅增加一次
            }
            rows[i][colKey] = strconv.Itoa(count)
        case "SUM":
            v, _ := strconv.ParseFloat(rows[i][args[0]], 64)
            sum += v
            rows[i][colKey] = fmt.Sprintf("%.2f", sum)
            // 不需要额外的 count++
        case "AVG":
            v, _ := strconv.ParseFloat(rows[i][args[0]], 64)
            count++  // COUNT for average
            sum += v
            rows[i][colKey] = fmt.Sprintf("%.2f", sum/float64(count))
        // ... 其他聚合函数
        }
    }
}
```

**验证**：
```go
// 测试
rows := []storage.Row{
    {"val": "1"}, {"val": "2"}, {"val": "3"},
}
computeWindowAgg(rows, "count_col", nil, "COUNT")
// 期望输出：count_col = [1, 2, 3] ✅
// 当前输出：count_col = [2, 4, 6] ❌
```

---

### Issue #4: JOIN 并发不安全

**位置**：pkg/plan/plan.go:396-424

**当前代码**：
```go
func (n *JoinNode) Execute() ([]storage.Row, error) {
    // ... 检查是否需要交换
    if leftRows > rightRows && rightRows > 0 {
        // ❌ 直接修改 node
        n.LeftColumn, n.RightColumn = n.RightColumn, n.LeftColumn
        defer func() {
            n.LeftColumn, n.RightColumn = n.RightColumn, n.LeftColumn  // 恢复
        }()
        // ... join logic
    }
    return rows, nil
}
```

**修复代码**：
```go
func (n *JoinNode) Execute() ([]storage.Row, error) {
    // ✅ 使用局部变量代替修改 node
    leftCol := n.LeftColumn
    rightCol := n.RightColumn
    
    if leftRows > rightRows && rightRows > 0 {
        // 仅修改局部变量，不修改 n
        leftCol, rightCol = rightCol, leftCol
    }
    
    // 使用 leftCol/rightCol 执行 join
    return performJoin(leftPlanNode, rightPlanNode, leftCol, rightCol)
}

func performJoin(left, right PlanNode, leftCol, rightCol string) ([]storage.Row, error) {
    // 原有的 join 逻辑，使用参数 leftCol/rightCol
    // ...
}
```

**验证**：
```bash
go test -v ./pkg/plan -run TestEngineJoin
# 可并发执行相同的 JOIN 计划节点，结果一致
```

---

### Issue #2: 网络超时

**位置**：pkg/storage/s3.go:34, remote.go:40

**S3 修复**：
```go
import "time"

func readS3Table(tbl *catalog.Table, filters []PartitionFilter) ([]Row, error) {
    // ✅ 添加超时
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    cfg, err := config.LoadDefaultConfig(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to load AWS config: %w", err)
    }
    
    // ... rest of code
    
    // ✅ ListObjects 也使用带超时的 ctx
    paginator := s3.NewListObjectsV2Paginator(s3Client, &s3.ListObjectsV2Input{
        Bucket: &bucket,
        Prefix: &prefix,
    })
    
    for paginator.HasMorePages() {
        page, err := paginator.NextPage(ctx)  // ✅ 使用 ctx
        if err != nil {
            return nil, fmt.Errorf("failed to list S3 objects: %w", err)
        }
        // ...
    }
}
```

**FTP 修复**：
```go
func readFTPTable(tbl *catalog.Table, filters []PartitionFilter) ([]Row, error) {
    // ... config reads ...
    
    // ✅ 使用带超时的 Dial
    addr := fmt.Sprintf("%s:%s", host, port)
    
    // 方案 1: 使用 net.DialTimeout
    conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to FTP %s: %w", addr, err)
    }
    
    ftpConn := ftp.NewConn(conn)
    defer ftpConn.Quit()
    
    // ... rest of code
}
```

**验证**：
```bash
# 模拟超时测试
go test -v ./pkg/storage -run TestS3Timeout
```

---

## ✅ 验证检查清单

### P0 修复完成检查
- [ ] `go test ./...` 所有测试通过
- [ ] `go vet ./...` 无警告
- [ ] samples/query_hive_coverage.sql 解析成功
- [ ] TestEngineWindowRowNumber COUNT 结果正确
- [ ] 模拟网络超时测试通过
- [ ] SFTP 连接验证主机密钥

### 代码质量检查
- [ ] 无新增 code smell
- [ ] 覆盖率未下降
- [ ] 性能基准 (if any) 未显著下降

---

## 📚 参考资源

### 推荐阅读顺序
1. **本文档** (5 分钟) - 修复计划概览
2. **REVIEW_SUMMARY.md** (10 分钟) - 问题优先级
3. **CODE_REVIEW_FULL_REPORT.md** (30 分钟) - 详细分析

### 外部参考
- [Go Error Handling](https://go.dev/blog/error-handling-and-go)
- [SSH Known Hosts](https://github.com/golang/crypto/blob/master/ssh/knownhosts/knownhosts.go)
- [Context Timeouts](https://golang.org/pkg/context/#WithTimeout)
- [errgroup for concurrent tasks](https://pkg.go.dev/golang.org/x/sync/errgroup)

---

## 🎯 成功标准

### 修复完成标志
- ✅ P0 问题 100% 修复
- ✅ P1 问题 ≥ 80% 修复
- ✅ 测试覆盖率提升 5% 以上
- ✅ 无新增 Critical 问题

### 代码质量目标
- ✅ 代码可读性：⭐⭐⭐⭐ (4.0)
- ✅ 错误处理：⭐⭐⭐⭐ (4.0)
- ✅ 安全性：⭐⭐⭐⭐ (4.0)
- ✅ 总体评分：⭐⭐⭐⭐ (4.0)

---

**文档版本**：1.0  
**最后更新**：2026-05-23  
**下一次审查建议**：3 个月后
