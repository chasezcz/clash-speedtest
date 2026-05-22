package ip

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/metacubex/mihomo/constant"
	"github.com/oschwald/maxminddb-golang"
)

type IPLocation struct {
	Country     string `json:"country"`
	CountryCode string `json:"countryCode"`
}

var (
	proxyLocationCache sync.Map // map[string]*IPLocation
	mmdbOnce           sync.Once
	mmdbReader         *maxminddb.Reader
	mmdbErr            error
	geoipPath          string
)

const defaultMMDBURL = "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geoip.metadb"

// SetGeoIPPath 设置 GeoIP 数据库路径，必须在首次查询前调用
func SetGeoIPPath(path string) {
	geoipPath = path
}

// geoip2Country MaxMind 格式的国家记录
type geoip2Country struct {
	Country struct {
		IsoCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

// metaCountry MetaCubeX 格式的国家记录
type metaCountry string

// loadMMDB 加载 MMDB 数据库（仅执行一次）
func loadMMDB() {
	if geoipPath == "" {
		home, _ := os.UserHomeDir()
		geoipPath = filepath.Join(home, ".config", "clash-speedtest", "country.mmdb")
	}

	// 不存在则自动下载
	if _, err := os.Stat(geoipPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "GeoIP 数据库不存在，正在下载到 %s ...\n", geoipPath)
		if err := downloadMMDB(geoipPath); err != nil {
			mmdbErr = fmt.Errorf("下载 GeoIP 数据库失败: %w", err)
			return
		}
		fmt.Fprintln(os.Stderr, "GeoIP 数据库下载完成")
	}

	reader, err := maxminddb.Open(geoipPath)
	if err != nil {
		mmdbErr = fmt.Errorf("打开 GeoIP 数据库失败: %w", err)
		return
	}
	mmdbReader = reader
}

// downloadMMDB 从 MetaCubeX 下载 MMDB 数据库
func downloadMMDB(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", defaultMMDBURL, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(tmpPath)
		return err
	}
	f.Close()

	return os.Rename(tmpPath, path)
}

// UpdateGeoIP 更新 GeoIP 数据库
func UpdateGeoIP() error {
	if geoipPath == "" {
		home, _ := os.UserHomeDir()
		geoipPath = filepath.Join(home, ".config", "clash-speedtest", "country.mmdb")
	}

	fmt.Fprintf(os.Stderr, "正在更新 GeoIP 数据库到 %s ...\n", geoipPath)
	if err := downloadMMDB(geoipPath); err != nil {
		return fmt.Errorf("更新 GeoIP 数据库失败: %w", err)
	}
	fmt.Fprintln(os.Stderr, "GeoIP 数据库更新完成")
	return nil
}

// lookupCountryIP 从本地 MMDB 查询 IP 的国家代码
func lookupCountryIP(ip net.IP) (string, error) {
	mmdbOnce.Do(loadMMDB)
	if mmdbErr != nil {
		return "", mmdbErr
	}
	if mmdbReader == nil {
		return "", fmt.Errorf("GeoIP 数据库未加载")
	}

	// 尝试 MaxMind 格式
	var country geoip2Country
	if err := mmdbReader.Lookup(ip, &country); err == nil && country.Country.IsoCode != "" {
		return strings.ToUpper(country.Country.IsoCode), nil
	}

	// 尝试 MetaCubeX 格式（国家代码直接作为值）
	var code string
	if err := mmdbReader.Lookup(ip, &code); err == nil && code != "" {
		return strings.ToUpper(code), nil
	}

	return "", fmt.Errorf("未找到 IP %s 的归属地", ip)
}

// LookupCountry 从本地 MMDB 查询 IP/主机的国家代码
func LookupCountry(host string) (string, error) {
	ip := net.ParseIP(host)
	if ip == nil {
		// 尝试 DNS 解析
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return "", fmt.Errorf("DNS 解析 %s 失败: %w", host, err)
		}
		if len(addrs) == 0 {
			return "", fmt.Errorf("DNS 解析 %s 无结果", host)
		}
		ip = addrs[0].IP
	}
	return lookupCountryIP(ip)
}

