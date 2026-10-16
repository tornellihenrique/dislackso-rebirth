package auth

import (
	"testing"
	"time"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/events"
)

func TestRequestEmailChangeChecks(t *testing.T) {
	h := newHarness(t)
	signed := h.registerAna(t)
	actor := ActorFor(signed.User, signed.Session)
	if _, err := h.svc.Register(t.Context(), client, RegisterInput{Username: "bia", Email: "bia@example.com", Password: "correct horse"}); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, email, password, field, reason string
		kind                                 error
	}{
		{"wrong password", "new@example.com", "wrong", "password", domain.ReasonFieldIncorrect, domain.ErrInvalid},
		{"same address", "ANA@example.com", "correct horse", "new_email", domain.ReasonFieldUnchanged, domain.ErrInvalid},
		{"taken address", "bia@example.com", "correct horse", "new_email", domain.ReasonFieldTaken, domain.ErrConflict},
		{"not an address", "nope", "correct horse", "new_email", domain.ReasonFieldInvalidFormat, domain.ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := h.svc.RequestEmailChange(t.Context(), client, actor, tt.email, tt.password)
			requireField(t, err, tt.kind, tt.field, tt.reason)
		})
	}
}

func TestEmailChangeIsConfirmedByLink(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	signed := h.registerAna(t)
	actor := ActorFor(signed.User, signed.Session)
	verify, _ := h.emails.Last("verify")

	res, err := h.svc.RequestEmailChange(ctx, client, actor, " New@Example.com ", "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Sent || res.User.Email != "ana@example.com" {
		t.Fatalf("result = %+v, want a confirmation sent and the old address kept", res)
	}
	sent, ok := h.emails.Last("change_email")
	if !ok || sent.To != "New@Example.com" {
		t.Fatalf("confirmation = %+v, want one to the new address", sent)
	}
	h.events.Take()

	if err := h.svc.ConfirmEmailChange(ctx, client, tokenFromLink(t, sent.Link)); err != nil {
		t.Fatal(err)
	}
	u, _ := h.store.UserByID(ctx, signed.User.ID)
	if u.Email != "New@Example.com" || !u.EmailVerified() {
		t.Errorf("user = %s verified=%v, want the new address, verified", u.Email, u.EmailVerified())
	}
	evs := h.events.Take()
	if len(evs) != 1 || evs[0].Topics[0] != events.UserTopic(u.ID) {
		t.Errorf("events = %+v, want MeUpdated to the user", evs)
	}
	err = h.svc.ConfirmEmailChange(ctx, client, tokenFromLink(t, sent.Link))
	requireDomainErr(t, err, domain.ErrNotFound, domain.ReasonTokenInvalid)
	err = h.svc.VerifyEmail(ctx, client, tokenFromLink(t, verify.Link))
	requireDomainErr(t, err, domain.ErrNotFound, domain.ReasonTokenInvalid)
}

func TestEmailChangeWithoutEmailAppliesAtOnce(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	h.mailer.Off = true
	signed := h.registerAna(t)
	sentBefore := len(h.emails.Sent)

	res, err := h.svc.RequestEmailChange(ctx, client, ActorFor(signed.User, signed.Session), "new@example.com", "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if res.Sent || res.User.Email != "new@example.com" || res.User.EmailVerified() {
		t.Errorf("result = %+v, want the new address in place, unverified", res)
	}
	if len(h.emails.Sent) != sentBefore {
		t.Error("an email was sent without email configured")
	}
	if _, err := h.svc.Login(ctx, client, "new@example.com", "correct horse"); err != nil {
		t.Errorf("login with the new address: %v", err)
	}
}

func TestConfirmEmailChangeFailsWhenTheAddressWasTaken(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	signed := h.registerAna(t)
	if _, err := h.svc.RequestEmailChange(ctx, client, ActorFor(signed.User, signed.Session), "new@example.com", "correct horse"); err != nil {
		t.Fatal(err)
	}
	sent, _ := h.emails.Last("change_email")
	if _, err := h.svc.Register(ctx, client, RegisterInput{Username: "bia", Email: "new@example.com", Password: "correct horse"}); err != nil {
		t.Fatal(err)
	}
	err := h.svc.ConfirmEmailChange(ctx, client, tokenFromLink(t, sent.Link))
	requireField(t, err, domain.ErrConflict, "new_email", domain.ReasonFieldTaken)
}

func TestEmailChangeLinkExpires(t *testing.T) {
	h := newHarness(t)
	ctx := t.Context()
	signed := h.registerAna(t)
	if _, err := h.svc.RequestEmailChange(ctx, client, ActorFor(signed.User, signed.Session), "new@example.com", "correct horse"); err != nil {
		t.Fatal(err)
	}
	sent, _ := h.emails.Last("change_email")
	h.clock.Advance(changeEmailTokenTTL + time.Second)
	err := h.svc.ConfirmEmailChange(ctx, client, tokenFromLink(t, sent.Link))
	requireDomainErr(t, err, domain.ErrNotFound, domain.ReasonTokenInvalid)
}
