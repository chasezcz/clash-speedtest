package ip

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateNodeName(t *testing.T) {
	nameCount := make(map[string]int)

	// Test first occurrence - no suffix
	name1 := GenerateNodeName(RenameParams{CountryCode: "US", DownloadSpeed: 10 * 1024 * 1024}, nameCount)
	expected1 := "🇺🇸 US 001 | ⬇️ 10.00MB/s"
	if name1 != expected1 {
		t.Errorf("Expected %s, got %s", expected1, name1)
	}

	// Test second occurrence - should have -01 suffix
	name2 := GenerateNodeName(RenameParams{CountryCode: "US", DownloadSpeed: 10 * 1024 * 1024}, nameCount)
	expected2 := "🇺🇸 US 002 | ⬇️ 10.00MB/s"
	if name2 != expected2 {
		t.Errorf("Expected %s, got %s", expected2, name2)
	}

	// Test third occurrence - should have -02 suffix
	name3 := GenerateNodeName(RenameParams{CountryCode: "US", DownloadSpeed: 10 * 1024 * 1024}, nameCount)
	expected3 := "🇺🇸 US 003 | ⬇️ 10.00MB/s"
	if name3 != expected3 {
		t.Errorf("Expected %s, got %s", expected3, name3)
	}

	// Test different country - no suffix
	name4 := GenerateNodeName(RenameParams{CountryCode: "HK", DownloadSpeed: 5 * 1024 * 1024}, nameCount)
	expected4 := "🇭🇰 HK 001 | ⬇️ 5.00MB/s"
	if name4 != expected4 {
		t.Errorf("Expected %s, got %s", expected4, name4)
	}

	// Test different speed - no suffix
	name5 := GenerateNodeName(RenameParams{CountryCode: "US", DownloadSpeed: 15 * 1024 * 1024}, nameCount)
	expected5 := "🇺🇸 US 004 | ⬇️ 15.00MB/s"
	if name5 != expected5 {
		t.Errorf("Expected %s, got %s", expected5, name5)
	}
}

func TestGenerateNodeNameUploadFallback(t *testing.T) {
	nameCount := make(map[string]int)

	name := GenerateNodeName(RenameParams{CountryCode: "JP", UploadSpeed: 8 * 1024 * 1024}, nameCount)
	expected := "🇯🇵 JP 001 | ⬆️ 8.00MB/s"
	if name != expected {
		t.Errorf("Expected %s, got %s", expected, name)
	}
}

func TestGenerateNodeNameUnknownCountry(t *testing.T) {
	nameCount := make(map[string]int)

	name := GenerateNodeName(RenameParams{CountryCode: "XX", DownloadSpeed: 10 * 1024 * 1024}, nameCount)
	expected := "🏳️ XX 001 | ⬇️ 10.00MB/s"
	if name != expected {
		t.Errorf("Expected %s, got %s", expected, name)
	}
}

func TestGenerateNodeNameFromTemplate(t *testing.T) {
	nameCount := make(map[string]int)

	// custom template
	name, err := GenerateNodeNameFromTemplate("{{.CountryCode}}-{{.Index}} {{.Speed}}MB/s", RenameParams{CountryCode: "US", DownloadSpeed: 10 * 1024 * 1024}, nameCount)
	if err != nil {
		t.Fatalf("template error: %v", err)
	}
	expected := "US-001 10.00MB/s"
	if name != expected {
		t.Errorf("Expected %s, got %s", expected, name)
	}

	// empty template uses default
	nameCount2 := make(map[string]int)
	name2, err := GenerateNodeNameFromTemplate("", RenameParams{CountryCode: "HK", DownloadSpeed: 5 * 1024 * 1024}, nameCount2)
	if err != nil {
		t.Fatalf("template error: %v", err)
	}
	expected2 := "🇭🇰 HK 001 | ⬇️ 5.00MB/s"
	if name2 != expected2 {
		t.Errorf("Expected %s, got %s", expected2, name2)
	}

	// invalid template returns error
	_, err = GenerateNodeNameFromTemplate("{{.Invalid", RenameParams{CountryCode: "US"}, make(map[string]int))
	if err == nil {
		t.Error("expected parse error for invalid template")
	}
}

