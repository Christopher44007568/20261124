package strided

import (
	"fmt"
	"math"
)

// validate checks the full geometry of p against len(buf). Every
// multiplication and addition is guarded so hostile dimensions report
// ErrOverflow instead of wrapping around.
func validate(buf []byte, p Params) error {
	bpp := p.Format.BytesPerPixel()
	if bpp == 0 {
		return &Error{Kind: ErrInvalidStride, Format: p.Format, Row: -1, Offset: -1,
			Detail: fmt.Sprintf("unknown format %d", int(p.Format))}
	}
	if p.Width <= 0 || p.Height <= 0 {
		return &Error{Kind: ErrEmptyImage, Format: p.Format, Row: -1, Offset: -1,
			Detail: fmt.Sprintf("width=%d height=%d", p.Width, p.Height)}
	}

	w := int64(p.Width)
	if w > math.MaxInt64/int64(bpp) {
		return &Error{Kind: ErrOverflow, Format: p.Format, Row: -1, Offset: -1,
			Detail: fmt.Sprintf("width*bpp = %d*%d overflows int64", w, bpp)}
	}
	rowBytes := w * int64(bpp)

	stride := int64(p.Stride)
	absStride := stride
	if absStride < 0 {
		absStride = -absStride
	}
	if absStride < rowBytes {
		return &Error{Kind: ErrInvalidStride, Format: p.Format, Row: -1, Offset: -1,
			Detail: fmt.Sprintf("|stride|=%d < rowBytes=%d", absStride, rowBytes)}
	}

	// Byte range of the first and last row; intermediate rows lie inside
	// their hull because row starts move monotonically with y.
	first := int64(p.Offset)
	last := first
	if h := int64(p.Height - 1); h > 0 && stride != 0 {
		if absStride > math.MaxInt64/h {
			return &Error{Kind: ErrOverflow, Format: p.Format, Row: -1, Offset: -1,
				Detail: fmt.Sprintf("(height-1)*|stride| = %d*%d overflows int64", h, absStride)}
		}
		delta := h * stride
		if (delta > 0 && first > math.MaxInt64-delta) ||
			(delta < 0 && first < math.MinInt64-delta) {
			return &Error{Kind: ErrOverflow, Format: p.Format, Row: -1, Offset: -1,
				Detail: fmt.Sprintf("offset+(height-1)*stride = %d+(%d) overflows int64", first, delta)}
		}
		last = first + delta
	}

	lo := min(first, last)
	hiStart := max(first, last)
	if hiStart > math.MaxInt64-rowBytes {
		return &Error{Kind: ErrOverflow, Format: p.Format, Row: -1, Offset: -1,
			Detail: fmt.Sprintf("row end %d+%d overflows int64", hiStart, rowBytes)}
	}
	hi := hiStart + rowBytes

	if lo < 0 || hi > int64(len(buf)) {
		row, off := firstBadRow(p, rowBytes, int64(len(buf)))
		return &Error{Kind: ErrOutOfBounds, Format: p.Format, Row: row, Offset: off,
			Detail: fmt.Sprintf("need bytes [%d,%d) of %d-byte buffer", lo, hi, len(buf))}
	}
	return nil
}

// firstBadRow finds the first row whose [start, start+rowBytes) range
// escapes [0, bufLen). Assumes the hull check already failed.
func firstBadRow(p Params, rowBytes, bufLen int64) (int, int64) {
	for y := 0; y < p.Height; y++ {
		start := int64(p.Offset) + int64(y)*int64(p.Stride)
		if start < 0 || start+rowBytes > bufLen {
			return y, start
		}
	}
	return -1, -1
}
