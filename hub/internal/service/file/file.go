// Package file stores uploads. Avatars and guild icons are decoded and
// re-encoded on the hub, so no upload is ever served as the client sent it.
package file

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"io"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/service/internal/access"
)

const (
	// ImageSize is the edge of stored avatars and guild icons.
	ImageSize = 256
	// MaxImageBytes caps avatar and icon uploads before decoding.
	MaxImageBytes = 10 << 20

	originalNameMaxLen = 255
)

var limitUploadUser = ports.Limit{Burst: 10, Every: 6 * time.Second}

type Deps struct {
	Files        ports.FileRepo
	Blobs        ports.BlobStore
	Images       ports.ImageProcessor
	Invites      ports.InviteRepo
	Guilds       ports.GuildRepo
	IDs          ports.IDGen
	Clock        ports.Clock
	Limiter      ports.RateLimiter
	Verification access.Verification
}

type Service struct {
	d Deps
}

// Request describes an upload. Name is the client's file name, kept for
// display only; Crop selects the part of the image to keep, in source pixels.
type Request struct {
	Purpose domain.FilePurpose
	Name    string
	Crop    image.Rectangle
}

func New(d Deps) *Service { return &Service{d: d} }

func tooLarge() error {
	return &domain.Error{Kind: domain.ErrInvalid, Reason: domain.ReasonFileTooLarge}
}

// Upload stores an image.
func (s *Service) Upload(ctx context.Context, actor domain.Actor, req Request, r io.Reader) (domain.File, error) {
	if !actor.EmailVerified && s.d.Verification.VerificationRequired(ctx) {
		return domain.File{}, domain.Precondition(domain.ReasonEmailNotVerified)
	}
	if !req.Purpose.Valid() {
		return domain.File{}, domain.Invalid(domain.FieldViolation{Field: "purpose", Reason: domain.ReasonFieldInvalid})
	}
	if ok, retry := s.d.Limiter.Allow("upload:user:"+actor.UserID.String(), limitUploadUser); !ok {
		return domain.File{}, domain.RateLimited(retry)
	}
	raw, err := io.ReadAll(io.LimitReader(r, MaxImageBytes+1))
	if err != nil {
		return domain.File{}, err
	}
	if len(raw) > MaxImageBytes {
		return domain.File{}, tooLarge()
	}
	img, err := s.d.Images.Square(bytes.NewReader(raw), ImageSize, req.Crop)
	if err != nil {
		return domain.File{}, err
	}
	sum := sha256.Sum256(img.Data)
	key := hex.EncodeToString(sum[:])
	if err := s.d.Blobs.Put(ctx, key, bytes.NewReader(img.Data)); err != nil {
		return domain.File{}, err
	}
	f := domain.File{
		ID:           s.d.IDs.NewID(),
		OwnerID:      actor.UserID,
		Purpose:      req.Purpose,
		SHA256:       sum[:],
		Size:         int64(len(img.Data)),
		Mime:         img.Mime,
		OriginalName: cleanName(req.Name),
		Width:        img.Width,
		Height:       img.Height,
		StorageKey:   key,
		CreatedAt:    s.d.Clock.Now(),
	}
	if err := s.d.Files.CreateFile(ctx, f); err != nil {
		return domain.File{}, err
	}
	return f, nil
}

// cleanName keeps the base name, without control characters, within the length limit.
func cleanName(name string) string {
	name = path.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	if name == "." || name == "/" {
		return ""
	}
	for len(name) > originalNameMaxLen {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	return name
}

// Open returns a file for download. Avatars and guild icons are visible to
// every signed-in user, like profile pictures.
func (s *Service) Open(ctx context.Context, _ domain.Actor, id domain.ID) (domain.File, ports.Blob, error) {
	f, err := s.d.Files.FileByID(ctx, id)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.File{}, nil, domain.NotFound(domain.ReasonFileNotFound)
	}
	if err != nil {
		return domain.File{}, nil, err
	}
	return s.open(ctx, f)
}

// OpenInviteIcon returns the icon of the guild an invite leads to, for the
// landing page shown before signing in.
func (s *Service) OpenInviteIcon(ctx context.Context, code string) (domain.File, ports.Blob, error) {
	notFound := domain.NotFound(domain.ReasonFileNotFound)
	inv, err := s.d.Invites.InviteByCode(ctx, domain.NormalizeInviteCode(code))
	if errors.Is(err, domain.ErrNotFound) || (err == nil && !inv.Usable(s.d.Clock.Now())) {
		return domain.File{}, nil, notFound
	}
	if err != nil {
		return domain.File{}, nil, err
	}
	g, err := s.d.Guilds.GuildByID(ctx, inv.GuildID)
	if err != nil {
		return domain.File{}, nil, err
	}
	if g.IconFileID == 0 {
		return domain.File{}, nil, notFound
	}
	f, err := s.d.Files.FileByID(ctx, g.IconFileID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.File{}, nil, notFound
	}
	if err != nil {
		return domain.File{}, nil, err
	}
	return s.open(ctx, f)
}

func (s *Service) open(ctx context.Context, f domain.File) (domain.File, ports.Blob, error) {
	b, err := s.d.Blobs.Open(ctx, f.StorageKey)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.File{}, nil, domain.NotFound(domain.ReasonFileNotFound)
	}
	if err != nil {
		return domain.File{}, nil, err
	}
	return f, b, nil
}
