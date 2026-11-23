package strided

import (
	"errors"
	"image"
	"math"
	"testing"
)

// sentinel fills every non-pixel byte (padding, prefix) so any misread
// of padding as pixel data corrupts the output and fails the test.
const sentinel = 0xEE

// pack builds a buffer holding a w*h image of bpp bytes per pixel with
// the given stride (possibly negative) and start offset. Pixel bytes
// come from gen(x, y); everything else is sentinel.
func pack(w, h, bpp, stride, offset int, gen func(x, y int) []byte) []byte {
	rowBytes := w * bpp
	lo, hi := offset, offset+rowBytes
	for y := 0; y < h; y++ {
		s := offset + y*stride
		if s < lo {
			lo = s
		}
		if s+rowBytes > hi {
			hi = s + rowBytes
		}
	}
	if lo < 0 {
		panic("pack: geometry escapes buffer")
	}
	buf := make([]byte, hi)
	for i := range buf {
		buf[i] = sentinel
	}
	for y := 0; y < h; y++ {
		s := offset + y*stride
		for x := 0; x < w; x++ {
			copy(buf[s+x*bpp:], gen(x, y))
		}
	}
	return buf
}

// requirePixels compares the decoded image against gen byte-by-byte.
func requirePixels(t *testing.T, img image.Image, f Format, w, h int, gen func(x, y int) []byte) {
	t.Helper()
	b := img.Bounds()
	if b.Dx() != w || b.Dy() != h {
		t.Fatalf("bounds = %v, want %dx%d", b, w, h)
	}
	switch f {
	case Gray:
		g, ok := img.(*image.Gray)
		if !ok {
			t.Fatalf("Gray decode returned %T", img)
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				got := g.Pix[y*g.Stride+x]
				if want := gen(x, y)[0]; got != want {
					t.Fatalf("pixel (%d,%d) = %#02x, want %#02x", x, y, got, want)
				}
			}
		}
	case RGB, RGBA:
		n, ok := img.(*image.NRGBA)
		if !ok {
			t.Fatalf("%s decode returned %T", f, img)
		}
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				want := gen(x, y)
				got := n.Pix[y*n.Stride+x*4 : y*n.Stride+x*4+4]
				exp := [4]byte{want[0], want[1], want[2], 0xff}
				if f == RGBA {
					exp[3] = want[3]
				}
				if got[0] != exp[0] || got[1] != exp[1] || got[2] != exp[2] || got[3] != exp[3] {
					t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, got, exp)
				}
			}
		}
	}
}

func TestErrors(t *testing.T) {
	buf := make([]byte, 256)
	cases := []struct {
		name string
		p    Params
		kind ErrKind
		row  int // -1 = don't check
	}{
		{"zeroWidth", Params{Width: 0, Height: 4, Stride: 12, Format: RGB}, ErrEmptyImage, -1},
		{"zeroHeight", Params{Width: 4, Height: 0, Stride: 12, Format: RGB}, ErrEmptyImage, -1},
		{"negativeHeight", Params{Width: 4, Height: -2, Stride: 12, Format: Gray}, ErrEmptyImage, -1},
		{"strideTooSmall", Params{Width: 4, Height: 4, Stride: 11, Format: RGB}, ErrInvalidStride, -1},
		{"negStrideTooSmall", Params{Width: 4, Height: 4, Stride: -11, Format: RGB}, ErrInvalidStride, -1},
		{"unknownFormat", Params{Width: 4, Height: 4, Stride: 99, Format: Format(9)}, ErrInvalidStride, -1},
		{"truncated", Params{Width: 8, Height: 16, Stride: 32, Format: RGBA}, ErrOutOfBounds, 8},
		{"negativeOffset", Params{Width: 4, Height: 4, Stride: 12, Offset: -1, Format: RGB}, ErrOutOfBounds, 0},
		{"negStrideBelowZero", Params{Width: 4, Height: 8, Stride: -16, Offset: 100, Format: RGB}, ErrOutOfBounds, 7},
		{"offsetPastEnd", Params{Width: 4, Height: 4, Stride: 12, Offset: 300, Format: RGB}, ErrOutOfBounds, 0},
		{"overflowWidthBpp", Params{Width: math.MaxInt64/4 + 1, Height: 1, Stride: 1, Format: RGBA}, ErrOverflow, -1},
		{"overflowSpan", Params{Width: 1, Height: 1 << 32, Stride: 1 << 32, Format: Gray}, ErrOverflow, -1},
		{"overflowOffset", Params{Width: 1, Height: 2, Stride: 4, Offset: math.MaxInt - 1, Format: Gray}, ErrOverflow, -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Decode(buf, c.p)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var se *Error
			if !errors.As(err, &se) {
				t.Fatalf("error type = %T, want *Error", err)
			}
			if se.Kind != c.kind {
				t.Fatalf("kind = %v, want %v (err: %v)", se.Kind, c.kind, err)
			}
			if c.row >= 0 && se.Row != c.row {
				t.Fatalf("row = %d, want %d (err: %v)", se.Row, c.row, err)
			}
		})
	}
}

