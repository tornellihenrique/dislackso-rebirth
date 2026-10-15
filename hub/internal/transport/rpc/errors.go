package rpc

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect/v2"
	"connectrpc.com/connect/v2/connectproto"
	"google.golang.org/protobuf/proto"

	dislacksov1 "github.com/tornellihenrique/dislackso-rebirth/hub/gen/dislackso/v1"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

var codes = []struct {
	kind error
	code connect.Code
}{
	{domain.ErrNotFound, connect.CodeNotFound},
	{domain.ErrForbidden, connect.CodePermissionDenied},
	{domain.ErrConflict, connect.CodeAlreadyExists},
	{domain.ErrRateLimited, connect.CodeResourceExhausted},
	{domain.ErrInvalid, connect.CodeInvalidArgument},
	{domain.ErrUnauthenticated, connect.CodeUnauthenticated},
	{domain.ErrFailedPrecondition, connect.CodeFailedPrecondition},
}

// errorMapper turns service errors into Connect errors. Domain errors keep
// their reason, field violations and retry delay as typed details; anything
// else is logged and answered with a bare Internal error.
type errorMapper struct {
	logger *slog.Logger
}

func (m errorMapper) toConnect(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*connect.Error](err); ok {
		return err
	}
	if errors.Is(err, context.Canceled) {
		return connect.NewError(connect.CodeCanceled, "canceled")
	}
	for _, c := range codes {
		if !errors.Is(err, c.kind) {
			continue
		}
		de, ok := domain.AsError(err)
		if !ok {
			return connect.NewError(c.code, c.kind.Error())
		}
		return withDetails(connect.NewError(c.code, message(de)), de)
	}
	m.logger.ErrorContext(ctx, "rpc failed", slog.Any("error", err))
	return connect.NewError(connect.CodeInternal, "internal error")
}

func message(de *domain.Error) string {
	switch {
	case de.Message != "":
		return de.Message
	case de.Reason != "":
		return de.Reason
	default:
		return de.Kind.Error()
	}
}

func withDetails(ce *connect.Error, de *domain.Error) *connect.Error {
	if de.Reason != "" {
		ce = ce.WithDetail(reasonDetail(de.Reason))
	}
	if len(de.Fields) > 0 {
		v := &dislacksov1.FieldViolations{}
		for _, f := range de.Fields {
			v.Violations = append(v.Violations, &dislacksov1.FieldViolation{Field: f.Field, Reason: f.Reason})
		}
		ce = ce.WithDetail(detail(v))
	}
	if de.RetryAfter > 0 {
		ce = ce.WithDetail(detail(&dislacksov1.RateLimited{RetryAfterMs: de.RetryAfter.Milliseconds()}))
	}
	return ce
}

func reasonDetail(reason string) *connect.ErrorDetail {
	return detail(&dislacksov1.ErrorReason{Reason: reason})
}

// detail encodes a message the hub defines; encoding cannot fail for them,
// and a nil detail is ignored by WithDetail.
func detail(msg proto.Message) *connect.ErrorDetail {
	d, err := connectproto.NewErrorDetail(msg)
	if err != nil {
		return nil
	}
	return d
}
