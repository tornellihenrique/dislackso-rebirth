package imaging

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"slices"

	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/domain"
	"github.com/tornellihenrique/dislackso-rebirth/hub/internal/ports"
)

// A GIF is taken apart block by block and each frame is decoded on its own
// rather than with gif.DecodeAll: frame count and sizes are checked before any
// pixel is decoded, memory stays at one frame plus the canvas, and files that
// browsers show but image/gif refuses (a frame reaching past the logical
// screen, a broken last frame) still work.

const (
	maxGIFScreen = 4_000_000
	maxGIFFrames = 300
	// maxGIFWork bounds canvas area times frame count, which is what
	// composing and scaling every frame costs.
	maxGIFWork = 100_000_000
	// maxAnimatedEdge keeps animations small: every frame costs edge² bytes
	// or so, and 160 px still covers the largest avatar at 2× pixel density.
	maxAnimatedEdge = 160
)

const (
	gifExtension   = 0x21
	gifImage       = 0x2c
	gifTrailer     = 0x3b
	gifControl     = 0xf9
	gifApplication = 0xff
)

type gifFrame struct {
	control  []byte // graphic control extension, if any
	block    []byte // image descriptor, local color table and image data
	bounds   image.Rectangle
	delay    int
	disposal byte
}

type gifFile struct {
	head      []byte // header, logical screen descriptor and global color table
	width     int
	height    int
	frames    []gifFrame
	loopCount int
}

func isGIF(data []byte) bool {
	return bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a"))
}

func imageTooLarge() error {
	return domain.Invalid(domain.FieldViolation{Field: "file", Reason: domain.ReasonImageTooLarge})
}

func squareGIF(data []byte, size int, crop image.Rectangle) (ports.Image, error) {
	f, err := parseGIF(data)
	if err != nil {
		return ports.Image{}, err
	}
	w, h := f.width, f.height
	for _, fr := range f.frames {
		w, h = max(w, fr.bounds.Max.X), max(h, fr.bounds.Max.Y)
	}
	if w > 0xffff || h > 0xffff || !within(w, h, maxGIFScreen) || w*h*len(f.frames) > maxGIFWork {
		return ports.Image{}, imageTooLarge()
	}
	// Frames are decoded against a screen grown to hold all of them, as browsers do.
	head := slices.Clone(f.head)
	binary.LittleEndian.PutUint16(head[6:], uint16(w)) //nolint:gosec // at most 0xffff, checked above
	binary.LittleEndian.PutUint16(head[8:], uint16(h)) //nolint:gosec // at most 0xffff, checked above

	if len(f.frames) > 1 {
		size = min(size, maxAnimatedEdge)
	}
	canvas := image.NewRGBA(image.Rect(0, 0, w, h))
	region := squareIn(canvas.Bounds(), crop)
	anim := &gif.GIF{LoopCount: f.loopCount}
	var last *image.RGBA
	for i, fr := range f.frames {
		frame, err := decodeFrame(head, fr)
		if err != nil {
			if i == 0 {
				return ports.Image{}, unsupported()
			}
			// Browsers play the frames before a broken one.
			break
		}
		b := frame.Bounds()
		var saved *image.RGBA
		if fr.disposal == gif.DisposalPrevious {
			saved = image.NewRGBA(b)
			draw.Draw(saved, b, canvas, b.Min, draw.Src)
		}
		over(canvas, frame)
		last = scaleSquare(canvas, region, size)
		anim.Image = append(anim.Image, quantize(last))
		anim.Delay = append(anim.Delay, fr.delay)
		switch fr.disposal {
		case gif.DisposalBackground:
			// Browsers clear to transparent, not to the background color.
			draw.Draw(canvas, b, image.Transparent, image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			draw.Draw(canvas, b, saved, b.Min, draw.Src)
		}
	}
	if len(anim.Image) == 1 {
		return encodeStill(last)
	}
	return encodeAnimation(anim)
}

type scanner struct {
	data []byte
	pos  int
}

func (s *scanner) take(n int) ([]byte, bool) {
	if n > len(s.data)-s.pos {
		return nil, false
	}
	b := s.data[s.pos : s.pos+n]
	s.pos += n
	return b, true
}

func (s *scanner) next() (byte, bool) {
	b, ok := s.take(1)
	if !ok {
		return 0, false
	}
	return b[0], true
}

func (s *scanner) skipSubBlocks() bool {
	for {
		n, ok := s.next()
		if !ok {
			return false
		}
		if n == 0 {
			return true
		}
		if _, ok := s.take(int(n)); !ok {
			return false
		}
	}
}

func colorTableSize(flags byte) int {
	if flags&0x80 == 0 {
		return 0
	}
	return 3 << (flags&7 + 1)
}

func le16(b []byte) int { return int(binary.LittleEndian.Uint16(b)) }

// parseGIF finds the frames. A file cut short or followed by garbage keeps
// the frames before that point, like browsers do.
func parseGIF(data []byte) (gifFile, error) {
	s := &scanner{data: data}
	lsd, ok := s.take(13)
	if !ok {
		return gifFile{}, unsupported()
	}
	if _, ok := s.take(colorTableSize(lsd[10])); !ok {
		return gifFile{}, unsupported()
	}
	f := gifFile{head: data[:s.pos], width: le16(lsd[6:]), height: le16(lsd[8:]), loopCount: -1}
	var control []byte
scan:
	for {
		start := s.pos
		c, ok := s.next()
		if !ok {
			break
		}
		switch c {
		case gifExtension:
			label, ok := s.next()
			if !ok {
				break scan
			}
			switch label {
			case gifControl:
				b, ok := s.take(6)
				if !ok || b[0] != 4 {
					break scan
				}
				control = data[start:s.pos]
			case gifApplication:
				if !f.application(s) {
					break scan
				}
			default:
				if !s.skipSubBlocks() {
					break scan
				}
			}
		case gifImage:
			desc, ok := s.take(9)
			if !ok {
				break scan
			}
			if _, ok := s.take(colorTableSize(desc[8])); !ok {
				break scan
			}
			if _, ok := s.next(); !ok { // LZW minimum code size
				break scan
			}
			if !s.skipSubBlocks() {
				break scan
			}
			x, y := le16(desc[0:]), le16(desc[2:])
			fr := gifFrame{
				control: control,
				block:   data[start:s.pos],
				bounds:  image.Rect(x, y, x+le16(desc[4:]), y+le16(desc[6:])),
			}
			if control != nil {
				fr.disposal = control[3] >> 2 & 7
				fr.delay = le16(control[4:])
			}
			if len(f.frames) == maxGIFFrames {
				return gifFile{}, imageTooLarge()
			}
			f.frames = append(f.frames, fr)
			control = nil
		default:
			break scan
		}
	}
	if len(f.frames) == 0 {
		return gifFile{}, unsupported()
	}
	return f, nil
}

// application reads an application extension, keeping the loop count of
// the NETSCAPE looping extension.
func (f *gifFile) application(s *scanner) bool {
	n, ok := s.next()
	if !ok {
		return false
	}
	if n == 0 {
		return true
	}
	id, ok := s.take(int(n))
	if !ok {
		return false
	}
	if string(id) == "NETSCAPE2.0" || string(id) == "ANIMEXTS1.0" {
		n, ok := s.next()
		if !ok {
			return false
		}
		if n == 0 {
			return true
		}
		b, ok := s.take(int(n))
		if !ok {
			return false
		}
		if len(b) == 3 && b[0] == 1 {
			f.loopCount = le16(b[1:])
		}
	}
	return s.skipSubBlocks()
}

func decodeFrame(head []byte, fr gifFrame) (*image.Paletted, error) {
	buf := make([]byte, 0, len(head)+len(fr.control)+len(fr.block)+1)
	buf = append(append(append(append(buf, head...), fr.control...), fr.block...), gifTrailer)
	img, err := gif.Decode(bytes.NewReader(buf))
	if err != nil {
		return nil, fmt.Errorf("decode gif frame: %w", err)
	}
	p, ok := img.(*image.Paletted)
	if !ok {
		return nil, errors.New("decode gif frame: not paletted")
	}
	return p, nil
}

// over draws the opaque pixels of a frame onto the canvas. GIF transparency
// is all or nothing, so the canvas never holds partial alpha.
func over(canvas *image.RGBA, frame *image.Paletted) {
	var colors [256][4]uint8
	var opaque [256]bool
	for i, c := range frame.Palette {
		r, g, b, a := c.RGBA()
		if a == 0 {
			continue
		}
		colors[i] = [4]uint8{uint8(r >> 8 & 0xff), uint8(g >> 8 & 0xff), uint8(b >> 8 & 0xff), 0xff}
		opaque[i] = true
	}
	b := frame.Bounds().Intersect(canvas.Bounds())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		src := frame.Pix[frame.PixOffset(b.Min.X, y):]
		dst := canvas.Pix[canvas.PixOffset(b.Min.X, y):]
		for x := range b.Dx() {
			if i := src[x]; opaque[i] {
				copy(dst[4*x:4*x+4], colors[i][:])
			}
		}
	}
}

