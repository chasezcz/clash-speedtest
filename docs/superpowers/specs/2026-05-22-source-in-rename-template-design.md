# 改名模板增强 + CSV 全套记录输出

## 概述

三个增强点：
1. 改名模板新增 `{{.Source}}` 占位符，区分多配置来源
2. 改名模板新增 `{{.JitterMs}}` 和 `{{.PacketLossPct}}` 占位符
3. `-o` 输出 YAML 时自动同名生成 CSV 全套测速记录

---

## 一、来源占位符 `{{.Source}}`

### 来源名称提取规则

在 `LoadProxies()` 中，根据配置路径类型提取来源名：

| 配置路径类型 | 示例 | 来源名 |
| --- | --- | --- |
| 本地文件 | `config1.yaml` | `config1` |
| 带路径的本地文件 | `./path/to/sub.yaml` | `sub` |
| HTTP(S) URL | `https://example.com/config.yaml` | `example.com` |
| Proxy Provider | provider key `myProvider` | `myProvider` |

**本地文件：** `filepath.Base(path)` 后去掉 `.yaml`/`.yml` 后缀。

**HTTP URL：** `url.Parse(path)` 后取 `Hostname()`。

**Proxy Provider：** 使用 provider 的 key name（即 `proxy-providers` YAML 中的键名），而非配置文件名。

### 数据模型变更

**`speedtester.CProxy`（speedtester.go:161）：**

```go
type CProxy struct {
    constant.Proxy
    Config map[string]any
    Source string  // 配置来源名称
}
```

**`speedtester.Result`（speedtester.go:358）：**

新增字段：

```go
Source string `json:"source"`
```

**`ip.NodeNameData`（ip/rename.go:32）：**

新增字段：

```go
Source        string // 配置来源名称
JitterMs      string // 抖动（毫秒）
PacketLossPct string // 丢包率（百分比）
```

### 数据流

1. `LoadProxies()` 根据 configPath 提取来源名，设置 `CProxy.Source`
2. `testProxy()` 将 `proxy.Source` 复制到 `Result.Source`
3. `saveConfig()` 将 `result.Source` 传给 `GenerateNodeNameFromTemplate`
4. `buildNodeNameData()` 填入 `NodeNameData.Source`

---

## 二、模板新增抖动和丢包

### 新占位符

| 占位符 | 说明 | 示例值 |
| --- | --- | --- |
| `{{.JitterMs}}` | 抖动（毫秒），0 时为 `"N/A"` | `"15"` |
| `{{.PacketLossPct}}` | 丢包率（百分比），保留一位小数 | `"2.5"` |

### 数据流

`buildNodeNameData()` 已有 `latency` 参数，需新增 `jitter` 和 `packetLoss` 参数：

```go
func buildNodeNameData(countryCode string, latency, jitter time.Duration, packetLoss, downloadSpeed, uploadSpeed float64, nameCount map[string]int) NodeNameData
```

`GenerateNodeNameFromTemplate` 和 `GenerateNodeName` 同步新增这两个参数。

### 模板示例

```
--rename-template '{{.Source}} | {{.Flag}} {{.CountryCode}} {{.Index}} | {{.Direction}} {{.Speed}}{{.SpeedUnit}} | {{.JitterMs}}ms {{.PacketLossPct}}%'
```

输出：`config1 | 🇺🇸 US 001 | ⬇️ 10.00MB/s | 15ms 2.5%`

---

## 三、CSV 全套记录输出

### 触发方式

传 `-o output.yaml` 时，自动在同目录生成 `output.csv`，无需额外参数。

### CSV 列

| 列名 | 来源 | 说明 |
| --- | --- | --- |
| Source | `Result.Source` | 配置来源 |
| Name | `Result.ProxyName` | 节点名（重命名后的） |
| Type | `Result.ProxyType` | 代理类型 |
| Server | `Result.ProxyConfig["server"]` | 服务器地址 |
| Port | `Result.ProxyConfig["port"]` | 端口 |
| Latency | `Result.Latency` | 平均延迟（ms） |
| Jitter | `Result.Jitter` | 抖动（ms） |
| PacketLoss | `Result.PacketLoss` | 丢包率（%） |
| DownloadSpeed | `Result.DownloadSpeed` | 下载速度（MB/s） |
| UploadSpeed | `Result.UploadSpeed` | 上传速度（MB/s） |

