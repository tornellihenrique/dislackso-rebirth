// Package imaging decodes untrusted images and re-encodes them, which drops
// metadata (EXIF, GPS) and anything hidden after the image data.
package imaging

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif" // registers the GIF decoder; the first frame is used
	"image/jpeg"
	"image/png"
	"io"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // registers the WebP decoder

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

// MaxPixels refuses images whose decoded size would use too much memory
// (a small file can declare huge dimensions).
const MaxPixels = 40_000_000

const jpegQuality = 90

type Processor struct{}

func unsupported() error {
	return domain.Invalid(domain.FieldViolation{Field: "file", Reason: domain.ReasonUnsupportedFile})
}

// Square center-crops the image to a square, scales it down to at most
// size pixels and encodes it as JPEG, or PNG when it has transparency.
func (Processor) Square(r io.Reader, size int) (ports.Image, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return ports.Image{}, fmt.Errorf("read image: %w", err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > MaxPixels {
		return ports.Image{}, unsupported()
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ports.Image{}, unsupported()
	}

	b := src.Bounds()
	side := min(b.Dx(), b.Dy())
	crop := image.Rect(0, 0, side, side).Add(image.Pt(b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2))
	edge := min(side, size)
	dst := image.NewRGBA(image.Rect(0, 0, edge, edge))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, crop, draw.Src, nil)

	var out bytes.Buffer
	mime := "image/jpeg"
	if dst.Opaque() {
		err = jpeg.Encode(&out, dst, &jpeg.Options{Quality: jpegQuality})
	} else {
		mime = "image/png"
		err = (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&out, dst)
	}
	if err != nil {
		return ports.Image{}, fmt.Errorf("encode image: %w", err)
	}
	return ports.Image{Data: out.Bytes(), Mime: mime, Width: edge, Height: edge}, nil
}
