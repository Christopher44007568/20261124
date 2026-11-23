// Package strided decodes tightly validated pixel buffers whose row stride
// is not necessarily width*bpp: rows may carry padding bytes, the stride
// may be negative (bottom-up layout), and the first row may start at a
// non-zero offset inside the buffer.
package strided

import (
	"image"
)

// Format is the pixel layout of the source buffer.
type Format int

const (
	Gray Format = iota // 1 byte per pixel
	RGB                // 3 bytes per pixel, no alpha
	RGBA               // 4 bytes per pixel
)

func (f Format) String() string {
	switch f {
	case Gray:
		return "gray"
	case RGB:
		return "rgb"
	case RGBA:
		return "rgba"
	default:
		return "unknown"
	}
}

// BytesPerPixel returns the packed size of one pixel.
func (f Format) BytesPerPixel() int {
	switch f {
	case Gray:
		return 1
	case RGB:
		return 3
	case RGBA:
		return 4
	default:
		return 0
	}
}

// Params describes the source buffer geometry.
type Params struct {
	Width  int // pixels per row, must be > 0
	Height int // number of rows, must be > 0
	Stride int // bytes between consecutive row starts; may be negative
	Offset int // byte offset of the first row's first pixel
	Format Format
}

// Decode reads the image described by p out of buf and returns a freshly
// allocated stdlib image: *image.Gray for Gray, *image.NRGBA for RGB
// (alpha forced to 255) and RGBA.
//
// Padding bytes between rows are never read as pixels. A negative Stride
// walks backwards through buf, i.e. row 0 starts at Offset and each
// subsequent row starts Stride bytes earlier. All geometry is validated
// before any pixel is touched; failures return a *Error that names the
// offending row and byte offset.
func Decode(buf []byte, p Params) (image.Image, error) {
	if err := validate(buf, p); err != nil {
		return nil, err
	}
	bpp := p.Format.BytesPerPixel()
	rowBytes := p.Width * bpp

	switch p.Format {
	case Gray:
		dst := image.NewGray(image.Rect(0, 0, p.Width, p.Height))
		for y := 0; y < p.Height; y++ {
			src := rowStart(p.Offset, p.Stride, y)
			copy(dst.Pix[y*dst.Stride:y*dst.Stride+rowBytes], buf[src:src+rowBytes])
		}
		return dst, nil
	case RGB:
		dst := image.NewNRGBA(image.Rect(0, 0, p.Width, p.Height))
		for y := 0; y < p.Height; y++ {
			src := rowStart(p.Offset, p.Stride, y)
			s, d := src, y*dst.Stride
			for x := 0; x < p.Width; x++ {
				dst.Pix[d+0] = buf[s+0]
				dst.Pix[d+1] = buf[s+1]
				dst.Pix[d+2] = buf[s+2]
				dst.Pix[d+3] = 0xff
				s += 3
				d += 4
			}
		}
		return dst, nil
	case RGBA:
		dst := image.NewNRGBA(image.Rect(0, 0, p.Width, p.Height))
		for y := 0; y < p.Height; y++ {
			src := rowStart(p.Offset, p.Stride, y)
			copy(dst.Pix[y*dst.Stride:y*dst.Stride+rowBytes], buf[src:src+rowBytes])
		}
		return dst, nil
	}
	panic("unreachable: validate rejects unknown formats")
}

// rowStart returns the byte offset of row y's first pixel. Validated
// geometry guarantees the result plus rowBytes stays inside buf, so the
// int64->int conversion is safe.
func rowStart(offset, stride, y int) int {
	return offset + y*stride
}
