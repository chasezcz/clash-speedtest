# 设计：AI 服务解锁检测 + 延迟测试重试

## 概述

两个独立功能：

1. **延迟重试**：延迟测试多轮执行，合并所有成功 ping，提高 flaky 代理的测试成功率
2. **AI 服务解锁检测**：通过代理检测 OpenAI、Claude、Codex 是否可达，结果写入重命名模板

## 功能一：延迟重试

### 动机

部分代理首次连接失败（N/A），但重试可能成功。当前 6 次 ping 全部失败就放弃，导致可用节点被标记为不可用。

### 设计

**CLI flag：**
```
-latency-retries 2    延迟测试轮数，每轮 6 次 ping（默认 2）
```

**行为：**
- 始终执行所有轮次（不提前停止）
- 每轮使用新的 http.Client（新连接）
- 每轮内 100ms 间隔
- 合并所有轮次的成功 ping 计算统计
- packetLoss = 总失败数 / (轮数 × 6) × 100

**`testLatency` 改造：**

```go
func (st *SpeedTester) testLatency(proxy constant.Proxy, minLatency time.Duration) *latencyResult {
    var allLatencies []time.Duration
    totalPings := st.config.LatencyRetries * 6
    failedPings := 0

    for range st.config.LatencyRetries {
        client := st.createClient(proxy, minLatency)
        for range 6 {
            time.Sleep(100 * time.Millisecond)
            start := time.Now()
            req, _ := http.NewRequest(http.MethodHead, st.downloadURL, nil)
            resp, err := client.Do(req)
            if err != nil {
                failedPings++
                continue
            }
            resp.Body.Close()
            allLatencies = append(allLatencies, time.Since(start))
        }
        client.CloseIdleConnections()
    }

    return calculateLatencyStats(allLatencies, failedPings, totalPings)
}
```

**`calculateLatencyStats` 签名变更：**

```go
func calculateLatencyStats(latencies []time.Duration, failedPings, totalPings int) *latencyResult
```

packetLoss 计算：`float64(failedPings) / float64(totalPings) * 100`

## 功能二：AI 服务解锁检测

### 动机

用户需要知道代理节点能否访问 OpenAI、Claude、Codex 等 AI 服务，以便在重命名中标注解锁状态。

### 设计

**新增 `unlock/` 包：**

```
unlock/
  unlock.go        # 核心检测逻辑
  unlock_test.go   # 测试
```

**服务定义：**

| 服务 | 检测 URL | 判断逻辑 |
|------|---------|---------|
| OpenAI | `https://api.openai.com/v1/models` | 401 + body 含 `"Invalid API Key"` → 解锁 |
| Claude | `https://api.anthropic.com/v1/messages` | 401 + body 含 `"invalid x-api-key"` → 解锁 |
| Codex | `https://chatgpt.com/` | 200 + body 含特定标记 → 解锁 |

**核心接口：**

```go
func CheckServices(proxy constant.Proxy, timeout time.Duration) []string
```

- 所有服务并发检测
- 通过 proxy.DialContext 发 HTTP 请求
- 返回解锁的服务名称列表（如 `["OpenAI", "Codex"]`）
- 全部超时/失败返回空切片

**CLI flag：**
```
-unlock              检测 AI 服务解锁状态（默认关闭）
-unlock-timeout 10s  解锁检测超时（默认 10s）
```

**Result 扩展：**

```go
type Result struct {
    // ... existing fields ...
    UnlockServices []string `json:"unlock_services,omitempty"`
}
```

**NodeNameData 扩展：**

```go
type NodeNameData struct {
    // ... existing fields ...
    Unlock string // "OpenAI|Claude" 格式，空表示无解锁服务
}
```

**模板渲染：**
- 有解锁服务：`{{.Unlock}}` → `"OpenAI|Codex"`
- 无解锁服务：`{{.Unlock}}` → `""`

### 流程

```
测速 → testProxy:
  1. testLatency（多轮重试）
  2. 如果 -unlock → unlock.CheckServices → 写入 Result.UnlockServices
  3. download/upload

saveConfig rename:
  1. GetIPLocationViaProxy → 出口国家
  2. GenerateNodeNameFromTemplate（含 Unlock 字段）
```

## 文件变更清单

| 文件 | 变更类型 | 说明 |
|------|---------|------|
| `unlock/unlock.go` | 新增 | 服务定义 + CheckServices |
| `unlock/unlock_test.go` | 新增 | 单元测试 |
| `speedtester/speedtester.go` | 修改 | Result 加 UnlockServices，Config 加 LatencyRetries，testLatency 多轮重试，calculateLatencyStats 签名变更 |
| `speedtester/speedtester_test.go` | 修改 | 更新延迟测试相关用例 |
| `ip/rename.go` | 修改 | NodeNameData 加 Unlock 字段，buildNodeNameData 接收 unlock 参数 |
| `ip/rename_test.go` | 修改 | 更新测试 |
| `main.go` | 修改 | 加 -unlock/-unlock-timeout/-latency-retries flag，testProxy 调 unlock |
