package main

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestMarshalConfigNoWrap(t *testing.T) {
	proxies := []map[string]any{
		{"name": "bitz｜🇨🇳｜TW｜001｜⚡｜210.00｜ms｜210｜0.00｜0.00｜257｜5.6｜🇹🇼 台湾-广东专线 NeaRoute｜｜｜｜", "type": "trojan"},
		{"name": "bitz｜🇺🇸｜US｜001｜⚡｜258.00｜ms｜258｜0.00｜0.00｜41｜22.2｜🇺🇸 美国-广东专线 BGP 2｜｜｜｜", "type": "trojan"},
		{"name": "short", "type": "trojan"},
	}

	data, err := marshalConfigNoWrap(proxies)
	if err != nil {
		t.Fatalf("marshalConfigNoWrap failed: %v", err)
	}

	output := string(data)

	// 每个 name 应该在单独一行上，不应该有折行
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 跳过空行、proxies: 行、非 name 的键值行
		if trimmed == "" || trimmed == "proxies:" || !strings.HasPrefix(trimmed, "name:") {
			continue
		}
		// name 行应该包含闭合引号（即值在一行内结束）
		if strings.HasPrefix(trimmed, `name: "`) && !strings.HasSuffix(trimmed, `"`) {
			t.Errorf("name value wrapped across lines: %s", line)
		}
	}

	// Round-trip 验证
	type config struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	var parsed config
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("round-trip unmarshal failed: %v", err)
	}

	expected := []string{
		"bitz｜🇨🇳｜TW｜001｜⚡｜210.00｜ms｜210｜0.00｜0.00｜257｜5.6｜🇹🇼 台湾-广东专线 NeaRoute｜｜｜｜",
		"bitz｜🇺🇸｜US｜001｜⚡｜258.00｜ms｜258｜0.00｜0.00｜41｜22.2｜🇺🇸 美国-广东专线 BGP 2｜｜｜｜",
		"short",
	}
	for i, p := range parsed.Proxies {
		name, ok := p["name"].(string)
		if !ok {
			t.Errorf("proxy %d: name is not a string: %v", i, p["name"])
			continue
		}
		if name != expected[i] {
			t.Errorf("proxy %d: expected %q, got %q", i, expected[i], name)
		}
	}
}

func TestMarshalConfigNoWrapWithSpecialChars(t *testing.T) {
	proxies := []map[string]any{
		{"name": `test with "quotes" and \backslash\ and a very long string that should trigger wrapping behavior in yaml.v2`, "type": "trojan"},
	}

	data, err := marshalConfigNoWrap(proxies)
	if err != nil {
		t.Fatalf("marshalConfigNoWrap failed: %v", err)
	}

	type config struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	var parsed config
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("round-trip unmarshal failed: %v", err)
	}

	expected := `test with "quotes" and \backslash\ and a very long string that should trigger wrapping behavior in yaml.v2`
	if parsed.Proxies[0]["name"] != expected {
		t.Errorf("expected %q, got %q", expected, parsed.Proxies[0]["name"])
	}
}