// encodeAnimation draws each frame only where it differs from the one
// before. An animation with transparent pixels keeps whole frames, each
// clearing the last, since a frame cannot make a drawn pixel transparent
// again.
func encodeAnimation(anim *gif.GIF) (ports.Image, error) {
	edge := anim.Image[0].Bounds().Dx()
	anim.Config = image.Config{Width: edge, Height: edge}
	disposal := byte(gif.DisposalNone)
	if slices.ContainsFunc(anim.Image, func(m *image.Paletted) bool { return slices.Contains(m.Pix, transparentIndex) }) {
		disposal = gif.DisposalBackground
	} else {
		// Back to front, so each frame is compared with the previous one as it was drawn.
		for i := len(anim.Image) - 1; i > 0; i-- {
			anim.Image[i] = changedPart(anim.Image[i-1], anim.Image[i])
		}
	}
	anim.Disposal = slices.Repeat([]byte{disposal}, len(anim.Image))
	var out bytes.Buffer
	if err := gif.EncodeAll(&out, anim); err != nil {
		return ports.Image{}, fmt.Errorf("encode gif: %w", err)
	}
	return ports.Image{Data: out.Bytes(), Mime: "image/gif", Width: edge, Height: edge}, nil
}

// changedPart turns cur into the smallest rectangle that differs from prev,
// with unchanged pixels inside it made transparent so prev shows through.
func changedPart(prev, cur *image.Paletted) *image.Paletted {
	prevColors, curColors := paletteRGBA(prev.Palette), paletteRGBA(cur.Palette)
	changed := image.Rectangle{}
	b := cur.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			i := cur.PixOffset(x, y)
			if curColors[cur.Pix[i]] == prevColors[prev.Pix[prev.PixOffset(x, y)]] {
				cur.Pix[i] = transparentIndex
				continue
			}
			changed = changed.Union(image.Rect(x, y, x+1, y+1))
		}
	}
	if changed.Empty() {
		changed = image.Rect(0, 0, 1, 1)
	}
	sub, _ := cur.SubImage(changed).(*image.Paletted)
	return sub
}

func paletteRGBA(p color.Palette) [256]color.RGBA {
	var out [256]color.RGBA
	for i, c := range p {
		out[i], _ = color.RGBAModel.Convert(c).(color.RGBA)
	}
	return out
}