// resolveHost 将主机名解析为首个 IP 地址。
// 如果输入已经是 IP 地址，直接返回。
func resolveHost(host string) (string, error) {
	if ip := net.ParseIP(host); ip != nil {
		return host, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return "", fmt.Errorf("DNS 解析 %s 失败: %w", host, err)
	}
	if len(addrs) == 0 {
		return "", fmt.Errorf("DNS 解析 %s 无结果", host)
	}
	return addrs[0].IP.String(), nil
}

// exitIPService 用于发现出口 IP 的轻量服务（只返回 IP，不查归属地）
type exitIPService struct {
	name  string
	url   string
	parse func(body []byte) (string, error) // 返回 IP 字符串
}

var exitIPServices = []exitIPService{
	{
		name: "Cloudflare",
		url:  "https://1.1.1.1/cdn-cgi/trace",
		parse: func(body []byte) (string, error) {
			for _, line := range strings.Split(string(body), "\n") {
				if strings.HasPrefix(line, "ip=") {
					return strings.TrimPrefix(line, "ip="), nil
				}
			}
			return "", fmt.Errorf("未找到 ip 字段")
		},
	},
	{
		name: "ip.sb",
		url:  "https://api.ip.sb/jsonip",
		parse: func(body []byte) (string, error) {
			var resp struct {
				IP string `json:"ip"`
			}
			if err := json.Unmarshal(body, &resp); err != nil {
				return "", err
			}
			return resp.IP, nil
		},
	},
	{
		name: "ipinfo.io",
		url:  "https://ipinfo.io/json",
		parse: func(body []byte) (string, error) {
			var resp struct {
				IP string `json:"ip"`
			}
			if err := json.Unmarshal(body, &resp); err != nil {
				return "", err
			}
			return resp.IP, nil
		},
	},
}

var proxyLocationCache2 sync.Map

// GetIPLocationViaProxy 通过代理查询出口 IP 归属地。
// 通过代理发出轻量 HTTP 请求发现出口 IP，然后用本地 MMDB 查询归属地。
// 结果按代理名称缓存。
func GetIPLocationViaProxy(proxy constant.Proxy) (*IPLocation, error) {
	name := proxy.Name()
	if cached, ok := proxyLocationCache2.Load(name); ok {
		return cached.(*IPLocation), nil
	}

	// 通过代理发现出口 IP
	exitIP, err := discoverExitIP(proxy)
	if err != nil {
		return nil, fmt.Errorf("发现出口 IP 失败 (代理: %s): %w", name, err)
	}

	// 用本地 MMDB 查询归属地
	countryCode, err := LookupCountry(exitIP)
	if err != nil {
		return nil, fmt.Errorf("查询出口 IP 归属地失败 (%s): %w", exitIP, err)
	}

	loc := &IPLocation{CountryCode: countryCode}
	proxyLocationCache2.Store(name, loc)
	return loc, nil
}

// discoverExitIP 通过代理访问轻量服务，发现出口 IP
func discoverExitIP(proxy constant.Proxy) (string, error) {
	client := &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				host, portStr, err := net.SplitHostPort(addr)
				if err != nil {
					return nil, err
				}
				port, _ := strconv.ParseUint(portStr, 10, 16)
				return proxy.DialContext(ctx, &constant.Metadata{
					Host:    host,
					DstPort: uint16(port),
				})
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	type serviceResult struct {
		ip  string
		err error
	}

	ch := make(chan serviceResult, len(exitIPServices))
	done := make(chan struct{})

	for _, svc := range exitIPServices {
		go func(svc exitIPService) {
			ip, err := queryExitIPService(ctx, client, svc)
			select {
			case ch <- serviceResult{ip, err}:
			case <-done:
			}
		}(svc)
	}

	var lastErr error
	for range exitIPServices {
		select {
		case r := <-ch:
			if r.err == nil && r.ip != "" {
				close(done)
				return r.ip, nil
			}
			if r.err != nil {
				lastErr = r.err
			}
		case <-ctx.Done():
			close(done)
			if lastErr != nil {
				return "", lastErr
			}
			return "", ctx.Err()
		}
	}

	close(done)
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("所有出口 IP 发现服务均失败")
}

func queryExitIPService(ctx context.Context, client *http.Client, svc exitIPService) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", svc.url, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s 返回状态码 %d", svc.name, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return svc.parse(body)
}
