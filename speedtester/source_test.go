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
