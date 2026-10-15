// Package files serves uploads and downloads over plain HTTP, since large
// binaries never go through RPC.
package files

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/auth"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/file"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/httpauth"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/transport/wire"
)

// multipartOverhead allows for the multipart framing around the file.
const multipartOverhead = 64 << 10

type Authenticator interface {
	Authenticate(ctx context.Context, raw string) (auth.Principal, error)
}

type Service interface {
	Upload(ctx context.Context, actor domain.Actor, purpose domain.FilePurpose, name string, r io.Reader) (domain.File, error)
	Open(ctx context.Context, actor domain.Actor, id domain.ID) (domain.File, ports.Blob, error)
	OpenInviteIcon(ctx context.Context, code string) (domain.File, ports.Blob, error)
}

type Options struct {
	Files  Service
	Auth   Authenticator
	Logger *slog.Logger
}

func Register(mux *http.ServeMux, o Options) {
	h := handler(o)
	mux.HandleFunc("POST /api/uploads", h.upload)
	mux.HandleFunc("GET /files/{id}/{name}", h.download)
	mux.HandleFunc("GET /api/invites/{code}/icon", h.inviteIcon)
}

type handler Options

func (h handler) actor(r *http.Request) (domain.Actor, error) {
	p, err := h.Auth.Authenticate(r.Context(), httpauth.SessionToken(r))
	if err != nil {
		return domain.Actor{}, err
	}
	return p.Actor, nil
}

// upload takes one multipart part named "file"; ?purpose= says what it is for.
func (h handler) upload(w http.ResponseWriter, r *http.Request) {
	if !httpauth.SameOrigin(r) {
		http.Error(w, "cross-origin request refused", http.StatusForbidden)
		return
	}
	actor, err := h.actor(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, file.MaxImageBytes+multipartOverhead)
	mr, err := r.MultipartReader()
	if err != nil {
		h.fail(w, r, domain.Invalid(domain.FieldViolation{Field: "file", Reason: domain.ReasonFieldRequired}))
		return
	}
	for {
		part, err := mr.NextPart()
		if err != nil {
			if errors.As(err, new(*http.MaxBytesError)) {
				err = &domain.Error{Kind: domain.ErrInvalid, Reason: domain.ReasonFileTooLarge}
			} else {
				err = domain.Invalid(domain.FieldViolation{Field: "file", Reason: domain.ReasonFieldRequired})
			}
			h.fail(w, r, err)
			return
		}
		if part.FormName() != "file" {
			_ = part.Close()
			continue
		}
		f, err := h.Files.Upload(r.Context(), actor, domain.FilePurpose(r.URL.Query().Get("purpose")), part.FileName(), part)
		_ = part.Close()
		if err != nil {
			if errors.As(err, new(*http.MaxBytesError)) {
				err = &domain.Error{Kind: domain.ErrInvalid, Reason: domain.ReasonFileTooLarge}
			}
			h.fail(w, r, err)
			return
		}
		body, err := protojson.Marshal(wire.FileRef(f))
		if err != nil {
			h.fail(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(body) //nolint:gosec // protojson output served as application/json, never HTML
		return
	}
}

func (h handler) download(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actor(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	id, err := domain.ParseID(r.PathValue("id"), "id")
	if err != nil {
		h.fail(w, r, domain.NotFound(domain.ReasonFileNotFound))
		return
	}
	f, blob, err := h.Files.Open(r.Context(), actor, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	defer func() { _ = blob.Close() }()
	// A file ID always names the same bytes, so browsers may keep it.
	serve(w, r, f, blob, "private, max-age=31536000, immutable")
}

func (h handler) inviteIcon(w http.ResponseWriter, r *http.Request) {
	f, blob, err := h.Files.OpenInviteIcon(r.Context(), r.PathValue("code"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	defer func() { _ = blob.Close() }()
	serve(w, r, f, blob, "private, max-age=300")
}

// serve sends a stored file. The sandbox CSP and nosniff keep a file from
// ever running as a page on the instance's origin.
func serve(w http.ResponseWriter, r *http.Request, f domain.File, blob ports.Blob, cacheControl string) {
	h := w.Header()
	h.Set("Content-Type", f.Mime)
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", cacheControl)
	h.Set("ETag", `"`+hex.EncodeToString(f.SHA256)+`"`)
	disposition := "attachment"
	if strings.HasPrefix(f.Mime, "image/") {
		disposition = "inline"
	}
	if f.OriginalName != "" {
		disposition = mime.FormatMediaType(disposition, map[string]string{"filename": f.OriginalName})
	}
	h.Set("Content-Disposition", disposition)
	http.ServeContent(w, r, "", f.CreatedAt, blob)
}

type fieldJSON struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

type errorBody struct {
	Code         string      `json:"code"`
	Reason       string      `json:"reason,omitempty"`
	Fields       []fieldJSON `json:"fields,omitempty"`
	RetryAfterMS int64       `json:"retry_after_ms,omitempty"`
}

var statuses = []struct {
	kind   error
	status int
	code   string
}{
	{domain.ErrNotFound, http.StatusNotFound, "not_found"},
	{domain.ErrForbidden, http.StatusForbidden, "permission_denied"},
	{domain.ErrRateLimited, http.StatusTooManyRequests, "resource_exhausted"},
	{domain.ErrInvalid, http.StatusBadRequest, "invalid_argument"},
	{domain.ErrUnauthenticated, http.StatusUnauthorized, "unauthenticated"},
	{domain.ErrFailedPrecondition, http.StatusPreconditionFailed, "failed_precondition"},
}

// fail answers with a JSON error shaped like the RPC errors, so the client
// shows the same messages.
func (h handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	for _, s := range statuses {
		if !errors.Is(err, s.kind) {
			continue
		}
		body := errorBody{Code: s.code}
		if de, ok := domain.AsError(err); ok {
			body.Reason, body.RetryAfterMS = de.Reason, de.RetryAfter.Milliseconds()
			for _, f := range de.Fields {
				body.Fields = append(body.Fields, fieldJSON{Field: f.Field, Reason: f.Reason})
			}
			if de.Reason == domain.ReasonFileTooLarge {
				s.status = http.StatusRequestEntityTooLarge
			}
		}
		if body.RetryAfterMS > 0 {
			w.Header().Set("Retry-After", strconv.FormatInt((body.RetryAfterMS+999)/1000, 10))
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(s.status)
		_ = json.NewEncoder(w).Encode(body)
		return
	}
	h.Logger.ErrorContext(r.Context(), "file request failed", slog.String("path", r.URL.Path), slog.Any("error", err))
	http.Error(w, "internal error", http.StatusInternalServerError)
}
