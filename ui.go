package reprojection

import "math"

// Reticle is a fixed, screen-space aiming element.
type Reticle struct {
	Center    Vec2 // normalized screen coordinates
	Radius    float64
	Thickness float64
	Depth     float64
	Color     Color
}

// DepthRuler is a fixed screen-space depth scale. Its ticks are only a depth
// cue; merging it into a world depth buffer must not decide its visibility.
type DepthRuler struct {
	X          float64 // normalized screen X
	Top        float64 // normalized screen Y
	Bottom     float64
	Width      float64
	TickLength float64
	Depth      float64
	Color      Color
}

// HandCursor follows the tracked hand but remains a screen-space overlay.
type HandCursor struct {
	Radius float64
	Depth  float64
	Color  Color
}

type UILayer struct {
	Reticle Reticle
	Ruler   DepthRuler
	Cursor  HandCursor
}

func DefaultUI() UILayer {
	return UILayer{
		Reticle: Reticle{
			Center:    Vec2{0.5, 0.5},
			Radius:    14,
			Thickness: 2,
			Depth:     1.25,
			Color:     RGB(112, 255, 167),
		},
		Ruler: DepthRuler{
			X:          0.90,
			Top:        0.32,
			Bottom:     0.68,
			Width:      3,
			TickLength: 16,
			Depth:      1.40,
			Color:      RGB(255, 219, 92),
		},
		Cursor: HandCursor{
			Radius: 10,
			Depth:  1.10,
			Color:  RGB(88, 225, 255),
		},
	}
}

// UIState is the latest screen-space cursor sample used by the layer.
type UIState struct {
	Cursor  Vec2
	Visible bool
}

type UIPixel struct {
	Color  Color
	Depth  float64
	Filled bool
}

type UIImage struct {
	Width  int
	Height int
	Pixels []UIPixel
}

func NewUIImage(width, height int) *UIImage {
	return &UIImage{Width: width, Height: height, Pixels: make([]UIPixel, width*height)}
}

func (img *UIImage) At(x, y int) *UIPixel {
	if x < 0 || y < 0 || x >= img.Width || y >= img.Height {
		return nil
	}
	return &img.Pixels[y*img.Width+x]
}

func (img *UIImage) blend(x, y int, c Color, depth float64) {
	p := img.At(x, y)
	if p == nil {
		return
	}
	if !p.Filled || depth < p.Depth {
		p.Depth = depth
	}
	p.Color = c.Over(p.Color)
	p.Filled = true
}

func (img *UIImage) line(a, b Vec2, width float64, c Color, depth float64) {
	dx := b.X - a.X
	dy := b.Y - a.Y
	steps := int(math.Ceil(math.Max(math.Abs(dx), math.Abs(dy))))
	if steps < 1 {
		steps = 1
	}
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		x := a.X + dx*t
		y := a.Y + dy*t
		img.square(x, y, width, c, depth)
	}
}

func (img *UIImage) square(cx, cy, size float64, c Color, depth float64) {
	half := math.Max(0.5, size/2)
	x0 := int(math.Floor(cx - half))
	x1 := int(math.Ceil(cx + half))
	y0 := int(math.Floor(cy - half))
	y1 := int(math.Ceil(cy + half))
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			img.blend(x, y, c, depth)
		}
	}
}

func (img *UIImage) ring(center Vec2, radius, thickness float64, c Color, depth float64) {
	outer := radius + thickness/2
	inner := math.Max(0, radius-thickness/2)
	x0 := int(math.Floor(center.X - outer - 1))
	x1 := int(math.Ceil(center.X + outer + 1))
	y0 := int(math.Floor(center.Y - outer - 1))
	y1 := int(math.Ceil(center.Y + outer + 1))
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			d := math.Hypot(float64(x)+0.5-center.X, float64(y)+0.5-center.Y)
			if d >= inner && d <= outer {
				img.blend(x, y, c, depth)
			}
		}
	}
}

