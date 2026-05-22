package ip

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"
)

var countryFlags = map[string]string{
	"US": "🇺🇸", "CN": "🇨🇳", "GB": "🇬🇧", "UK": "🇬🇧", "JP": "🇯🇵", "DE": "🇩🇪", "FR": "🇫🇷", "RU": "🇷🇺",
	"SG": "🇸🇬", "HK": "🇭🇰", "TW": "🇨🇳", "KR": "🇰🇷", "CA": "🇨🇦", "AU": "🇦🇺", "NL": "🇳🇱", "IT": "🇮🇹",
	"ES": "🇪🇸", "SE": "🇸🇪", "NO": "🇳🇴", "DK": "🇩🇰", "FI": "🇫🇮", "CH": "🇨🇭", "AT": "🇦🇹", "BE": "🇧🇪",
	"BR": "🇧🇷", "IN": "🇮🇳", "TH": "🇹🇭", "MY": "🇲🇾", "VN": "🇻🇳", "PH": "🇵🇭", "ID": "🇮🇩", "UA": "🇺🇦",
	"TR": "🇹🇷", "IL": "🇮🇱", "AE": "🇦🇪", "SA": "🇸🇦", "EG": "🇪🇬", "ZA": "🇿🇦", "NG": "🇳🇬", "KE": "🇰🇪",
	"RO": "🇷🇴", "PL": "🇵🇱", "CZ": "🇨🇿", "HU": "🇭🇺", "BG": "🇧🇬", "HR": "🇭🇷", "SI": "🇸🇮", "SK": "🇸🇰",
	"LT": "🇱🇹", "LV": "🇱🇻", "EE": "🇪🇪", "PT": "🇵🇹", "GR": "🇬🇷", "IE": "🇮🇪", "LU": "🇱🇺", "MT": "🇲🇹",
	"CY": "🇨🇾", "IS": "🇮🇸", "MX": "🇲🇽", "AR": "🇦🇷", "CL": "🇨🇱", "CO": "🇨🇴", "PE": "🇵🇪", "VE": "🇻🇪",
	"EC": "🇪🇨", "UY": "🇺🇾", "PY": "🇵🇾", "BO": "🇧🇴", "CR": "🇨🇷", "PA": "🇵🇦", "GT": "🇬🇹", "HN": "🇭🇳",
	"SV": "🇸🇻", "NI": "🇳🇮", "BZ": "🇧🇿", "JM": "🇯🇲", "TT": "🇹🇹", "BB": "🇧🇧", "GD": "🇬🇩", "LC": "🇱🇨",
	"VC": "🇻🇨", "AG": "🇦🇬", "DM": "🇩🇲", "KN": "🇰🇳", "BS": "🇧🇸", "CU": "🇨🇺", "DO": "🇩🇴", "HT": "🇭🇹",
	"PR": "🇵🇷", "VI": "🇻🇮", "GU": "🇬🇺", "AS": "🇦🇸", "MP": "🇲🇵", "PW": "🇵🇼", "FM": "🇫🇲", "MH": "🇲🇭",
	"KI": "🇰🇮", "TV": "🇹🇻", "NR": "🇳🇷", "WS": "🇼🇸", "TO": "🇹🇴", "FJ": "🇫🇯", "VU": "🇻🇺", "SB": "🇸🇧",
	"PG": "🇵🇬", "NC": "🇳🇨", "PF": "🇵🇫", "WF": "🇼🇫", "CK": "🇨🇰", "NU": "🇳🇺", "TK": "🇹🇰", "SC": "🇸🇨",
}

// DefaultNameTemplate is the built-in format when -rename-template is not set.
const DefaultNameTemplate = `{{.Flag}} {{.CountryCode}} {{.Index}} | {{.Direction}} {{.Speed}}{{.SpeedUnit}}`

// NodeNameData is the data passed to the rename template.
type NodeNameData struct {
	Flag              string // country flag emoji
	CountryCode       string // e.g. US, HK
	Index             string // padded number, e.g. 001
	Direction         string // ⬇️, ⬆️, or ⚡
	Speed             string // primary metric value
	SpeedUnit         string // MB/s or ms
	LatencyMs         string // latency in milliseconds
	DownloadSpeedMBps string // download MB/s
	UploadSpeedMBps   string // upload MB/s
	Source            string // config source name
	JitterMs          string // jitter in milliseconds
	PacketLossPct     string // packet loss percentage
	Origin            string // original proxy name before rename
	Unlock            string // unlocked AI services, e.g. "OpenAI|Claude"
}

