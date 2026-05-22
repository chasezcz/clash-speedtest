package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/faceair/clash-speedtest/gist"
	"github.com/faceair/clash-speedtest/ip"
	"github.com/faceair/clash-speedtest/output"
	"github.com/faceair/clash-speedtest/speedtester"
	"github.com/faceair/clash-speedtest/tui"
	"github.com/faceair/clash-speedtest/unlock"
	mihomolog "github.com/metacubex/mihomo/log"
	"gopkg.in/yaml.v2"
)

// Version information injected via ldflags during build
var (
	version = "dev"
	commit  = "unknown"
)

var (
	configPathsConfig = flag.String("c", "", "config file path, also support http(s) url")
	filterRegexConfig = flag.String("f", ".+", "filter proxies by name, use regexp")
	blockKeywords     = flag.String("b", "", "block proxies by keywords, use | to separate multiple keywords (example: -b 'rate|x1|1x')")
	serverURL         = flag.String("server-url", "https://dl.google.com/chrome/mac/universal/stable/GGRO/googlechrome.dmg", "server url or direct download url")
	speedMode         = flag.String("speed-mode", "download", "speed test mode: fast, download, full")
	downloadSize      = flag.Int("download-size", 50*1024*1024, "download size for testing proxies")
	uploadSize        = flag.Int("upload-size", 20*1024*1024, "upload size for testing proxies (full mode only)")
	timeout           = flag.Duration("timeout", time.Second*5, "timeout for testing proxies")
	concurrent        = flag.Int("concurrent", 4, "download concurrent size")
	outputPath        = flag.String("output", "", "output config file path")
	gistToken         = flag.String("gist-token", "", "github gist token for updating output")
	gistAddress       = flag.String("gist-address", "", "github gist address or id for updating output (filename uses output basename)")
	repoToken         = flag.String("repo-token", "", "github token for updating repository file")
	repoAddress       = flag.String("repo-address", "", "github repository address or owner/repo for updating output")
	repoFilePath      = flag.String("repo-file-path", "", "repository file path for uploading output (default: output basename)")
	repoBranch        = flag.String("repo-branch", "", "repository branch for uploading output (default: repository default branch)")
	maxLatency        = flag.Duration("max-latency", time.Second, "filter latency greater than this value")
	maxPacketLoss     = flag.Float64("max-packet-loss", 100, "filter packet loss greater than this value(unit: %)")
	minDownloadSpeed  = flag.Float64("min-download-speed", 5, "filter download speed less than this value(unit: MB/s)")
	minUploadSpeed    = flag.Float64("min-upload-speed", 2, "filter upload speed less than this value(unit: MB/s, full mode only)")
	earlyStop         = flag.Int("early-stop", 0, "stop testing after this many results pass filters (0 disables)")
	renameNodes       = flag.Bool("rename", true, "rename nodes with IP location and speed")
	renameTemplate    = flag.String("rename-template", "", "name template for renaming (Go text/template). Placeholders: {{.Flag}}, {{.CountryCode}}, {{.Index}}, {{.Direction}}, {{.Speed}}, {{.SpeedUnit}}, {{.LatencyMs}}, {{.DownloadSpeedMBps}}, {{.UploadSpeedMBps}}, {{.Source}}, {{.JitterMs}}, {{.PacketLossPct}}, {{.Origin}}, {{.Unlock}}. Empty = default format")
	fastMode          = flag.Bool("fast", false, "fast mode (alias for --speed-mode fast)")
	versionFlag       = flag.Bool("v", false, "show version information")
	userAgent         = flag.String("ua", "", "User-Agent for fetching config from http(s) URL (default: mihomo kernel UA, e.g. mihomo/1.10.0)")
	geoipDB           = flag.String("geoip-db", "", "GeoIP MMDB database path (default: ~/.config/clash-speedtest/country.mmdb)")
	updateGeoIP       = flag.Bool("update-geoip", false, "download/update GeoIP database and exit")
	unlockCheck       = flag.Bool("unlock", false, "检测 AI 服务解锁状态（OpenAI/Claude/Codex）")
	unlockTimeout     = flag.Duration("unlock-timeout", 30*time.Second, "解锁检测超时")
	latencyRetries    = flag.Int("latency-retries", 2, "延迟测试轮数，每轮 6 次 ping")
)

