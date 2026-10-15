package notify

import (
	"slices"
	"strings"
	"testing"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/locale"
)

func TestCatalogCoversEveryLocaleAndKind(t *testing.T) {
	kinds := []Kind{KindVerifyEmail, KindPasswordReset, KindTest}
	for _, loc := range locale.Supported {
		msgs, ok := catalog[loc]
		if !ok {
			t.Errorf("no catalog for supported locale %s", loc)
			continue
		}
		for _, k := range kinds {
			m, ok := msgs[k]
			if !ok || m.Subject == "" || m.Intro == "" {
				t.Errorf("%s/%s is missing or incomplete", loc, k)
			}
		}
	}
	for loc := range catalog {
		if !slices.Contains(locale.Supported, loc) {
			t.Errorf("catalog locale %s is not in locale.Supported", loc)
		}
	}
}

func TestRender(t *testing.T) {
	link := "https://hub.example.com/reset-password?token=abc&x=1"
	m, err := Render(KindPasswordReset, "pt-BR", "ana@example.com", Data{Instance: "Equipe <Dev>", Name: "Ana", Link: link})
	if err != nil {
		t.Fatal(err)
	}
	if m.To != "ana@example.com" || m.Subject != "Redefina sua senha do Equipe <Dev>" {
		t.Errorf("To/Subject = %q/%q", m.To, m.Subject)
	}
	for _, want := range []string{"Olá, Ana,", "Escolher uma nova senha: " + link, "— Equipe <Dev>"} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("text body lacks %q:\n%s", want, m.Text)
		}
	}
	if !strings.Contains(m.HTML, `href="https://hub.example.com/reset-password?token=abc&amp;x=1"`) {
		t.Errorf("html body does not link the escaped URL:\n%s", m.HTML)
	}
	if strings.Contains(m.HTML, "<Dev>") || !strings.Contains(m.HTML, "Equipe &lt;Dev&gt;") {
		t.Error("html body does not escape the instance name")
	}
}

func TestRenderFallsBackToDefaultLocale(t *testing.T) {
	m, err := Render(KindTest, "fr", "a@example.com", Data{Instance: "Team"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Subject != "Test email from Team" {
		t.Errorf("Subject = %q, want the English one", m.Subject)
	}
	if strings.Contains(m.Text, "{") {
		t.Errorf("unfilled placeholder in %q", m.Text)
	}
}
