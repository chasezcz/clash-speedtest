package output

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/faceair/clash-speedtest/speedtester"
)

var csvHeader = []string{"Source", "Name", "Type", "Server", "Port", "Latency", "Jitter", "PacketLoss", "DownloadSpeed", "UploadSpeed", "Unlock", "DownloadError", "UploadError"}

// WriteCSV writes test results as CSV to the given path.
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

		name := r.ProxyName
		if renamed, ok := r.ProxyConfig["name"].(string); ok && renamed != "" {
			name = renamed
		}

		row := []string{
			r.Source,
			name,
			r.ProxyType,
			server,
			port,
			latencyMs,
			jitterMs,
			fmt.Sprintf("%.1f", r.PacketLoss),
			fmt.Sprintf("%.2f", r.DownloadSpeed/(1024*1024)),
			fmt.Sprintf("%.2f", r.UploadSpeed/(1024*1024)),
			strings.Join(r.UnlockServices, "|"),
			r.DownloadError,
			r.UploadError,
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// CSVPathFromYAML returns the CSV path derived from a YAML output path.
func CSVPathFromYAML(yamlPath string) string {
	ext := filepath.Ext(yamlPath)
	return strings.TrimSuffix(yamlPath, ext) + ".csv"
}