// Render rasterizes the UI directly into screen space. Camera pose is not an
// input, which is the point: fixed UI must not inherit the world camera.
func (layer UILayer) Render(cam Camera, state UIState) *UIImage {
	img := NewUIImage(cam.Width, cam.Height)
	w := float64(cam.Width)
	h := float64(cam.Height)

	reticle := layer.Reticle
	rc := Vec2{reticle.Center.X * w, reticle.Center.Y * h}
	img.ring(rc, reticle.Radius, reticle.Thickness, reticle.Color, reticle.Depth)
	for i := 0; i < 4; i++ {
		angle := float64(i) * math.Pi / 2
		a := Vec2{rc.X + math.Cos(angle)*(reticle.Radius+4), rc.Y + math.Sin(angle)*(reticle.Radius+4)}
		b := Vec2{rc.X + math.Cos(angle)*(reticle.Radius+9), rc.Y + math.Sin(angle)*(reticle.Radius+9)}
		img.line(a, b, reticle.Thickness, reticle.Color, reticle.Depth)
	}

	ruler := layer.Ruler
	rx := ruler.X * w
	top := ruler.Top * h
	bottom := ruler.Bottom * h
	img.line(Vec2{rx, top}, Vec2{rx, bottom}, ruler.Width, ruler.Color, ruler.Depth)
	for i := 0; i < 5; i++ {
		y := top + (bottom-top)*float64(i)/4
		length := ruler.TickLength
		if i%2 == 0 {
			length *= 1.6
		}
		img.line(Vec2{rx, y}, Vec2{rx - length, y}, ruler.Width, ruler.Color, ruler.Depth)
	}

	if state.Visible {
		img.ring(state.Cursor, layer.Cursor.Radius, 2, layer.Cursor.Color, layer.Cursor.Depth)
		img.square(state.Cursor.X, state.Cursor.Y, 3, layer.Cursor.Color, layer.Cursor.Depth)
	}
	return img
}

type UIAnchor struct {
	Name   string
	Screen Vec2
	Depth  float64
}

func (layer UILayer) Anchors(cam Camera, state UIState) []UIAnchor {
	w := float64(cam.Width)
	h := float64(cam.Height)
	anchors := []UIAnchor{
		{
			Name:   "reticle",
			Screen: Vec2{layer.Reticle.Center.X * w, layer.Reticle.Center.Y * h},
			Depth:  layer.Reticle.Depth,
		},
		{
			Name:   "ruler",
			Screen: Vec2{layer.Ruler.X * w, (layer.Ruler.Top + layer.Ruler.Bottom) * 0.5 * h},
			Depth:  layer.Ruler.Depth,
		},
	}
	if state.Visible {
		anchors = append(anchors, UIAnchor{Name: "cursor", Screen: state.Cursor, Depth: layer.Cursor.Depth})
	}
	return anchors
}

// TransformUI is the intentionally wrong path: every UI pixel is treated as a
// world sample at its depth scale and carried through the world camera.
func TransformUI(src *UIImage, cam Camera, sourcePose, targetPose Pose) *UIImage {
	dst := NewUIImage(src.Width, src.Height)
	for i := range src.Pixels {
		p := &src.Pixels[i]
		if !p.Filled {
			continue
		}
		x := i % src.Width
		y := i / src.Width
		screen := Vec2{float64(x) + 0.5, float64(y) + 0.5}
		world := cam.UnprojectScreen(screen, p.Depth, sourcePose)
		out, depth, ok := cam.Project(world, targetPose)
		if !ok || !cam.inside(out) {
			continue
		}
		x0 := int(math.Floor(out.X))
		y0 := int(math.Floor(out.Y))
		for oy := 0; oy < 2; oy++ {
			for ox := 0; ox < 2; ox++ {
				q := dst.At(x0+ox, y0+oy)
				if q == nil {
					continue
				}
				if q.Filled && q.Depth <= depth {
					continue
				}
				q.Color = p.Color.Over(q.Color)
				q.Depth = depth
				q.Filled = true
			}
		}
	}
	return dst
}
