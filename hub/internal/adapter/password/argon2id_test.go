package password

import (
	"strings"
	"testing"
)

var fastParams = Params{MemoryKiB: 64, Iterations: 1, Parallelism: 1, SaltLen: 16, KeyLen: 32}

func TestHashAndVerify(t *testing.T) {
	h := NewArgon2id(fastParams, 2)
	encoded, err := h.Hash("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=64,t=1,p=1$") {
		t.Errorf("encoded = %q, want the PHC argon2id format", encoded)
	}

	tests := []struct {
		password string
		want     bool
	}{
		{"correct horse", true},
		{"correct horse ", false},
		{"Correct horse", false},
		{"", false},
	}
	for _, tt := range tests {
		ok, err := h.Verify(encoded, tt.password)
		if err != nil {
			t.Fatalf("Verify(%q): %v", tt.password, err)
		}
		if ok != tt.want {
			t.Errorf("Verify(%q) = %v, want %v", tt.password, ok, tt.want)
		}
	}

	again, _ := h.Hash("correct horse")
	if again == encoded {
		t.Error("two hashes of the same password share a salt")
	}
}

func TestVerifyUsesStoredParams(t *testing.T) {
	old := NewArgon2id(fastParams, 1)
	encoded, err := old.Hash("pw-12345678")
	if err != nil {
		t.Fatal(err)
	}
	stronger := fastParams
	stronger.Iterations = 2
	if ok, err := NewArgon2id(stronger, 1).Verify(encoded, "pw-12345678"); err != nil || !ok {
		t.Errorf("hash made with older params no longer verifies: ok=%v err=%v", ok, err)
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	h := NewArgon2id(fastParams, 1)
	for _, encoded := range []string{
		"",
		"plain",
		"$argon2i$v=19$m=64,t=1,p=1$c2FsdA$a2V5",
		"$argon2id$v=18$m=64,t=1,p=1$c2FsdA$a2V5",
		"$argon2id$v=19$m=x,t=1,p=1$c2FsdA$a2V5",
		"$argon2id$v=19$m=64,t=1,p=1$!!$a2V5",
	} {
		if _, err := h.Verify(encoded, "pw"); err == nil {
			t.Errorf("Verify(%q) accepted a malformed hash", encoded)
		}
	}
}