func TestGenerateNodeNameFastModeWithDefaultTemplate(t *testing.T) {
	nameCount := make(map[string]int)

	name := GenerateNodeName(RenameParams{CountryCode: "SG", Latency: 120 * time.Millisecond}, nameCount)
	expected := "🇸🇬 SG 001 | ⚡ 120.00ms"
	if name != expected {
		t.Errorf("Expected %s, got %s", expected, name)
	}
}

func TestGenerateNodeNameFromTemplateFastModeLatencyField(t *testing.T) {
	nameCount := make(map[string]int)

	name, err := GenerateNodeNameFromTemplate("{{.CountryCode}}-{{.Index}} {{.LatencyMs}}ms", RenameParams{CountryCode: "DE", Latency: 86 * time.Millisecond}, nameCount)
	if err != nil {
		t.Fatalf("template error: %v", err)
	}
	expected := "DE-001 86ms"
	if name != expected {
		t.Errorf("Expected %s, got %s", expected, name)
	}
}

func TestGenerateNodeNameWithSourceJitterPacketLoss(t *testing.T) {
	nameCount := make(map[string]int)

	name, err := GenerateNodeNameFromTemplate(
		"{{.Source}} | {{.CountryCode}}-{{.Index}} {{.JitterMs}}ms {{.PacketLossPct}}%",
		RenameParams{
			Source:        "config1",
			CountryCode:   "US",
			Latency:       100 * time.Millisecond,
			Jitter:        15 * time.Millisecond,
			PacketLoss:    2.5,
			DownloadSpeed: 10 * 1024 * 1024,
		}, nameCount,
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

	name := GenerateNodeName(RenameParams{Source: "mysub", CountryCode: "HK", DownloadSpeed: 5 * 1024 * 1024}, nameCount)
	expected := "🇭🇰 HK 001 | ⬇️ 5.00MB/s"
	if name != expected {
		t.Errorf("Expected %q, got %q", expected, name)
	}
}

func TestGenerateNodeNameJitterNA(t *testing.T) {
	nameCount := make(map[string]int)

	name, err := GenerateNodeNameFromTemplate(
		"{{.CountryCode}} {{.JitterMs}}",
		RenameParams{CountryCode: "US", DownloadSpeed: 10 * 1024 * 1024}, nameCount,
	)
	if err != nil {
		t.Fatalf("template error: %v", err)
	}
	if !strings.Contains(name, "N/A") {
		t.Errorf("expected N/A for zero jitter, got %q", name)
	}
}

func TestGenerateNodeNameWithOrigin(t *testing.T) {
	nameCount := make(map[string]int)
	name, err := GenerateNodeNameFromTemplate(
		"{{.Origin}}-{{.CountryCode}}-{{.Index}}",
		RenameParams{Source: "sub1", OriginName: "原始节点名", CountryCode: "JP", Latency: 100 * time.Millisecond}, nameCount,
	)
	if err != nil {
		t.Fatalf("template error: %v", err)
	}
	expected := "原始节点名-JP-001"
	if name != expected {
		t.Errorf("Expected %q, got %q", expected, name)
	}
}

func TestGenerateNodeNameWithUnlock(t *testing.T) {
	nameCount := make(map[string]int)
	name, err := GenerateNodeNameFromTemplate(
		"{{.CountryCode}}-{{.Index}} [{{.Unlock}}]",
		RenameParams{CountryCode: "US", Latency: 100 * time.Millisecond, UnlockServices: []string{"OpenAI", "Claude"}}, nameCount,
	)
	if err != nil {
		t.Fatalf("template error: %v", err)
	}
	expected := "US-001 [OpenAI|Claude]"
	if name != expected {
		t.Errorf("Expected %q, got %q", expected, name)
	}
}

func TestGenerateNodeNameWithUnlockEmpty(t *testing.T) {
	nameCount := make(map[string]int)
	name, err := GenerateNodeNameFromTemplate(
		"{{.CountryCode}}-{{.Index}} [{{.Unlock}}]",
		RenameParams{CountryCode: "US", Latency: 100 * time.Millisecond}, nameCount,
	)
	if err != nil {
		t.Fatalf("template error: %v", err)
	}
	expected := "US-001 []"
	if name != expected {
		t.Errorf("Expected %q, got %q", expected, name)
	}
}
