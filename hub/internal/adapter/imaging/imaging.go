// Package imaging decodes untrusted images and re-encodes them, which drops
// metadata (EXIF, GPS) and anything hidden after the image data.
package imaging

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
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

// Square crops the largest square centered in crop (the whole image when
// crop is empty), scales it down to at most size pixels and encodes it as
// JPEG, or PNG when it has transparency. Animated GIFs stay animated GIFs.
func (Processor) Square(r io.Reader, size int, crop image.Rectangle) (ports.Image, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return ports.Image{}, fmt.Errorf("read image: %w", err)
	}
	if isGIF(data) {
		return squareGIF(data, size, crop)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || !within(cfg.Width, cfg.Height, MaxPixels) {
		return ports.Image{}, unsupported()
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ports.Image{}, unsupported()
	}
	return encodeStill(scaleSquare(src, squareIn(src.Bounds(), crop), size))
}

func within(w, h, maxPixels int) bool {
	return w > 0 && h > 0 && w*h <= maxPixels
}

// squareIn is the largest square centered in crop, or in bounds when crop is
// empty or outside the image. crop is relative to the image's top-left corner.
func squareIn(bounds, crop image.Rectangle) image.Rectangle {
	r := crop.Add(bounds.Min).Intersect(bounds)
	if r.Empty() {
		r = bounds
	}
	side := min(r.Dx(), r.Dy())
	return image.Rect(0, 0, side, side).Add(image.Pt(r.Min.X+(r.Dx()-side)/2, r.Min.Y+(r.Dy()-side)/2))
}

func scaleSquare(src image.Image, region image.Rectangle, size int) *image.RGBA {
	edge := min(region.Dx(), size)
	dst := image.NewRGBA(image.Rect(0, 0, edge, edge))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, region, draw.Src, nil)
	return dst
}

func encodeStill(img *image.RGBA) (ports.Image, error) {
	var out bytes.Buffer
	mime := "image/jpeg"
	var err error
	if img.Opaque() {
		err = jpeg.Encode(&out, img, &jpeg.Options{Quality: jpegQuality})
	} else {
		mime = "image/png"
		err = (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&out, img)
	}
	if err != nil {
		return ports.Image{}, fmt.Errorf("encode image: %w", err)
	}
	edge := img.Bounds().Dx()
	return ports.Image{Data: out.Bytes(), Mime: mime, Width: edge, Height: edge}, nil
}
