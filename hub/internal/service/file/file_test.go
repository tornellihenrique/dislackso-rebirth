package file

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/fakes"
)

type harness struct {
	svc     *Service
	store   *fakes.Store
	blobs   *fakes.Blobs
	clock   *fakes.Clock
	limiter *fakes.Limiter
}

var ana = domain.Actor{UserID: 1, EmailVerified: true}

func newHarness() *harness {
	h := &harness{store: fakes.NewStore(), blobs: &fakes.Blobs{}, clock: fakes.NewClock(), limiter: &fakes.Limiter{}}
	h.svc = New(Deps{
		Files: h.store, Blobs: h.blobs, Images: fakes.Images{}, Invites: h.store, Guilds: h.store,
		IDs: &fakes.IDs{}, Clock: h.clock, Limiter: h.limiter, Verification: fakes.Verification(true),
	})
	return h
}

func TestUploadAndOpen(t *testing.T) {
	h := newHarness()
	ctx := t.Context()

	f, err := h.svc.Upload(ctx, ana, domain.FileAvatar, `C:\Users\ana\me.jpg`, strings.NewReader("img:face"))
	if err != nil {
		t.Fatal(err)
	}
	if f.OwnerID != 1 || f.Purpose != domain.FileAvatar || f.Mime != "image/png" || f.Width != ImageSize ||
		f.OriginalName != "me.jpg" || len(f.StorageKey) != 64 || f.Size != int64(len("png:img:face")) {
		t.Errorf("file = %+v", f)
	}
	// The same picture uploaded twice is stored once.
	if _, err := h.svc.Upload(ctx, ana, domain.FileAvatar, "again.png", strings.NewReader("img:face")); err != nil || h.blobs.Puts != 1 {
		t.Errorf("second upload: puts = %d, err = %v", h.blobs.Puts, err)
	}

	got, blob, err := h.svc.Open(ctx, domain.Actor{UserID: 2}, f.ID)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(blob)
	if got.ID != f.ID || string(data) != "png:img:face" {
		t.Errorf("opened %+v: %q", got, data)
	}
	if _, _, err := h.svc.Open(ctx, ana, 999); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown file: %v", err)
	}
}

func TestUploadRefusals(t *testing.T) {
	h := newHarness()
	ctx := t.Context()
	tests := []struct {
		name    string
		actor   domain.Actor
		purpose domain.FilePurpose
		body    io.Reader
		kind    error
		reason  string
	}{
		{"not an image", ana, domain.FileAvatar, strings.NewReader("<svg/>"), domain.ErrInvalid, domain.ReasonUnsupportedFile},
		{"unknown purpose", ana, "attachment", strings.NewReader("img:x"), domain.ErrInvalid, domain.ReasonFieldInvalid},
		{"too large", ana, domain.FileGuildIcon, bytes.NewReader(append([]byte("img:"), make([]byte, MaxImageBytes)...)), domain.ErrInvalid, domain.ReasonFileTooLarge},
		{"unverified", domain.Actor{UserID: 1}, domain.FileAvatar, strings.NewReader("img:x"), domain.ErrFailedPrecondition, domain.ReasonEmailNotVerified},
	}
	for _, tt := range tests {
		_, err := h.svc.Upload(ctx, tt.actor, tt.purpose, "x.png", tt.body)
		de, ok := domain.AsError(err)
		if !errors.Is(err, tt.kind) || !ok || (de.Reason != tt.reason && (len(de.Fields) == 0 || de.Fields[0].Reason != tt.reason)) {
			t.Errorf("%s: %v", tt.name, err)
		}
	}
	h.limiter.Deny = map[string]time.Duration{"upload:user:1": time.Second}
	if _, err := h.svc.Upload(ctx, ana, domain.FileAvatar, "x.png", strings.NewReader("img:x")); !errors.Is(err, domain.ErrRateLimited) {
		t.Errorf("rate limit: %v", err)
	}
}

func TestOpenInviteIcon(t *testing.T) {
	h := newHarness()
	ctx := t.Context()
	icon, err := h.svc.Upload(ctx, ana, domain.FileGuildIcon, "icon.png", strings.NewReader("img:logo"))
	if err != nil {
		t.Fatal(err)
	}
	_ = h.store.CreateGuild(ctx, domain.Guild{ID: 100, Name: "Team", IconFileID: icon.ID, OwnerID: 1})
	_ = h.store.CreateGuild(ctx, domain.Guild{ID: 101, Name: "Plain", OwnerID: 1})
	expires := h.clock.Now().Add(time.Minute)
	_ = h.store.CreateInvite(ctx, domain.Invite{Code: "WITHICON", GuildID: 100, ExpiresAt: &expires})
	_ = h.store.CreateInvite(ctx, domain.Invite{Code: "NOICON22", GuildID: 101})

	f, blob, err := h.svc.OpenInviteIcon(ctx, "withicon")
	if err != nil || f.ID != icon.ID {
		t.Fatalf("icon: %+v, %v", f, err)
	}
	_ = blob.Close()
	for _, code := range []string{"NOICON22", "UNKNOWN2"} {
		if _, _, err := h.svc.OpenInviteIcon(ctx, code); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("%s: %v", code, err)
		}
	}
	h.clock.Advance(time.Hour)
	if _, _, err := h.svc.OpenInviteIcon(ctx, "WITHICON"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expired invite still shows the icon: %v", err)
	}
}

func TestCleanName(t *testing.T) {
	for in, want := range map[string]string{
		"photo.png":              "photo.png",
		"../../etc/passwd":       "passwd",
		`C:\fakepath\me.jpg`:     "me.jpg",
		"a\x00b\nc.png":          "abc.png",
		"":                       "",
		strings.Repeat("é", 200): strings.Repeat("é", 127),
		"dir/":                   "dir",
	} {
		if got := cleanName(in); got != want {
			t.Errorf("cleanName(%q) = %q, want %q", in, got, want)
		}
	}
}
