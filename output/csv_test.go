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
			ProxyName:     "node-1",
			ProxyType:     "ss",
			ProxyConfig:   map[string]any{"server": "1.1.1.1", "port": 443},
			Source:        "config1",
			Latency:       100 * time.Millisecond,
			Jitter:        15 * time.Millisecond,
			PacketLoss:    2.5,
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
	expectedHeader := []string{"Source", "Name", "Type", "Server", "Port", "Latency", "Jitter", "PacketLoss", "DownloadSpeed", "UploadSpeed", "Unlock", "DownloadError", "UploadError"}
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
	// Latency: 100ms = 100.00
	if row[5] != "100.00" {
		t.Errorf("Latency = %q, want %q", row[5], "100.00")
	}
	// Jitter: 15ms = 15.00
	if row[6] != "15.00" {
		t.Errorf("Jitter = %q, want %q", row[6], "15.00")
	}
	// PacketLoss: 2.5%
	if row[7] != "2.5" {
		t.Errorf("PacketLoss = %q, want %q", row[7], "2.5")
	}
	// DownloadSpeed: 10MB/s
	if row[8] != "10.00" {
		t.Errorf("DownloadSpeed = %q, want %q", row[8], "10.00")
	}
	// UploadSpeed: 5MB/s
	if row[9] != "5.00" {
		t.Errorf("UploadSpeed = %q, want %q", row[9], "5.00")
	}
}

func TestWriteCSVEmpty(t *testing.T) {
	tmpFile := t.TempDir() + "/empty.csv"
	if err := WriteCSV(tmpFile, nil); err != nil {
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
	if len(records) != 1 {
		t.Fatalf("expected 1 row (header only), got %d", len(records))
	}
}

func TestCSVPathFromYAML(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/tmp/output.yaml", "/tmp/output.csv"},
		{"/tmp/output.yml", "/tmp/output.csv"},
		{"output.yaml", "output.csv"},
	}
	for _, tt := range tests {
		got := CSVPathFromYAML(tt.input)
		if got != tt.want {
			t.Errorf("CSVPathFromYAML(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
