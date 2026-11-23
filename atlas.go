package eyebuffer

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
)

// Sentinel colors make layout mistakes visible instead of silently blending
// into a neighbouring viewport.
var (
	GutterColor     = color.RGBA{R: 255, G: 0, B: 255, A: 255}
	RowPaddingColor = color.RGBA{R: 0, G: 255, B: 255, A: 255}
)

var eyePalette = [...]color.RGBA{
	{R: 218, G: 48, B: 42, A: 255},
	{R: 42, G: 92, B: 222, A: 255},
	{R: 34, G: 176, B: 82, A: 255},
	{R: 218, G: 142, B: 32, A: 255},
}

// Atlas owns the layout and its tightly referenced RGBA8 bytes. Layout.Stride
// is deliberately used for addressing so a tampered or caller-supplied stride
// produces a visible sampling failure.
type Atlas struct {
	Layout AtlasLayout
	Pix    []byte
}

// NewAtlas allocates and clears an atlas, including row padding.
func NewAtlas(layout AtlasLayout) (*Atlas, error) {
	if layout.Width <= 0 || layout.Height <= 0 || layout.Stride < layout.Width*BytesPerPixel {
		return nil, fmt.Errorf("invalid atlas geometry: %dx%d stride=%d", layout.Width, layout.Height, layout.Stride)
	}
	a := &Atlas{Layout: layout, Pix: make([]byte, layout.Stride*layout.Height)}
	for y := 0; y < layout.Height; y++ {
		fillRGBA(a.Pix[y*layout.Stride:(y+1)*layout.Stride], GutterColor)
		fillRGBA(a.Pix[y*layout.Stride+layout.Width*BytesPerPixel:(y+1)*layout.Stride], RowPaddingColor)
	}
	return a, nil
}

// Generate builds a layout and fills every eye level with deterministic texels.
func Generate(eyes []EyeSpec, opts LayoutOptions) (*Atlas, error) {
	layout, err := BuildLayout(eyes, opts)
	if err != nil {
		return nil, err
	}
	atlas, err := NewAtlas(layout)
	if err != nil {
		return nil, err
	}
	for eyeIndex, eye := range layout.Eyes {
		for level, lv := range eye.Levels {
			for y := 0; y < lv.Height; y++ {
				for x := 0; x < lv.Width; x++ {
					atlas.SetPixel(lv.Rect.Min.X+x, lv.Rect.Min.Y+y,
						TexelColor(eyeIndex, level, x, y, lv.Width, lv.Height))
				}
			}
		}
	}
	return atlas, nil
}

// TexelColor generates a high-contrast eye/level/coordinate tag. Different eyes
// never share the same palette, and the small hash term catches a one-row or
// one-pixel stride shift.
func TexelColor(eyeIndex, level, x, y, width, height int) color.RGBA {
	base := eyePalette[eyeIndex%len(eyePalette)]
	u := 0
	v := 0
	if width > 1 {
		u = x * 255 / (width - 1)
	}
	if height > 1 {
		v = y * 255 / (height - 1)
	}
	checker := -10
	if ((x/8)+(y/8)+level)%2 == 0 {
		checker = 14
	}
	hash := (x*37 + y*17 + level*29 + eyeIndex*53) & 31
	return color.RGBA{
		R: clamp8(int(base.R) + u/7 + v/31 + level*9 + checker + hash - 16),
		G: clamp8(int(base.G) + v/6 + u/41 + level*5 + checker + ((hash * 3) & 15) - 7),
		B: clamp8(int(base.B) + (u+v)/12 + level*7 + checker + ((hash * 5) & 15) - 7),
		A: 255,
	}
}

func clamp8(value int) uint8 {
	if value < 0 {
		return 0
	}
	if value > 255 {
		return 255
	}
	return uint8(value)
}

func fillRGBA(dst []byte, c color.RGBA) {
	for i := 0; i+3 < len(dst); i += 4 {
		dst[i] = c.R
		dst[i+1] = c.G
		dst[i+2] = c.B
		dst[i+3] = c.A
	}
}

func (a *Atlas) offset(x, y int) int {
	return y*a.Layout.Stride + x*BytesPerPixel
}

// Pixel returns one atlas texel. The boolean is false outside Width/Height.
func (a *Atlas) Pixel(x, y int) (color.RGBA, bool) {
	if x < 0 || y < 0 || x >= a.Layout.Width || y >= a.Layout.Height {
		return color.RGBA{}, false
	}
	off := a.offset(x, y)
	if off < 0 || off+3 >= len(a.Pix) {
		return color.RGBA{}, false
	}
	return color.RGBA{R: a.Pix[off], G: a.Pix[off+1], B: a.Pix[off+2], A: a.Pix[off+3]}, true
}

// SetPixel writes one atlas texel, ignoring coordinates outside the atlas.
func (a *Atlas) SetPixel(x, y int, c color.RGBA) {
	if x < 0 || y < 0 || x >= a.Layout.Width || y >= a.Layout.Height {
		return
	}
	off := a.offset(x, y)
	if off < 0 || off+3 >= len(a.Pix) {
		return
	}
	a.Pix[off] = c.R
	a.Pix[off+1] = c.G
	a.Pix[off+2] = c.B
	a.Pix[off+3] = c.A
}

// Image converts the padded atlas rows to an image without exposing row
// padding to PNG encoders.
func (a *Atlas) Image() *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, a.Layout.Width, a.Layout.Height))
	for y := 0; y < a.Layout.Height; y++ {
		src := a.Pix[y*a.Layout.Stride:]
		for x := 0; x < a.Layout.Width; x++ {
			di := y*dst.Stride + x*4
			si := x * 4
			copy(dst.Pix[di:di+4], src[si:si+4])
		}
	}
	return dst
}

// WritePNG writes the visible atlas rectangle.
func (a *Atlas) WritePNG(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, a.Image()); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// CopyLevel copies a caller-provided source image into a mip level. Source
// stride is validated before a single row is copied, which catches the common
// width-versus-row-span bug at its origin.
func (a *Atlas) CopyLevel(eyeIndex, level int, src []byte, srcStride int) error {
	lv, ok := a.Layout.Level(eyeIndex, level)
	if !ok {
		return fmt.Errorf("eye %d level %d does not exist", eyeIndex, level)
	}
	if srcStride < lv.Width*BytesPerPixel {
		return fmt.Errorf("eye %q level %d source stride %d is smaller than width span %d",
			lv.Eye, level, srcStride, lv.Width*BytesPerPixel)
	}
	if srcStride%4 != 0 {
		return fmt.Errorf("eye %q level %d source stride %d is not 4-byte aligned",
			lv.Eye, level, srcStride)
	}
	if len(src) < srcStride*lv.Height {
		return fmt.Errorf("eye %q level %d source has %d bytes, need at least %d",
			lv.Eye, level, len(src), srcStride*lv.Height)
	}
	for y := 0; y < lv.Height; y++ {
		srcRow := src[y*srcStride : y*srcStride+lv.Width*BytesPerPixel]
		dstOff := a.offset(lv.Rect.Min.X, lv.Rect.Min.Y+y)
		copy(a.Pix[dstOff:dstOff+lv.Width*BytesPerPixel], srcRow)
	}
	return nil
}
