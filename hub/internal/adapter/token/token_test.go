package token

import (
	"testing"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

func TestTokens(t *testing.T) {
	a, err := Random{}.NewToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Random{}.NewToken()
	if len(a) != 43 || a == b {
		t.Errorf("tokens %q and %q: want 43 distinct characters", a, b)
	}
}

func TestInviteCodes(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		code, err := Random{}.NewInviteCode()
		if err != nil {
			t.Fatal(err)
		}
		if !domain.IsGuildInviteCode(code) {
			t.Fatalf("%q is not a valid invite code", code)
		}
		if seen[code] {
			t.Fatalf("duplicate code %q", code)
		}
		seen[code] = true
	}
}