### 实现方式

新增 `output/csv.go`，提供 `WriteCSV(path string, results []*speedtester.Result, filter resultFilter, renameEnabled bool, renameTemplate string) error` 函数。

在 `saveConfig()` 中，YAML 写入成功后调用 `WriteCSV`，CSV 文件名 = YAML 文件名把后缀换成 `.csv`。

CSV 中的 Name 列使用重命名后的名称（与 YAML 中一致），所以 CSV 输出需要在 rename 之后执行。

### 编码细节

- 使用 Go 标准库 `encoding/csv`
- 首行写列头
- 数值列保留两位小数
- 速度单位统一为 MB/s
- 延迟/抖动单位统一为 ms

---

## 四、Gist 多文件上传

当前 `UpdateFile` 只支持单文件。新增 `UpdateFiles` 方法，一次 API 调用上传 YAML + CSV。

### 新增方法

```go
// gist/gist.go
func (u *Uploader) UpdateFiles(token, address string, files map[string][]byte) error
```

内部逻辑与 `UpdateFile` 相同，只是 `updateRequest.Files` 填入多个文件。

### 调用方式

`main.go` 中，若有 gist 配置，同时传入 YAML 和 CSV：

```go
files := map[string][]byte{
    yamlFilename: yamlData,
    csvFilename:  csvData,
}
uploader.UpdateFiles(*gistToken, *gistAddress, files)
```

原有的 `UpdateFile` 保留不变，供 repo 上传等单文件场景使用。

---

## 接口变更汇总

### GenerateNodeNameFromTemplate 新签名

```go
func GenerateNodeNameFromTemplate(tmpl, source, countryCode string, latency, jitter time.Duration, packetLoss, downloadSpeed, uploadSpeed float64, nameCount map[string]int) (string, error)
```

### GenerateNodeName 新签名

```go
func GenerateNodeName(source, countryCode string, latency, jitter time.Duration, packetLoss, downloadSpeed, uploadSpeed float64, nameCount map[string]int) string
```

---

## 需要修改的文件

| 文件 | 变更内容 |
| --- | --- |
| `speedtester/speedtester.go` | `CProxy` 和 `Result` 新增 `Source`；`LoadProxies()` 设置 source；`testProxy()` 传播 source |
| `ip/rename.go` | `NodeNameData` 新增 `Source`/`JitterMs`/`PacketLossPct`；更新函数签名；`buildNodeNameData` 填充新字段 |
| `main.go` | 调用 rename 时传入新参数；更新 `--rename-template` 帮助文本；调用 CSV 输出；gist 上传改为多文件 |
| `output/csv.go`（新建） | CSV 写入逻辑 |
| `gist/gist.go` | 新增 `UpdateFiles` 方法 |
| `ip/rename_test.go` | 所有测试调用新增 source/jitter/packetLoss 参数 |
| `speedtester/speedtester_test.go` | 按需更新 `CProxy` 构造 |

## 边界情况

- **来源名为空：** `{{.Source}}` 渲染为 `""`，不会崩溃或报错
- **来源名重复：** 两个目录下的 `config.yaml` 都产生来源名 `config`，可以接受——用户应自行区分命名
- **去重：** `deduplicateProxiesByServerPort` 保留首次出现的 proxy，source 随之保留
- **Proxy Provider：** 来源是 provider key name，比配置文件名更有意义
- **抖动/丢包为零：** `{{.JitterMs}}` 输出 `"N/A"`，`{{.PacketLossPct}}` 输出 `"0.0"`
- **CSV 无重命名：** 若 `--rename=false`，CSV 的 Name 列使用原始节点名
- **CSV 无过滤：** CSV 与 YAML 使用同一个 `resultFilter`，只包含通过过滤的结果
- **Gist 多文件：** YAML 和 CSV 在一次 PATCH 请求中上传，任一文件更新失败则整体报错