func main() {
	flag.Parse()
	mihomolog.SetLevel(mihomolog.SILENT)

	// Handle version flag
	if *versionFlag {
		fmt.Printf("clash-speedtest version %s (commit %s)\n", version, commit)
		os.Exit(0)
	}

	// Handle GeoIP update flag
	if *updateGeoIP {
		if *geoipDB != "" {
			ip.SetGeoIPPath(*geoipDB)
		}
		if err := ip.UpdateGeoIP(); err != nil {
			log.Fatalf("更新 GeoIP 数据库失败: %s", err)
		}
		os.Exit(0)
	}

	// Set GeoIP database path
	if *geoipDB != "" {
		ip.SetGeoIPPath(*geoipDB)
	}

	if *configPathsConfig == "" {
		log.Fatalln("please specify the configuration file")
	}

	var err error
	requestedMode := speedtester.SpeedModeFast
	if !*fastMode {
		requestedMode, err = speedtester.ParseSpeedMode(*speedMode)
		if err != nil {
			log.Fatalf("parse speed mode failed: %s", err)
		}
	}

	speedTester, err := speedtester.New(&speedtester.Config{
		ConfigPaths:      *configPathsConfig,
		FilterRegex:      *filterRegexConfig,
		BlockRegex:       *blockKeywords,
		ServerURL:        *serverURL,
		DownloadSize:     *downloadSize,
		UploadSize:       *uploadSize,
		Timeout:          *timeout,
		Concurrent:       *concurrent,
		MaxPacketLoss:    *maxPacketLoss,
		MaxLatency:       *maxLatency,
		MinDownloadSpeed: *minDownloadSpeed * 1024 * 1024,
		MinUploadSpeed:   *minUploadSpeed * 1024 * 1024,
		Mode:             requestedMode,
		OutputPath:       *outputPath,
		UserAgent:        *userAgent,
		LatencyRetries:   *latencyRetries,
		PostTest:         buildPostTestCallback(*unlockCheck, *unlockTimeout),
	})
	if err != nil {
		log.Fatalf("create speed tester failed: %s", err)
	}
	effectiveMode := speedTester.Mode()
	resultFilter := newResultFilter(effectiveMode)
	stopper, err := newEarlyStopper(*earlyStop, resultFilter)
	if err != nil {
		log.Fatalf("create early stopper failed: %s", err)
	}

	allProxies, err := speedTester.LoadProxies()
	if err != nil {
		log.Fatalf("load proxies failed: %s", err)
	}

	outputMode := output.DetermineOutputMode(output.IsTerminalFile)

	var tsvWriter *output.TSVWriter
	if outputMode == output.OutputModeTSV {
		var err error
		tsvWriter, err = output.NewTSVWriter(os.Stdout, effectiveMode)
		if err != nil {
			log.Fatalf("create TSV writer failed: %s", err)
		}
	}

	results := make([]*speedtester.Result, 0, len(allProxies))

	if outputMode == output.OutputModeInteractive {
		collectResults := *outputPath != ""
		// Run TUI for Interactive mode
		resultChannel := make(chan *speedtester.Result, len(allProxies))
		resultsDone := make(chan struct{})

		// Start testing in goroutine to send results to channel
		go func() {
			speedTester.TestProxiesUntil(allProxies, func(result *speedtester.Result) bool {
				if collectResults {
					results = append(results, result)
				}
				resultChannel <- result
				return stopper.ShouldContinue(result)
			})
			close(resultChannel)
			close(resultsDone)
		}()

		var saveCallback func() tui.SaveResult
		if collectResults {
			saveCallback = func() tui.SaveResult {
				<-resultsDone
				results = output.SortResults(results, effectiveMode)
				out, err := saveConfig(results, resultFilter)
				sr := tui.SaveResult{
					YamlPath: out.yamlPath,
					CsvPath:  out.csvPath,
					GistOK:   out.gistOK,
					GistErr:  out.gistErr,
					RepoOK:   out.repoOK,
					RepoErr:  out.repoErr,
				}
				if err != nil {
					sr.GistErr = err.Error()
				}
				return sr
			}
		}

		// Create and run TUI
		p := tea.NewProgram(
			tui.NewTUIModel(effectiveMode, len(allProxies), resultChannel, saveCallback),
			tea.WithAltScreen(),
			tea.WithMouseAllMotion(),
		)
		if _, err := p.Run(); err != nil {
			log.Fatalf("TUI failed: %s", err)
		}
		return
	}

	// TSV mode: collect results synchronously
	speedTester.TestProxiesUntil(allProxies, func(result *speedtester.Result) bool {
		results = append(results, result)

		if tsvWriter != nil {
			if err := tsvWriter.WriteRow(result, len(results)-1); err != nil {
				log.Printf("write TSV row failed: %s", err)
			}
		}
		return stopper.ShouldContinue(result)
	})

	results = output.SortResults(results, effectiveMode)

	if *outputPath != "" {
		out, err := saveConfig(results, resultFilter)
		if err != nil {
			log.Fatalf("save config file failed: %s", err)
		}
		fmt.Printf("\n配置已保存到: %s\n", out.yamlPath)
	}
}

type saveOutcome struct {
	yamlPath string
	csvPath  string
	gistOK   bool
	gistErr  string
	repoOK   bool
	repoErr  string
}

