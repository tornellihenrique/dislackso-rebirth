package locale

import "testing"

func TestMatch(t *testing.T) {
	tests := []struct {
		header string
		want   string
	}{
		{"", "en"},
		{"pt-BR,pt;q=0.9,en;q=0.8", "pt-BR"},
		{"pt-PT", "pt-BR"},
		{"pt", "pt-BR"},
		{"en-GB,pt-BR;q=0.5", "en"},
		{"fr-FR", "en"},
		{"fr-FR,pt;q=0.5", "pt-BR"},
		{"not a header;;;", "en"},
	}
	for _, tt := range tests {
		if got := Match(tt.header); got != tt.want {
			t.Errorf("Match(%q) = %q, want %q", tt.header, got, tt.want)
		}
	}
}
