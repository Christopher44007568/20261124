package reprojection

import "math"

// PanelClass separates the world's near field from its background.
type PanelClass uint8

const (
	Background PanelClass = iota
	MidField
	NearField
)

// WorldPanel is a finite, camera-facing rectangle at a fixed world Z.
type WorldPanel struct {
	Name      string
	Class     PanelClass
	Z         float64
	MinX      float64
	MaxX      float64
	MinY      float64
	MaxY      float64
	Base      Color
	Accent    Color
	Grid      float64
	Highlight float64
}

func (p WorldPanel) intersect(origin, dir Vec3) (Vec3, float64, bool) {
	if math.Abs(dir.Z) < 1e-9 {
		return Vec3{}, 0, false
	}
	t := (p.Z - origin.Z) / dir.Z
	if t <= nearClip {
		return Vec3{}, 0, false
	}
	hit := origin.Add(dir.Scale(t))
	if hit.X < p.MinX || hit.X > p.MaxX || hit.Y < p.MinY || hit.Y > p.MaxY {
		return Vec3{}, 0, false
	}
	return hit, t, true
}

func (p WorldPanel) colorAt(hit Vec3) Color {
	grid := p.Grid
	if grid <= 0 {
		grid = 0.5
	}
	gx := math.Mod(math.Abs(hit.X)+grid*0.5, grid)
	gy := math.Mod(math.Abs(hit.Y)+grid*0.5, grid)
	line := math.Min(gx, grid-gx) < grid*0.055 ||
		math.Min(gy, grid-gy) < grid*0.055

	edge := math.Min(
		math.Min(hit.X-p.MinX, p.MaxX-hit.X),
		math.Min(hit.Y-p.MinY, p.MaxY-hit.Y),
	)
	border := 1 - math.Min(1, edge/math.Max(p.Highlight, 1e-6))
	if p.Highlight <= 0 {
		border = 0
	}

	c := p.Base
	if line {
		c = lerpColor(c, p.Accent, 0.35)
	}
	if border > 0 {
		c = lerpColor(c, p.Accent, 0.25+0.75*border)
	}

	// Mark the optical axis on the back wall so rotation is visible.
	if p.Class == Background && hit.X*hit.X+hit.Y*hit.Y < 0.035 {
		c = lerpColor(c, RGB(235, 242, 255), 0.7)
	}
	return c
}

// DefaultWorld builds a background wall, a mid-field slab and two close
// occluders. The near occluders are in front of the UI's declared depth.
func DefaultWorld() []WorldPanel {
	return []WorldPanel{
		{
			Name:   "back-wall",
			Class:  Background,
			Z:      5.0,
			MinX:   -20,
			MaxX:   20,
			MinY:   -12,
			MaxY:   12,
			Base:   RGB(18, 29, 52),
			Accent: RGB(54, 91, 138),
			Grid:   0.55,
		},
		{
			Name:      "mid-slab",
			Class:     MidField,
			Z:         2.4,
			MinX:      -1.25,
			MaxX:      -0.62,
			MinY:      -0.58,
			MaxY:      0.58,
			Base:      RGB(20, 88, 92),
			Accent:    RGB(78, 210, 198),
			Grid:      0.22,
			Highlight: 0.025,
		},
		{
			Name:      "near-center",
			Class:     NearField,
			Z:         0.62,
			MinX:      -0.24,
			MaxX:      0.20,
			MinY:      -0.38,
			MaxY:      0.38,
			Base:      RGB(176, 68, 39),
			Accent:    RGB(255, 187, 79),
			Grid:      0.11,
			Highlight: 0.018,
		},
		{
			Name:      "near-right",
			Class:     NearField,
			Z:         0.82,
			MinX:      0.58,
			MaxX:      0.88,
			MinY:      -0.34,
			MaxY:      0.34,
			Base:      RGB(103, 45, 133),
			Accent:    RGB(238, 121, 255),
			Grid:      0.14,
			Highlight: 0.02,
		},
	}
}
