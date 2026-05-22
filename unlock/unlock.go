package unlock

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/metacubex/mihomo/constant"
)

type serviceCheck struct {
	Name    string
	URL     string
	Method  string
	Headers map[string]string
	Check   func(statusCode int, body string) bool
}

var services = []serviceCheck{
	{
		Name:   "OpenAI",
		URL:    "https://api.openai.com/v1/models",
		Method: "GET",
		Check: func(statusCode int, body string) bool {
			// 401 表示从该地区可以访问 OpenAI（只是没带 key）
			// 403 + unsupported_country 才是地区封锁
			if statusCode == 403 && strings.Contains(strings.ToLower(body), "unsupported_country") {
				return false
			}
			return statusCode == 401
		},
	},
	{
		Name:   "Claude",
		URL:    "https://api.anthropic.com/v1/messages",
		Method: "POST",
		Headers: map[string]string{
			"x-api-key":       "test",
			"content-type":    "application/json",
			"anthropic-version": "2023-06-01",
		},
		Check: func(statusCode int, body string) bool {
			// 401 表示从该地区可以访问（只是 key 无效）
			// 403 才是地区封锁
			return statusCode == 401
		},
	},
	{
		Name:   "Codex",
		URL:    "https://chatgpt.com/",
		Method: "GET",
		Check: func(statusCode int, body string) bool {
			return statusCode == 200
		},
	},
}

// newProxyClient 创建通过代理发请求的 HTTP 客户端
func newProxyClient(proxy constant.Proxy, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
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
}

// CheckServices 通过代理并发检测 AI 服务解锁状态。
// 返回解锁的服务名称列表（如 ["OpenAI", "Codex"]）。
// 全部超时或失败返回空切片。
func CheckServices(proxy constant.Proxy, timeout time.Duration) []string {
	client := newProxyClient(proxy, timeout)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	type result struct {
		name     string
		unlocked bool
	}

	ch := make(chan result, len(services))

	for _, svc := range services {
		go func(svc serviceCheck) {
			unlocked := checkService(ctx, client, svc)
			select {
			case ch <- result{svc.Name, unlocked}:
			case <-ctx.Done():
			}
		}(svc)
	}

	var unlocked []string
	for range services {
		select {
		case r := <-ch:
			if r.unlocked {
				unlocked = append(unlocked, r.name)
			}
		case <-ctx.Done():
			return unlocked
		}
	}

	return unlocked
}

func checkService(ctx context.Context, client *http.Client, svc serviceCheck) bool {
	method := svc.Method
	if method == "" {
		method = "GET"
	}
	req, err := http.NewRequestWithContext(ctx, method, svc.URL, nil)
	if err != nil {
		log.Printf("[解锁检测] %s 创建请求失败: %s", svc.Name, err)
		return false
	}
	for k, v := range svc.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[解锁检测] %s 请求失败: %s", svc.Name, err)
		return false
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	ok := svc.Check(resp.StatusCode, string(body))
	if !ok {
		log.Printf("[解锁检测] %s 未解锁 (status=%d)", svc.Name, resp.StatusCode)
	}
	return ok
}