func saveConfig(results []*speedtester.Result, filter resultFilter) (saveOutcome, error) {
	proxies := make([]map[string]any, 0)
	nameCount := make(map[string]int) // Track name usage to avoid duplicates

	for _, result := range results {
		if !filter.Match(result) {
			continue
		}

		proxyConfig := result.ProxyConfig
		if proxyConfig["name"] == nil || proxyConfig["server"] == nil {
			continue
		}
		if *renameNodes {
			var location *ip.IPLocation
			if result.Proxy != nil {
				location, _ = ip.GetIPLocationViaProxy(result.Proxy)
			}
			if location == nil || location.CountryCode == "" {
				proxies = append(proxies, proxyConfig)
				continue
			}
			originName, _ := proxyConfig["name"].(string)
			renameParams := ip.RenameParams{
				Source:         result.Source,
				OriginName:     originName,
				CountryCode:    location.CountryCode,
				Latency:        result.Latency,
				Jitter:         result.Jitter,
				PacketLoss:     result.PacketLoss,
				DownloadSpeed:  result.DownloadSpeed,
				UploadSpeed:    result.UploadSpeed,
				UnlockServices: result.UnlockServices,
			}
			name, err := ip.GenerateNodeNameFromTemplate(*renameTemplate, renameParams, nameCount)
			if err != nil {
				log.Printf("重命名模板解析错误: %s，使用默认名称", err)
				name = ip.GenerateNodeName(renameParams, nameCount)
			}
			proxyConfig["name"] = name
		}
		proxies = append(proxies, proxyConfig)
	}

	out := saveOutcome{
		yamlPath: *outputPath,
		csvPath:  output.CSVPathFromYAML(*outputPath),
	}

	yamlData, err := marshalConfigNoWrap(proxies)
	if err != nil {
		return out, err
	}

	if err := os.WriteFile(*outputPath, yamlData, 0o644); err != nil {
		return out, err
	}
	outputFilename := filepath.Base(filepath.Clean(*outputPath))

	if err := output.WriteCSV(out.csvPath, results); err != nil {
		log.Printf("写入 CSV 失败: %s", err)
	}

	if *gistToken != "" && *gistAddress != "" {
		uploader := gist.NewUploader(nil)
		files := map[string][]byte{outputFilename: yamlData}
		if csvData, err := os.ReadFile(out.csvPath); err == nil {
			files[filepath.Base(out.csvPath)] = csvData
		}
		if err := uploader.UpdateFiles(*gistToken, *gistAddress, files); err != nil {
			out.gistErr = err.Error()
			log.Printf("更新 gist 失败: %s", err)
		} else {
			out.gistOK = true
		}
	}

	if *repoToken != "" && *repoAddress != "" {
		uploader := gist.NewUploader(nil)
		repositoryFilePath := strings.TrimSpace(*repoFilePath)
		if repositoryFilePath == "" {
			repositoryFilePath = outputFilename
		}
		if err := uploader.UpdateRepoFile(*repoToken, *repoAddress, repositoryFilePath, *repoBranch, yamlData); err != nil {
			out.repoErr = err.Error()
			log.Printf("更新 repo 文件失败: %s", err)
		} else {
			out.repoOK = true
		}
	}

	return out, nil
}

func buildPostTestCallback(enabled bool, timeout time.Duration) func(result *speedtester.Result) {
	if !enabled {
		return nil
	}
	return func(result *speedtester.Result) {
		if result.Proxy == nil {
			return
		}
		result.UnlockServices = unlock.CheckServices(result.Proxy, timeout)
	}
}

// marshalConfigNoWrap 避免 yaml.v2 长字符串折行。
// 策略：用短占位符替换长字符串值，marshal 后用 yamlDoubleQuote 还原为单行引号格式。
func marshalConfigNoWrap(proxies []map[string]any) ([]byte, error) {
	type phEntry struct {
		placeholder string
		origValue   string
	}
	var phEntries []phEntry
	phCounter := 0

	converted := make([]map[string]any, len(proxies))
	for i, proxy := range proxies {
		m := make(map[string]any, len(proxy))
		for k, v := range proxy {
			if s, ok := v.(string); ok && len(s) > 60 {
				ph := fmt.Sprintf("XPLACEHOLDER%dX", phCounter)
				phCounter++
				phEntries = append(phEntries, phEntry{ph, s})
				m[k] = ph
			} else {
				m[k] = v
			}
		}
		converted[i] = m
	}

	config := &speedtester.RawConfig{Proxies: converted}
	data, err := yaml.Marshal(config)
	if err != nil {
		return nil, err
	}

	result := string(data)
	for _, entry := range phEntries {
		quoted := yamlDoubleQuote(entry.origValue)
		result = strings.Replace(result, entry.placeholder, quoted, 1)
	}
	return []byte(result), nil
}

// yamlDoubleQuote 将字符串编码为 YAML 双引号格式（单行，不折行）。
// 与 yaml.v2 风格一致：仅转义 emoji（>U+FFFF）和控制字符，其余 Unicode 原样输出。
func yamlDoubleQuote(s string) string {
	var buf strings.Builder
	buf.Grow(len(s) + 2)
	buf.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			buf.WriteString(`\\`)
		case '"':
			buf.WriteString(`\"`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			if r > 0xFFFF {
				// emoji 等超出 BMP 的字符转义为 \U 序列
				fmt.Fprintf(&buf, `\U%08X`, r)
			} else if r >= 0x20 && r != 0x7F {
				// 可打印 Unicode 原样输出（包括 CJK、全角等）
				buf.WriteRune(r)
			} else {
				// 控制字符转义
				fmt.Fprintf(&buf, `\x%02x`, r)
			}
		}
	}
	buf.WriteByte('"')
	return buf.String()
}
