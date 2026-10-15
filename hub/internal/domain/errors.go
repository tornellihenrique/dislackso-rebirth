package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Error kinds. Services return them wrapped in *Error; the RPC layer maps each
// kind to one Connect code.
var (
	ErrNotFound           = errors.New("not found")
	ErrForbidden          = errors.New("forbidden")
	ErrConflict           = errors.New("conflict")
	ErrRateLimited        = errors.New("rate limited")
	ErrInvalid            = errors.New("invalid")
	ErrUnauthenticated    = errors.New("unauthenticated")
	ErrFailedPrecondition = errors.New("failed precondition")
	// ErrUnavailable: a dependency such as the media server is down; retrying later may work.
	ErrUnavailable = errors.New("unavailable")
)

// Reasons are stable codes the client maps to localized messages.
const (
	ReasonInvalidCredentials   = "invalid_credentials" //nolint:gosec // a reason code, not a credential
	ReasonAccountDisabled      = "account_disabled"
	ReasonSessionExpired       = "session_expired"
	ReasonEmailNotVerified     = "email_not_verified"
	ReasonEmailAlreadyVerified = "email_already_verified"
	ReasonEmailUnavailable     = "email_unavailable"
	ReasonEmailDeliveryFailed  = "email_delivery_failed"
	ReasonTokenInvalid         = "token_invalid"
	ReasonRegistrationClosed   = "registration_closed"
	ReasonSetupRequired        = "setup_required"
	ReasonSetupComplete        = "setup_complete"
	ReasonSetupTokenInvalid    = "setup_token_invalid"
	ReasonNotLocal             = "not_local"
	ReasonNotInstanceAdmin     = "not_instance_admin"
	ReasonCannotTargetSelf     = "cannot_target_self"
	ReasonUserNotFound         = "user_not_found"
	ReasonSessionNotFound      = "session_not_found"
	ReasonInviteNotFound       = "invite_not_found"
	ReasonGuildNotFound        = "guild_not_found"
	ReasonChannelNotFound      = "channel_not_found"
	ReasonMemberNotFound       = "member_not_found"
	ReasonBanNotFound          = "ban_not_found"
	ReasonFileNotFound         = "file_not_found"
	ReasonMissingPermission    = "missing_permission"
	ReasonInvitesDisabled      = "invites_disabled"
	ReasonCannotTargetOwner    = "cannot_target_owner"
	ReasonCannotTargetAdmin    = "cannot_target_admin"
	ReasonOwnerCannotLeave     = "owner_cannot_leave"
	ReasonGuildLimitReached    = "guild_limit_reached"
	ReasonChannelLimitReached  = "channel_limit_reached"
	ReasonGuildFull            = "guild_full"
	ReasonChannelFull          = "channel_full"
	ReasonNotVoiceChannel      = "not_voice_channel"
	ReasonNotInVoice           = "not_in_voice"
	ReasonVoiceUnavailable     = "voice_unavailable"
	ReasonBanned               = "banned"
	ReasonUnsupportedFile      = "unsupported_file"
	ReasonFileTooLarge         = "file_too_large"
	// ReasonImageTooLarge: the image decodes to more pixels or frames than the hub processes.
	ReasonImageTooLarge = "image_too_large"
)

// Field violation reasons.
const (
	ReasonFieldRequired      = "required"
	ReasonFieldTooShort      = "too_short"
	ReasonFieldTooLong       = "too_long"
	ReasonFieldInvalidFormat = "invalid_format"
	ReasonFieldTaken         = "taken"
	ReasonFieldTooCommon     = "too_common"
	ReasonFieldIncorrect     = "incorrect"
	// ReasonFieldInvalid: the value is well-formed but not accepted, e.g. an expired invite code.
	ReasonFieldInvalid = "invalid"
)

type FieldViolation struct {
	Field  string
	Reason string
}

// Error is the error type services return.
type Error struct {
	Kind       error
	Reason     string
	Fields     []FieldViolation
	RetryAfter time.Duration
	// Message is extra detail that is safe to show the caller, such as an
	// SMTP server reply during setup.
	Message string
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString(e.Kind.Error())
	if e.Reason != "" {
		b.WriteString(": ")
		b.WriteString(e.Reason)
	}
	for _, f := range e.Fields {
		fmt.Fprintf(&b, " [%s: %s]", f.Field, f.Reason)
	}
	return b.String()
}

func (e *Error) Unwrap() error { return e.Kind }

func newError(kind error, reason string) *Error { return &Error{Kind: kind, Reason: reason} }

func NotFound(reason string) error        { return newError(ErrNotFound, reason) }
func Forbidden(reason string) error       { return newError(ErrForbidden, reason) }
func Unauthenticated(reason string) error { return newError(ErrUnauthenticated, reason) }
func Precondition(reason string) error    { return newError(ErrFailedPrecondition, reason) }
func Unavailable(reason string) error     { return newError(ErrUnavailable, reason) }

// Invalid reports request fields that break a rule.
func Invalid(fields ...FieldViolation) error {
	return &Error{Kind: ErrInvalid, Fields: fields}
}

// Conflict reports a unique field that is already taken.
func Conflict(field string) error {
	return &Error{Kind: ErrConflict, Fields: []FieldViolation{{Field: field, Reason: ReasonFieldTaken}}}
}

func RateLimited(retryAfter time.Duration) error {
	return &Error{Kind: ErrRateLimited, RetryAfter: retryAfter}
}

// AsError extracts the domain error from err, if any.
func AsError(err error) (*Error, bool) {
	var de *Error
	ok := errors.As(err, &de)
	return de, ok
}
