package imaging

import (
	"cmp"
	"image"
	"image/color"
	"slices"
)

// transparentIndex is the palette entry quantize keeps for transparent pixels.
const transparentIndex = 0

const (
	paletteColors = 255
	// transparentPixel marks a transparent pixel among 0xRRGGBB values.
	transparentPixel = 1 << 24
)

// quantize maps img to at most 255 colors, exactly when it has no more and by
// median cut otherwise, plus a transparent entry at transparentIndex. Pixels
// at least half opaque count as opaque.
func quantize(img *image.RGBA) *image.Paletted {
	b := img.Bounds()
	pixels := make([]uint32, 0, b.Dx()*b.Dy())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := img.Pix[img.PixOffset(b.Min.X, y):]
		for x := range b.Dx() {
			p := row[4*x : 4*x+4]
			a := uint32(p[3])
			if a < 0x80 {
				pixels = append(pixels, transparentPixel)
				continue
			}
			// RGBA is premultiplied.
			r, g, bl := min(uint32(p[0])*0xff/a, 0xff), min(uint32(p[1])*0xff/a, 0xff), min(uint32(p[2])*0xff/a, 0xff)
			pixels = append(pixels, r<<16|g<<8|bl)
		}
	}
	palette, index := exactPalette(pixels)
	if palette == nil {
		palette, index = medianCut(pixels, paletteColors)
	}
	out := image.NewPaletted(b, palette)
	for i, p := range pixels {
		if p != transparentPixel {
			out.Pix[i] = index(p)
		}
	}
	return out
}

func rgb(p uint32) color.RGBA {
	return color.RGBA{R: uint8(p >> 16 & 0xff), G: uint8(p >> 8 & 0xff), B: uint8(p & 0xff), A: 0xff}
}

// exactPalette returns nil when the pixels have more than paletteColors colors.
func exactPalette(pixels []uint32) (color.Palette, func(uint32) uint8) {
	indexes := map[uint32]uint8{}
	palette := color.Palette{color.RGBA{}}
	for _, p := range pixels {
		if _, ok := indexes[p]; ok || p == transparentPixel {
			continue
		}
		if len(palette) > paletteColors {
			return nil, nil
		}
		indexes[p] = uint8(len(palette)) //nolint:gosec // at most paletteColors
		palette = append(palette, rgb(p))
	}
	return palette, func(p uint32) uint8 { return indexes[p] }
}

// cell is one 15-bit color of the histogram median cut works on.
type cell struct {
	key   uint16
	rgb   [3]uint8 // 5-bit channels
	count int
	sum   [3]int // full-precision channel sums, for the box averages
}

func cellKey(p uint32) uint16 {
	return uint16(p>>19&0x1f)<<10 | uint16(p>>11&0x1f)<<5 | uint16(p>>3&0x1f)
}

type box struct {
	cells []cell
	count int
	// channel is the one with the widest spread of values.
	channel int
	spread  int
}

func newBox(cells []cell) box {
	b := box{cells: cells}
	lo, hi := [3]uint8{0xff, 0xff, 0xff}, [3]uint8{}
	for _, c := range cells {
		b.count += c.count
		for ch := range 3 {
			lo[ch], hi[ch] = min(lo[ch], c.rgb[ch]), max(hi[ch], c.rgb[ch])
		}
	}
	for ch := range 3 {
		if spread := int(hi[ch]) - int(lo[ch]); spread > b.spread {
			b.channel, b.spread = ch, spread
		}
	}
	return b
}

// split cuts the box at the weighted median of its widest channel.
func (b box) split() (box, box) {
	slices.SortFunc(b.cells, func(x, y cell) int { return cmp.Compare(x.rgb[b.channel], y.rgb[b.channel]) })
	seen, at := 0, 1
	for i, c := range b.cells[:len(b.cells)-1] {
		seen += c.count
		at = i + 1
		if seen*2 >= b.count {
			break
		}
	}
	return newBox(b.cells[:at]), newBox(b.cells[at:])
}

func medianCut(pixels []uint32, colors int) (color.Palette, func(uint32) uint8) {
	var hist [1 << 15]cell
	for _, p := range pixels {
		if p == transparentPixel {
			continue
		}
		c := &hist[cellKey(p)]
		c.count++
		c.sum[0] += int(p >> 16 & 0xff)
		c.sum[1] += int(p >> 8 & 0xff)
		c.sum[2] += int(p & 0xff)
	}
	var cells []cell
	for k := range hist {
		c := hist[k]
		if c.count == 0 {
			continue
		}
		c.key = uint16(k)
		c.rgb = [3]uint8{uint8(k >> 10 & 0x1f), uint8(k >> 5 & 0x1f), uint8(k & 0x1f)}
		cells = append(cells, c)
	}
	boxes := []box{newBox(cells)}
	for len(boxes) < colors {
		// The box whose pixels sit furthest from a single color, roughly.
		best, score := -1, 0
		for i, b := range boxes {
			if b.spread > 0 && b.count*b.spread > score {
				best, score = i, b.count*b.spread
			}
		}
		if best < 0 {
			break
		}
		lo, hi := boxes[best].split()
		boxes[best] = lo
		boxes = append(boxes, hi)
	}

	palette := make(color.Palette, 1, len(boxes)+1)
	palette[0] = color.RGBA{}
	var slot [1 << 15]uint8
	for i, b := range boxes {
		var sum [3]int
		for _, c := range b.cells {
			slot[c.key] = uint8(i + 1)
			for ch := range 3 {
				sum[ch] += c.sum[ch]
			}
		}
		avg := func(ch int) uint8 { return uint8(sum[ch] / b.count & 0xff) }
		palette = append(palette, color.RGBA{R: avg(0), G: avg(1), B: avg(2), A: 0xff})
	}
	return palette, func(p uint32) uint8 { return slot[cellKey(p)] }
}
