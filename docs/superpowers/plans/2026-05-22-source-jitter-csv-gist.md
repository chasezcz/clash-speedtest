# 改名模板增强 + CSV 输出 + Gist 多文件上传 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在改名模板中支持 `{{.Source}}`/`{{.JitterMs}}`/`{{.PacketLossPct}}` 占位符，`-o` 时自动生成 CSV 全套记录，gist 支持一次上传多文件。

**Architecture:** 在 `CProxy`/`Result`/`NodeNameData` 数据模型中新增字段，通过现有数据流管道传播。CSV 输出作为独立模块在 `saveConfig()` 中调用。Gist 新增 `UpdateFiles` 方法支持多文件 PATCH。

**Tech Stack:** Go 标准库 `encoding/csv`、`net/url`、`path/filepath`、`text/template`

---

### Task 1: deriveSourceName 辅助函数

**Files:**
- Create: `speedtester/source_test.go`
- Modify: `speedtester/speedtester.go`

- [ ] **Step 1: 编写失败测试**

```go
// speedtester/source_test.go
package speedtester

import "testing"

func TestDeriveSourceName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"config1.yaml", "config1"},
		{"config1.yml", "config1"},
		{"./path/to/sub.yaml", "sub"},
		{"https://example.com/config.yaml", "example.com"},
		{"https://sub.example.com/path/config.yaml", "sub.example.com"},
		{"", ""},
	}
	for _, tt := range tests {
		got := deriveSourceName(tt.input)
		if got != tt.want {
			t.Errorf("deriveSourceName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./speedtester/ -run TestDeriveSourceName -v`
Expected: FAIL — `deriveSourceName` 未定义

- [ ] **Step 3: 实现 deriveSourceName**

在 `speedtester/speedtester.go` 的 `LoadProxies` 函数上方添加：

```go
func deriveSourceName(configPath string) string {
	trimmed := strings.TrimSpace(configPath)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		u, err := url.Parse(trimmed)
		if err != nil {
			return ""
		}
		return u.Hostname()
	}
	base := filepath.Base(trimmed)
	ext := filepath.Ext(base)
	if ext == ".yaml" || ext == ".yml" {
		return strings.TrimSuffix(base, ext)
	}
	return base
}
```

确认 `speedtester.go` 的 import 中有 `"net/url"` 和 `"path/filepath"`。

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./speedtester/ -run TestDeriveSourceName -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add speedtester/source_test.go speedtester/speedtester.go
git commit -m "feat(speedtester): add deriveSourceName helper"
```

---

### Task 2: CProxy 和 Result 新增 Source 字段

**Files:**
- Modify: `speedtester/speedtester.go:161-164, 358-373`
- Modify: `speedtester/speedtester_test.go`

- [ ] **Step 1: 编写失败测试**

在 `speedtester/speedtester_test.go` 末尾添加：

```go
func TestCProxySourceField(t *testing.T) {
	proxy := &CProxy{
		Config: map[string]any{"server": "1.1.1.1", "port": 443},
		Source: "config1",
	}
	if proxy.Source != "config1" {
		t.Fatalf("expected source config1, got %q", proxy.Source)
	}
}