// RenameParams 参数化重命名所需的全部数据。
type RenameParams struct {
	Source         string
	OriginName     string
	CountryCode    string
	Latency        time.Duration
	Jitter         time.Duration
	PacketLoss     float64
	DownloadSpeed  float64
	UploadSpeed    float64
	UnlockServices []string
}

// GenerateNodeNameFromTemplate renders name from a text/template. Placeholders:
// {{.Flag}}, {{.CountryCode}}, {{.Index}}, {{.Direction}}, {{.Speed}}, {{.SpeedUnit}}, {{.LatencyMs}}, {{.DownloadSpeedMBps}}, {{.UploadSpeedMBps}}, {{.Source}}, {{.JitterMs}}, {{.PacketLossPct}}, {{.Origin}}, {{.Unlock}}.
// If template is empty, DefaultNameTemplate is used. On execute error, falls back to default format.
func GenerateNodeNameFromTemplate(tmpl string, params RenameParams, nameCount map[string]int) (string, error) {
	if tmpl == "" {
		tmpl = DefaultNameTemplate
	}
	t, err := template.New("name").Parse(tmpl)
	if err != nil {
		return "", err
	}
	data := buildNodeNameData(params, nameCount)
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		// fallback to default format so caller does not double-increment nameCount
		return fmt.Sprintf("%s %s %s | %s %s%s", data.Flag, data.CountryCode, data.Index, data.Direction, data.Speed, data.SpeedUnit), nil
	}
	return buf.String(), nil
}

func buildNodeNameData(params RenameParams, nameCount map[string]int) NodeNameData {
	flag, exists := countryFlags[strings.ToUpper(params.CountryCode)]
	if !exists {
		flag = "🏳️"
	}
	upperCountryCode := strings.ToUpper(params.CountryCode)
	speed := params.DownloadSpeed
	direction := "⬇️"
	speedUnit := "MB/s"
	if params.DownloadSpeed <= 0 {
		speed = params.UploadSpeed
		direction = "⬆️"
	}
	if params.DownloadSpeed <= 0 && params.UploadSpeed <= 0 && params.Latency > 0 {
		speed = float64(params.Latency.Milliseconds())
		direction = "⚡"
		speedUnit = "ms"
	}
	speedMBps := speed / (1024 * 1024)
	if speedUnit == "ms" {
		speedMBps = speed
	}
	count := nameCount[upperCountryCode] + 1
	nameCount[upperCountryCode] = count
	dlMBps := params.DownloadSpeed / (1024 * 1024)
	ulMBps := params.UploadSpeed / (1024 * 1024)
	latencyMs := "N/A"
	if params.Latency > 0 {
		latencyMs = fmt.Sprintf("%d", params.Latency.Milliseconds())
	}
	jitterMs := "N/A"
	if params.Jitter > 0 {
		jitterMs = fmt.Sprintf("%d", params.Jitter.Milliseconds())
	}
	return NodeNameData{
		Flag:              flag,
		CountryCode:       upperCountryCode,
		Index:             fmt.Sprintf("%03d", count),
		Direction:         direction,
		Speed:             fmt.Sprintf("%.2f", speedMBps),
		SpeedUnit:         speedUnit,
		LatencyMs:         latencyMs,
		DownloadSpeedMBps: fmt.Sprintf("%.2f", dlMBps),
		UploadSpeedMBps:   fmt.Sprintf("%.2f", ulMBps),
		Source:            params.Source,
		JitterMs:          jitterMs,
		PacketLossPct:     fmt.Sprintf("%.1f", params.PacketLoss),
		Origin:            params.OriginName,
		Unlock:            strings.Join(params.UnlockServices, "|"),
	}
}

func GenerateNodeName(params RenameParams, nameCount map[string]int) string {
	name, _ := GenerateNodeNameFromTemplate("", params, nameCount)
	return name
}
