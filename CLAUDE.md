# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

Clash-SpeedTest 是一个基于 Clash/Mihomo 核心的代理测速工具。读取 Clash 配置文件或订阅 URL，自动解析代理节点并进行延迟/下载/上传测速，支持交互式 TUI 和 TSV 管道两种输出模式。

## 常用命令

```bash
# 构建
go build -o clash-speedtest .
# 带版本信息构建
go build -ldflags "-X main.version=dev -X main.commit=$(git rev-parse --short HEAD)" -tags=with_gvisor -o clash-speedtest .

# 测试
go test ./...
# 单个包测试
go test ./speedtester/...
# 单个测试函数
go test -run TestFunctionName ./package/...

# 静态检查
go vet ./...

# 本地安装
go install github.com/faceair/clash-speedtest@latest
```

## 构建注意事项

- Go 1.24+，无 Makefile，标准 Go 工具链
- CGO_ENABLED=0（纯静态二进制），构建标签 `with_gvisor`（gVisor 网络栈）
- goreleaser 负责交叉编译和发版，通过 ldflags 注入 `main.version` 和 `main.commit`
- CI 仅在 tag push 时触发 goreleaser release

## 架构

### 数据流管线

```
配置文件/URL → LoadProxies() 解析 YAML → 正则/关键词过滤 → 去重 → 逐节点测速 → 输出
```

1. **加载**: `speedtester.LoadProxies()` 读取配置文件（本地或 HTTP），解析 `proxies` 和 `proxy-providers`，通过 mihomo 的 `adapter.ParseProxy()` 创建代理对象
2. **过滤**: 应用 `-f` 正则和 `-b` 屏蔽关键词，按 server:port 去重
3. **测速**: `TestProxiesUntil()` 逐节点执行：延迟（6次 HEAD）→ 并发下载（默认4协程）→ 可选上传；支持 early-stop
4. **输出**: 交互式 TUI（Bubble Tea）或 TSV（管道重定向时自动切换）
5. **后处理**: 排序、过滤、重命名（国旗+速率）、写入 YAML、可选上传 GitHub Gist/Repo

### 包职责

| 包 | 职责 |
|---|------|
| `main.go` | CLI flag 解析、流程编排、输出后处理 |
| `speedtester/` | 核心测速引擎：代理加载、延迟/下载/上传测试、Result 结构体 |
| `output/` | 输出格式化：TSV vs Interactive 模式检测、列头、行格式、排序 |
| `tui/` | Bubble Tea TUI：表格渲染、排序、详情面板、进度条、ETA |
| `ip/` | IP 地理位置查询（ip-api.com）、节点重命名（国旗 emoji + 速率模板） |
| `gist/` | GitHub Gist 和 Repo 文件上传客户端 |
| `download-server/` | 独立 HTTP 测试服务端（`/__down?bytes=N`、`/__up`） |

### 关键类型

- `speedtester.SpeedMode`: `"fast"`（仅延迟）/ `"download"` / `"full"`
- `speedtester.Config`: 所有测试参数
- `speedtester.CProxy`: 封装 mihomo `constant.Proxy` + 原始配置 map
- `speedtester.Result`: 测试结果（延迟、抖动、丢包率、下载/上传速率）
- `output.OutputMode`: `TSV` / `Interactive`，通过 `term.IsTerminal(stdout)` 自动判断
- `tui.tuiModel`: Bubble Tea Model（Elm 架构）

### 支持的代理类型

SS, SSR, Snell, Socks5, HTTP, VMess, VLESS, Trojan, Hysteria, Hysteria2, WireGuard, TUIC, SSH, Mieru, AnyTLS, Sudoku

### 核心依赖

- `metacubex/mihomo` — 代理协议解析和连接（上游 Clash 核心）
- `charmbracelet/bubbletea` + `bubbles` + `lipgloss` — TUI 框架
- `golang.org/x/term` — 终端检测
- `gopkg.in/yaml.v2` — YAML 解析

## 开发约定

- 无 linter 配置，用 `go vet ./...` 作为底线检查
- 每个包都有对应测试文件，TUI 包测试覆盖较全面
- `output.IsTerminal` 是可替换的函数变量，方便测试中 mock 终端检测
- `tui/perf.go` 可通过环境变量 `CLASH_SPEEDTEST_TUI_PERF=1` 启用 TUI 性能埋点