func TestResultSourceField(t *testing.T) {
	result := &Result{Source: "config2"}
	if result.Source != "config2" {
		t.Fatalf("expected source config2, got %q", result.Source)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./speedtester/ -run "TestCProxySourceField|TestResultSourceField" -v`
Expected: FAIL — `Source` 字段不存在

- [ ] **Step 3: 给 CProxy 和 Result 添加 Source 字段**

`speedtester.go:161` — CProxy 结构体：

```go
type CProxy struct {
	constant.Proxy
	Config map[string]any
	Source string
}
```

`speedtester.go:358` — Result 结构体新增字段（在 `ProxyConfig` 之后）：

```go
Source string `json:"source"`
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./speedtester/ -run "TestCProxySourceField|TestResultSourceField" -v`
Expected: PASS

- [ ] **Step 5: 运行全量测试确认无回归**

Run: `go test ./speedtester/... -v`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add speedtester/speedtester.go speedtester/speedtester_test.go
git commit -m "feat(speedtester): add Source field to CProxy and Result"
```

---

### Task 3: LoadProxies 和 testProxy 传播 Source

**Files:**
- Modify: `speedtester/speedtester.go:171-300` (LoadProxies)
- Modify: `speedtester/speedtester.go:443-448` (testProxy)

- [ ] **Step 1: 在 LoadProxies 中设置 CProxy.Source**

在 `speedtester.go` 的 `LoadProxies` 函数中，`for configPath := range` 循环开头（约第 176 行之后）添加：

```go
source := deriveSourceName(configPath)
```

在该循环内创建 `CProxy` 的两处，添加 `Source: source`：

1. 内联 proxies（约第 209 行）：
```go
proxies[name] = &CProxy{
    Proxy:  proxy,
    Config: config,
    Source: source,
}
```

2. Provider proxies（约第 245 行）：
```go
proxies[fmt.Sprintf("[%s] %s", name, proxy.Name())] = &CProxy{
    Proxy:  proxy,
    Config: pdProxies[proxy.Name()],
    Source: name, // provider key name，不是 configPath
}
```

- [ ] **Step 2: 在 testProxy 中传播 Source**

`speedtester.go:443` — testProxy 函数中，Result 初始化处：

```go
result := &Result{
    ProxyName:   name,
    ProxyType:   proxy.Type().String(),
    ProxyConfig: proxy.Config,
    Source:      proxy.Source,
}
```

- [ ] **Step 3: 运行全量测试确认通过**

Run: `go test ./speedtester/... -v`
Expected: PASS

- [ ] **Step 4: 提交**

```bash
git add speedtester/speedtester.go
git commit -m "feat(speedtester): propagate Source from LoadProxies through testProxy"
```

---

### Task 4: NodeNameData 新增字段 + 更新模板函数签名

**Files:**
- Modify: `ip/rename.go`
- Modify: `ip/rename_test.go`

- [ ] **Step 1: 编写失败测试**

在 `ip/rename_test.go` 末尾添加：

```go
func TestGenerateNodeNameWithSourceJitterPacketLoss(t *testing.T) {
	nameCount := make(map[string]int)

	name, err := GenerateNodeNameFromTemplate(
		"{{.Source}} | {{.CountryCode}}-{{.Index}} {{.JitterMs}}ms {{.PacketLossPct}}%",
		"config1", "US", 100*time.Millisecond, 15*time.Millisecond, 2.5, 10*1024*1024, 0, nameCount,
	)
	if err != nil {
		t.Fatalf("template error: %v", err)
	}
	expected := "config1 | US-001 15ms 2.5%"
	if name != expected {
		t.Errorf("Expected %q, got %q", expected, name)
	}
}

func TestGenerateNodeNameSourceOnly(t *testing.T) {
	nameCount := make(map[string]int)

	name := GenerateNodeName("mysub", "HK", 0, 0, 0, 5*1024*1024, 0, nameCount)
	expected := "🇭🇰 HK 001 | ⬆️ 5.00MB/s"
	if name != expected {
		t.Errorf("Expected %q, got %q", expected, name)
	}
}

func TestGenerateNodeNameJitterNA(t *testing.T) {
	nameCount := make(map[string]int)

	name, err := GenerateNodeNameFromTemplate(
		"{{.CountryCode}} {{.JitterMs}}",
		"", "US", 0, 0, 0, 10*1024*1024, 0, nameCount,
	)
	if err != nil {
		t.Fatalf("template error: %v", err)
	}
	if !strings.Contains(name, "N/A") {
		t.Errorf("expected N/A for zero jitter, got %q", name)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./ip/ -run "TestGenerateNodeNameWithSourceJitterPacketLoss|TestGenerateNodeNameSourceOnly|TestGenerateNodeNameJitterNA" -v`
Expected: FAIL — 签名不匹配

- [ ] **Step 3: 更新 NodeNameData 结构体**

`ip/rename.go:32` — 新增三个字段：

```go
type NodeNameData struct {
	Flag              string
	CountryCode       string
	Index             string
	Direction         string
	Speed             string
	SpeedUnit         string
	LatencyMs         string
	DownloadSpeedMBps string
	UploadSpeedMBps   string
	Source            string
	JitterMs          string
	PacketLossPct     string
}
```

- [ ] **Step 4: 更新 buildNodeNameData 签名和实现**

`ip/rename.go:64` — 新签名：

```go
func buildNodeNameData(source, countryCode string, latency, jitter time.Duration, packetLoss, downloadSpeed, uploadSpeed float64, nameCount map[string]int) NodeNameData {
```

在函数末尾的 `return NodeNameData{...}` 中新增：

```go
jitterMs := "N/A"
if jitter > 0 {
    jitterMs = fmt.Sprintf("%d", jitter.Milliseconds())
}
return NodeNameData{
    // ... 已有字段保持不变 ...
    Source:        source,
    JitterMs:      jitterMs,
    PacketLossPct: fmt.Sprintf("%.1f", packetLoss),
}
```

- [ ] **Step 5: 更新 GenerateNodeNameFromTemplate 签名**

`ip/rename.go:47` — 新签名：

```go
func GenerateNodeNameFromTemplate(tmpl, source, countryCode string, latency, jitter time.Duration, packetLoss, downloadSpeed, uploadSpeed float64, nameCount map[string]int) (string, error) {
```

函数内 `buildNodeNameData` 调用更新为：

```go
data := buildNodeNameData(source, countryCode, latency, jitter, packetLoss, downloadSpeed, uploadSpeed, nameCount)
```

- [ ] **Step 6: 更新 GenerateNodeName 签名**

`ip/rename.go:107` — 新签名：

```go
func GenerateNodeName(source, countryCode string, latency, jitter time.Duration, packetLoss, downloadSpeed, uploadSpeed float64, nameCount map[string]int) string {
	name, _ := GenerateNodeNameFromTemplate("", source, countryCode, latency, jitter, packetLoss, downloadSpeed, uploadSpeed, nameCount)
	return name
}
```

- [ ] **Step 7: 更新现有测试调用**

`ip/rename_test.go` 中所有已有测试，按新签名插入参数：

- `GenerateNodeName("US", ...)` → `GenerateNodeName("", "US", ...)`
- `GenerateNodeName("HK", ...)` → `GenerateNodeName("", "HK", ...)`
- `GenerateNodeName("JP", ...)` → `GenerateNodeName("", "JP", ...)`
- `GenerateNodeName("XX", ...)` → `GenerateNodeName("", "XX", ...)`
- `GenerateNodeName("SG", ...)` → `GenerateNodeName("", "SG", ...)`
- `GenerateNodeNameFromTemplate(tmpl, "US", ...)` → `GenerateNodeNameFromTemplate(tmpl, "", "US", ...)`
- `GenerateNodeNameFromTemplate("", "HK", ...)` → `GenerateNodeNameFromTemplate("", "", "HK", ...)`
- `GenerateNodeNameFromTemplate("{{.Invalid", "US", ...)` → `GenerateNodeNameFromTemplate("{{.Invalid", "", "US", ...)`
- `GenerateNodeNameFromTemplate(tmpl, "DE", ...)` → `GenerateNodeNameFromTemplate(tmpl, "", "DE", ...)`

每个调用在 `countryCode` 前插入 `""`（空 source），在 `latency` 后插入 `0, 0`（jitter, packetLoss）。

- [ ] **Step 8: 运行测试确认通过**

Run: `go test ./ip/... -v`
Expected: PASS

- [ ] **Step 9: 提交**

```bash
git add ip/rename.go ip/rename_test.go
git commit -m "feat(ip): add Source/JitterMs/PacketLossPct to rename template"
```

---

### Task 5: main.go 集成 rename 新参数

**Files:**
- Modify: `main.go:212-224` (saveConfig 中的 rename 调用)
- Modify: `main.go:50` (rename-template 帮助文本)

- [ ] **Step 1: 更新 rename-template 帮助文本**

`main.go:50`：

```go
renameTemplate = flag.String("rename-template", "", `name template for renaming (Go text/template). Placeholders: {{.Flag}}, {{.CountryCode}}, {{.Index}}, {{.Direction}}, {{.Speed}}, {{.SpeedUnit}}, {{.LatencyMs}}, {{.DownloadSpeedMBps}}, {{.UploadSpeedMBps}}, {{.Source}}, {{.JitterMs}}, {{.PacketLossPct}}. Empty = default format`)
```

- [ ] **Step 2: 更新 saveConfig 中的 rename 调用**

`main.go:218` — 更新为新签名：

```go
name, err := ip.GenerateNodeNameFromTemplate(*renameTemplate, result.Source, location.CountryCode, result.Latency, result.Jitter, result.PacketLoss, result.DownloadSpeed, result.UploadSpeed, nameCount)
```

`main.go:221` — fallback 调用：

```go
name = ip.GenerateNodeName(result.Source, location.CountryCode, result.Latency, result.Jitter, result.PacketLoss, result.DownloadSpeed, result.UploadSpeed, nameCount)
```

- [ ] **Step 3: 构建确认编译通过**

Run: `go build -o clash-speedtest .`
Expected: 成功

- [ ] **Step 4: 运行全量测试**

Run: `go test ./... -v`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add main.go
git commit -m "feat(main): wire Source/Jitter/PacketLoss to rename calls and update help text"
```

---

### Task 6: CSV 输出模块

**Files:**
- Create: `output/csv_test.go`
- Create: `output/csv.go`
- Modify: `main.go` (saveConfig 调用)

- [ ] **Step 1: 编写失败测试**

```go
// output/csv_test.go
package output

import (
	"encoding/csv"
	"os"
	"testing"
	"time"

	"github.com/faceair/clash-speedtest/speedtester"
)

func TestWriteCSV(t *testing.T) {
	results := []*speedtester.Result{
		{
			ProxyName:   "node-1",
			ProxyType:   "ss",
			ProxyConfig: map[string]any{"server": "1.1.1.1", "port": 443},
			Source:      "config1",
			Latency:     100 * time.Millisecond,
			Jitter:      15 * time.Millisecond,
			PacketLoss:  2.5,
			DownloadSpeed: 10 * 1024 * 1024,
			UploadSpeed:   5 * 1024 * 1024,
		},
	}

	tmpFile := t.TempDir() + "/test.csv"
	if err := WriteCSV(tmpFile, results); err != nil {
		t.Fatalf("WriteCSV error: %v", err)
	}

	f, err := os.Open(tmpFile)
	if err != nil {
		t.Fatalf("open csv: %v", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 rows (header + 1 data), got %d", len(records))
	}

	header := records[0]
	expectedHeader := []string{"Source", "Name", "Type", "Server", "Port", "Latency", "Jitter", "PacketLoss", "DownloadSpeed", "UploadSpeed"}
	if len(header) != len(expectedHeader) {
		t.Fatalf("header length mismatch: got %d, want %d", len(header), len(expectedHeader))
	}
	for i, h := range header {
		if h != expectedHeader[i] {
			t.Errorf("header[%d] = %q, want %q", i, h, expectedHeader[i])
		}
	}

	row := records[1]
	if row[0] != "config1" {
		t.Errorf("Source = %q, want %q", row[0], "config1")
	}
	if row[1] != "node-1" {
		t.Errorf("Name = %q, want %q", row[1], "node-1")
	}
	if row[2] != "ss" {
		t.Errorf("Type = %q, want %q", row[2], "ss")
	}
	if row[3] != "1.1.1.1" {
		t.Errorf("Server = %q, want %q", row[3], "1.1.1.1")
	}
}

func TestWriteCSVDerivesFilename(t *testing.T) {
	got := CSVPathFromYAML("/tmp/output.yaml")
	want := "/tmp/output.csv"
	if got != want {
		t.Errorf("CSVPathFromYAML = %q, want %q", got, want)
	}

	got2 := CSVPathFromYAML("/tmp/output.yml")
	want2 := "/tmp/output.csv"
	if got2 != want2 {
		t.Errorf("CSVPathFromYAML(.yml) = %q, want %q", got2, want2)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./output/ -run "TestWriteCSV" -v`
Expected: FAIL — `WriteCSV` 未定义

- [ ] **Step 3: 实现 CSV 输出**

```go
// output/csv.go
package output

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/faceair/clash-speedtest/speedtester"
)

var csvHeader = []string{"Source", "Name", "Type", "Server", "Port", "Latency", "Jitter", "PacketLoss", "DownloadSpeed", "UploadSpeed"}

func WriteCSV(path string, results []*speedtester.Result) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv file %s: %w", path, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write(csvHeader); err != nil {
		return err
	}

	for _, r := range results {
		server, _ := r.ProxyConfig["server"].(string)
		port := fmt.Sprintf("%v", r.ProxyConfig["port"])

		latencyMs := ""
		if r.Latency > 0 {
			latencyMs = fmt.Sprintf("%.2f", float64(r.Latency.Milliseconds()))
		}
		jitterMs := ""
		if r.Jitter > 0 {
			jitterMs = fmt.Sprintf("%.2f", float64(r.Jitter.Milliseconds()))
		}

		row := []string{
			r.Source,
			r.ProxyName,
			r.ProxyType,
			server,
			port,
			latencyMs,
			jitterMs,
			fmt.Sprintf("%.1f", r.PacketLoss),
			fmt.Sprintf("%.2f", r.DownloadSpeed/(1024*1024)),
			fmt.Sprintf("%.2f", r.UploadSpeed/(1024*1024)),
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

func CSVPathFromYAML(yamlPath string) string {
	ext := filepath.Ext(yamlPath)
	return strings.TrimSuffix(yamlPath, ext) + ".csv"
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./output/ -run "TestWriteCSV" -v`
Expected: PASS

- [ ] **Step 5: 在 saveConfig 中调用 CSV 输出**

`main.go` 的 `saveConfig` 函数中，YAML 写入成功后（约第 238 行 `outputFilename` 之后）添加：

```go
csvPath := output.CSVPathFromYAML(*outputPath)
if err := output.WriteCSV(csvPath, results); err != nil {
    log.Printf("write csv failed: %s", err)
}
```

- [ ] **Step 6: 构建并运行全量测试**

Run: `go build -o clash-speedtest . && go test ./... -v`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add output/csv.go output/csv_test.go main.go
git commit -m "feat(output): add CSV full-record export alongside YAML output"
```

---

### Task 7: Gist 多文件上传

**Files:**
- Modify: `gist/gist.go`
- Modify: `gist/gist_test.go`
- Modify: `main.go` (gist 上传调用)

- [ ] **Step 1: 编写失败测试**

在 `gist/gist_test.go` 中添加：

```go
func TestUpdateFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		var req updateRequest
		json.NewDecoder(r.Body).Decode(&req)
		if len(req.Files) != 2 {
			t.Errorf("expected 2 files, got %d", len(req.Files))
		}
		if _, ok := req.Files["output.yaml"]; !ok {
			t.Error("missing output.yaml")
		}
		if _, ok := req.Files["output.csv"]; !ok {
			t.Error("missing output.csv")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	uploader := NewUploaderWithBase(server.Client(), server.URL)
	files := map[string][]byte{
		"output.yaml": []byte("proxies: []"),
		"output.csv":  []byte("Source,Name\n"),
	}
	err := uploader.UpdateFiles("test-token", "abc123", files)
	if err != nil {
		t.Fatalf("UpdateFiles error: %v", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./gist/ -run TestUpdateFiles -v`
Expected: FAIL — `UpdateFiles` 未定义

- [ ] **Step 3: 实现 UpdateFiles**

在 `gist/gist.go` 的 `UpdateFile` 函数之后添加：

```go
func (u *Uploader) UpdateFiles(token, address string, files map[string][]byte) error {
	if token == "" {
		return fmt.Errorf("gist token is empty")
	}
	if len(files) == 0 {
		return fmt.Errorf("no files to upload")
	}

	gistID, err := ParseGistID(address)
	if err != nil {
		return err
	}

	gistFiles := make(map[string]gistFile, len(files))
	for name, content := range files {
		gistFiles[name] = gistFile{Content: string(content)}
	}
	payload := updateRequest{Files: gistFiles}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("build gist payload for %s failed: %w", gistID, err)
	}

	request, err := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/gists/%s", u.apiBase, gistID), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request for gist %s failed: %w", gistID, err)
	}
	request.Header.Set("Authorization", "token "+token)
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", u.userAgent)

	resp, err := u.client.Do(request)
	if err != nil {
		return fmt.Errorf("update gist %s request failed: %w", gistID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		responseBody := readResponseBody(resp.Body)
		return fmt.Errorf("update gist %s failed: status %s, body: %s", gistID, resp.Status, responseBody)
	}

	return nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./gist/ -run TestUpdateFiles -v`
Expected: PASS

- [ ] **Step 5: 更新 main.go gist 上传为多文件**

`main.go` 的 `saveConfig` 中，gist 上传部分（约第 241-246 行）替换为：

```go
if *gistToken != "" && *gistAddress != "" {
    uploader := gist.NewUploader(nil)
    csvPath := output.CSVPathFromYAML(*outputPath)
    files := map[string][]byte{outputFilename: yamlData}
    if csvData, err := os.ReadFile(csvPath); err == nil {
        files[filepath.Base(csvPath)] = csvData
    }
    if err := uploader.UpdateFiles(*gistToken, *gistAddress, files); err != nil {
        log.Printf("update gist failed: %s", err)
    }
}
```

注意：这里读取 CSV 文件是因为 CSV 在前面的步骤已经写入了。如果 CSV 写入失败（被 log 了），这里 ReadFile 会失败，files 中就只有 YAML，降级为单文件上传。

- [ ] **Step 6: 构建并运行全量测试**

Run: `go build -o clash-speedtest . && go test ./... -v`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add gist/gist.go gist/gist_test.go main.go
git commit -m "feat(gist): add UpdateFiles for multi-file upload in single API call"
```

---

### Task 8: 最终验证

- [ ] **Step 1: 运行全量测试**

Run: `go test ./... -v`
Expected: PASS

- [ ] **Step 2: 静态检查**

Run: `go vet ./...`
Expected: 无输出

- [ ] **Step 3: 构建二进制**

Run: `go build -o clash-speedtest .`
Expected: 成功

- [ ] **Step 4: 手动验证 help 文本**

Run: `./clash-speedtest -h 2>&1 | grep rename-template`
Expected: 输出包含 `{{.Source}}`, `{{.JitterMs}}`, `{{.PacketLossPct}}`
