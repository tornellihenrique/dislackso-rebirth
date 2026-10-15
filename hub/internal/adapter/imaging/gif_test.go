package imaging

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
)

var (
	red   = color.RGBA{255, 0, 0, 255}
	green = color.RGBA{0, 255, 0, 255}
	blue  = color.RGBA{0, 0, 255, 255}
)

// paletted is a w×h frame at (x, y) filled with c, with a transparent entry.
func paletted(x, y, w, h int, c color.RGBA) *image.Paletted {
	m := image.NewPaletted(image.Rect(x, y, x+w, y+h), color.Palette{color.RGBA{}, c})
	for i := range m.Pix {
		m.Pix[i] = 1
	}
	return m
}

func encodeGIF(t *testing.T, g *gif.GIF) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, g); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func rgbaAt(m image.Image, x, y int) color.RGBA {
	c, _ := color.RGBAModel.Convert(m.At(x, y)).(color.RGBA)
	return c
}

func near(a, b color.RGBA, tolerance int) bool {
	d := func(x, y uint8) bool { return max(int(x)-int(y), int(y)-int(x)) <= tolerance }
	return d(a.R, b.R) && d(a.G, b.G) && d(a.B, b.B) && d(a.A, b.A)
}

// play renders every frame of an animation the way browsers do.
func play(t *testing.T, data []byte) (*gif.GIF, []*image.RGBA) {
	t.Helper()
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	canvas := image.NewRGBA(image.Rect(0, 0, g.Config.Width, g.Config.Height))
	shown := make([]*image.RGBA, 0, len(g.Image))
	for i, m := range g.Image {
		b := m.Bounds()
		var saved *image.RGBA
		if g.Disposal[i] == gif.DisposalPrevious {
			saved = image.NewRGBA(b)
			draw.Draw(saved, b, canvas, b.Min, draw.Src)
		}
		over(canvas, m)
		frame := image.NewRGBA(canvas.Bounds())
		copy(frame.Pix, canvas.Pix)
		shown = append(shown, frame)
		switch g.Disposal[i] {
		case gif.DisposalBackground:
			draw.Draw(canvas, b, image.Transparent, image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			draw.Draw(canvas, b, saved, b.Min, draw.Src)
		}
	}
	return g, shown
}

func TestSquareAnimatedGIF(t *testing.T) {
	in := encodeGIF(t, &gif.GIF{
		Image: []*image.Paletted{
			paletted(0, 0, 300, 300, red),
			// Undone before the next frame.
			paletted(100, 100, 100, 100, blue),
			paletted(0, 0, 10, 10, green),
			// Changes nothing.
			paletted(0, 0, 10, 10, green),
		},
		Delay:    []int{10, 20, 30, 40},
		Disposal: []byte{gif.DisposalNone, gif.DisposalPrevious, gif.DisposalNone, gif.DisposalNone},
	})
	img, err := Processor{}.Square(bytes.NewReader(in), 150, image.Rectangle{})
	if err != nil {
		t.Fatal(err)
	}
	if img.Mime != "image/gif" || img.Width != 150 || img.Height != 150 {
		t.Fatalf("got %s %dx%d", img.Mime, img.Width, img.Height)
	}
	g, frames := play(t, img.Data)
	if diff := cmp.Diff([]int{10, 20, 30, 40}, g.Delay); diff != "" {
		t.Errorf("delays (-want +got):\n%s", diff)
	}
	if g.LoopCount != 0 {
		t.Errorf("loop count = %d, want 0 (forever)", g.LoopCount)
	}
	checks := []struct {
		frame, x, y int
		want        color.RGBA
	}{
		{0, 75, 75, red},
		{1, 75, 75, blue},
		{1, 10, 10, red},
		{2, 75, 75, red},
		{2, 2, 2, green},
		{3, 2, 2, green},
		{3, 75, 75, red},
	}
	for _, c := range checks {
		if got := rgbaAt(frames[c.frame], c.x, c.y); !near(got, c.want, 8) {
			t.Errorf("frame %d at %d,%d = %v, want %v", c.frame, c.x, c.y, got, c.want)
		}
	}
	if b := g.Image[3].Bounds(); b.Dx()*b.Dy() != 1 {
		t.Errorf("unchanged frame is %v, want a single pixel", b)
	}
}

func TestSquareTransparentGIF(t *testing.T) {
	in := encodeGIF(t, &gif.GIF{
		Image:     []*image.Paletted{paletted(0, 0, 40, 40, red), paletted(20, 20, 20, 20, blue)},
		Delay:     []int{5, 5},
		Disposal:  []byte{gif.DisposalBackground, gif.DisposalBackground},
		Config:    image.Config{Width: 40, Height: 40},
		LoopCount: -1,
	})
	img, err := Processor{}.Square(bytes.NewReader(in), 256, image.Rectangle{})
	if err != nil {
		t.Fatal(err)
	}
	g, frames := play(t, img.Data)
	if g.LoopCount != -1 {
		t.Errorf("loop count = %d, want -1 (play once)", g.LoopCount)
	}
	if got := rgbaAt(frames[1], 5, 5); got.A != 0 {
		t.Errorf("cleared area = %v, want transparent", got)
	}
	if got := rgbaAt(frames[1], 30, 30); !near(got, blue, 8) {
		t.Errorf("second frame = %v, want blue", got)
	}
}

func TestSquareTolerantGIF(t *testing.T) {
	anim := &gif.GIF{
		Image: []*image.Paletted{paletted(0, 0, 20, 20, red), paletted(0, 0, 20, 20, blue), paletted(0, 0, 20, 20, green)},
		Delay: []int{1, 1, 1},
	}
	// The logical screen says 10×10 while the frames are 20×20.
	oversized := encodeGIF(t, anim)
	oversized[6], oversized[8] = 10, 10
	if _, err := gif.DecodeAll(bytes.NewReader(oversized)); err == nil {
		t.Fatal("image/gif accepts the oversized frame, so this case tests nothing")
	}
	img, err := Processor{}.Square(bytes.NewReader(oversized), 256, image.Rectangle{})
	if err != nil || img.Width != 20 {
		t.Fatalf("oversized frames: %v, edge %d", err, img.Width)
	}

	// Cut inside the last frame.
	full := encodeGIF(t, anim)
	img, err = Processor{}.Square(bytes.NewReader(full[:len(full)-8]), 256, image.Rectangle{})
	if err != nil {
		t.Fatal(err)
	}
	if g, _ := play(t, img.Data); len(g.Image) != 2 {
		t.Errorf("truncated: %d frames, want 2", len(g.Image))
	}
}

func TestSquareRefusesHugeGIF(t *testing.T) {
	long := &gif.GIF{}
	for range maxGIFFrames + 1 {
		long.Image = append(long.Image, paletted(0, 0, 1, 1, red))
		long.Delay = append(long.Delay, 1)
	}
	wide := encodeGIF(t, &gif.GIF{Image: []*image.Paletted{paletted(0, 0, 1, 1, red)}, Delay: []int{1}})
	// A logical screen of 4000×4000.
	wide[6], wide[7], wide[8], wide[9] = 0xa0, 0x0f, 0xa0, 0x0f

	for name, in := range map[string][]byte{"frames": encodeGIF(t, long), "screen": wide} {
		_, err := Processor{}.Square(bytes.NewReader(in), 256, image.Rectangle{})
		de, ok := domain.AsError(err)
		if !errors.Is(err, domain.ErrInvalid) || !ok || len(de.Fields) != 1 || de.Fields[0].Reason != domain.ReasonImageTooLarge {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if _, err := (Processor{}).Square(bytes.NewReader([]byte("GIF89a\x01\x00")), 256, image.Rectangle{}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("header only: err = %v", err)
	}
}

func TestQuantize(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 256, 256))
	for y := range 256 {
		for x := range 256 {
			src.Set(x, y, color.RGBA{uint8(x), uint8(y), uint8(255 - x), 255})
		}
	}
	src.Set(0, 0, color.RGBA{})
	out := quantize(src)
	if len(out.Palette) > 256 {
		t.Fatalf("palette has %d colors", len(out.Palette))
	}
	if out.Pix[0] != transparentIndex {
		t.Errorf("transparent pixel mapped to %d", out.Pix[0])
	}
	abs := func(v int) int { return max(v, -v) }
	total := 0
	for y := range 256 {
		for x := range 256 {
			if x == 0 && y == 0 {
				continue
			}
			a, b := rgbaAt(src, x, y), rgbaAt(out, x, y)
			total += abs(int(a.R)-int(b.R)) + abs(int(a.G)-int(b.G)) + abs(int(a.B)-int(b.B))
		}
	}
	if mean := float64(total) / (256*256 - 1) / 3; mean > 6 {
		t.Errorf("mean channel error %.1f", mean)
	}

	few := solid(4, 4, red)
	few.Set(1, 1, blue)
	if got := quantize(few); len(got.Palette) != 3 || rgbaAt(got, 1, 1) != blue || rgbaAt(got, 0, 0) != red {
		t.Errorf("exact palette = %v", got.Palette)
	}
}