func TestErrorMessageLocates(t *testing.T) {
	_, err := Decode(make([]byte, 64), Params{Width: 8, Height: 8, Stride: 32, Format: RGBA})
	se := err.(*Error)
	if se.Row != 2 || se.Offset != 64 {
		t.Fatalf("row=%d offset=%d, want row=2 offset=64", se.Row, se.Offset)
	}
	if s := se.Error(); s == "" {
		t.Fatal("empty error message")
	} else {
		t.Log(s)
	}
}

// pixelGen returns a per-format generator from a scalar pattern.
func pixelGen(f Format, pattern func(x, y int) byte) func(x, y int) []byte {
	return func(x, y int) []byte {
		v := pattern(x, y)
		switch f {
		case Gray:
			return []byte{v}
		case RGB:
			return []byte{v, v + 1, v + 2}
		default: // RGBA
			return []byte{v, v + 1, v + 2, v + 3}
		}
	}
}

func checkerboard(x, y int) byte {
	if (x+y)%2 == 0 {
		return 0x10
	}
	return 0xF0
}

func solidBorder(w, h int) func(x, y int) byte {
	return func(x, y int) byte {
		if x == 0 || y == 0 || x == w-1 || y == h-1 {
			return 0xAB
		}
		return 0x00
	}
}

// crossRowGradient depends on the linear pixel index, so any row
// misalignment shifts every subsequent byte.
func crossRowGradient(w int) func(x, y int) byte {
	return func(x, y int) byte {
		return byte((y*w + x) * 7)
	}
}

func TestPatterns(t *testing.T) {
	const w, h = 7, 5 // odd width forces real padding
	patterns := map[string]func(x, y int) byte{
		"checkerboard":     checkerboard,
		"solidBorder":      solidBorder(w, h),
		"crossRowGradient": crossRowGradient(w),
	}
	formats := []Format{Gray, RGB, RGBA}
	geoms := []struct {
		name string
		geom func(bpp int) (stride, offset int)
	}{
		{"tight", func(bpp int) (int, int) { return w * bpp, 0 }},
		{"padded", func(bpp int) (int, int) { return w*bpp + 5, 0 }},
		{"offset", func(bpp int) (int, int) { return w*bpp + 3, 11 }},
		{"negativeStride", func(bpp int) (int, int) {
			s := w*bpp + 4
			return -s, (h-1)*s + 2 // row 0 sits near the buffer end
		}},
	}
	for pname, pattern := range patterns {
		for _, f := range formats {
			for _, g := range geoms {
				t.Run(pname+"/"+f.String()+"/"+g.name, func(t *testing.T) {
					bpp := f.BytesPerPixel()
					stride, offset := g.geom(bpp)
					gen := pixelGen(f, pattern)
					buf := pack(w, h, bpp, stride, offset, gen)
					img, err := Decode(buf, Params{Width: w, Height: h, Stride: stride, Offset: offset, Format: f})
					if err != nil {
						t.Fatalf("Decode: %v", err)
					}
					requirePixels(t, img, f, w, h, gen)
				})
			}
		}
	}
}
