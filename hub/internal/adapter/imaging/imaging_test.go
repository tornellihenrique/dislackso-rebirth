package imaging

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

func solid(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	return img
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestSquare(t *testing.T) {
	var gifBuf, jpegBuf bytes.Buffer
	if err := gif.Encode(&gifBuf, solid(40, 30, color.White), nil); err != nil {
		t.Fatal(err)
	}
	// Trailing bytes after the image (a classic polyglot trick) must not survive.
	if err := jpeg.Encode(&jpegBuf, solid(800, 400, color.RGBA{200, 10, 10, 255}), nil); err != nil {
		t.Fatal(err)
	}
	jpegBuf.WriteString("<script>alert(1)</script>")

	tests := []struct {
		name     string
		in       []byte
		wantMime string
		wantEdge int
	}{
		{"wide jpeg is cropped and scaled", jpegBuf.Bytes(), "image/jpeg", 256},
		{"transparent png stays png", encodePNG(t, solid(300, 500, color.RGBA{0, 0, 0, 0})), "image/png", 256},
		{"small image is not enlarged", encodePNG(t, solid(64, 80, color.White)), "image/jpeg", 64},
		{"still gif", gifBuf.Bytes(), "image/jpeg", 30},
	}
	for _, tt := range tests {
		img, err := Processor{}.Square(bytes.NewReader(tt.in), 256, image.Rectangle{})
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if img.Mime != tt.wantMime || img.Width != tt.wantEdge || img.Height != tt.wantEdge {
			t.Errorf("%s: got %s %dx%d", tt.name, img.Mime, img.Width, img.Height)
		}
		if bytes.Contains(img.Data, []byte("<script>")) {
			t.Errorf("%s: trailing data survived", tt.name)
		}
		if _, _, err := image.Decode(bytes.NewReader(img.Data)); err != nil {
			t.Errorf("%s: output does not decode: %v", tt.name, err)
		}
	}
}

func TestSquareRefusesBadInput(t *testing.T) {
	// A PNG header declaring 100000x100000 pixels: refused before decoding.
	huge := encodePNG(t, solid(1, 1, color.White))
	huge[16], huge[17], huge[18], huge[19] = 0, 1, 0x86, 0xa0
	huge[20], huge[21], huge[22], huge[23] = 0, 1, 0x86, 0xa0

	for name, in := range map[string][]byte{
		"text":  []byte("hello, not an image"),
		"svg":   []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`),
		"empty": nil,
		"huge":  huge,
	} {
		_, err := Processor{}.Square(bytes.NewReader(in), 256, image.Rectangle{})
		if !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}

func TestSquareCrop(t *testing.T) {
	src := solid(200, 100, red)
	for y := range 100 {
		for x := 100; x < 200; x++ {
			src.Set(x, y, blue)
		}
	}
	tests := []struct {
		name string
		crop image.Rectangle
		want color.RGBA
	}{
		{"right half", image.Rect(100, 0, 200, 100), blue},
		{"left half", image.Rect(0, 0, 100, 100), red},
		{"a crop outside the image falls back to the center", image.Rect(500, 500, 600, 600), red},
	}
	for _, tt := range tests {
		img, err := Processor{}.Square(bytes.NewReader(encodePNG(t, src)), 256, tt.crop)
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if img.Width != 100 {
			t.Errorf("%s: edge %d", tt.name, img.Width)
		}
		out, _, err := image.Decode(bytes.NewReader(img.Data))
		if err != nil {
			t.Fatal(err)
		}
		if got := rgbaAt(out, 25, 50); !near(got, tt.want, 8) {
			t.Errorf("%s: pixel = %v, want %v", tt.name, got, tt.want)
		}
	}
}
