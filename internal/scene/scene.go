// Package scene builds the deterministic synthetic scene: planar panels at
// known depths carrying checkerboard texture and bright markers whose 3-D
// world positions are known ground truth.
package scene

import "stereoval/internal/geo"

// Marker is a bright circular landmark with a known world position.
type Marker struct {
	ID     int
	Center geo.Vec3
	Radius float64 // world units
	Panel  int
}

// Panel is a fronto-parallel quad (normal points toward -Z, i.e. the camera).
type Panel struct {
	ID             int
	Center         geo.Vec3
	Width, Height  float64
	CheckerPerUnit float64
}

// Corner returns corner i: 0=TL, 1=TR, 2=BR, 3=BL.
func (p Panel) Corner(i int) geo.Vec3 {
	hw, hh := p.Width/2, p.Height/2
	dx := [4]float64{-hw, hw, hw, -hw}[i]
	dy := [4]float64{hh, hh, -hh, -hh}[i]
	return geo.Vec3{X: p.Center.X + dx, Y: p.Center.Y + dy, Z: p.Center.Z}
}

// LocalUV maps a world point to panel-local UV in [0,1]x[0,1].
func (p Panel) LocalUV(w geo.Vec3) (u, v float64, inside bool) {
	u = (w.X - (p.Center.X - p.Width/2)) / p.Width
	v = (w.Y - (p.Center.Y - p.Height/2)) / p.Height
	inside = u >= 0 && u <= 1 && v >= 0 && v <= 1
	return
}

// Scene is the full static environment.
type Scene struct {
	Wall    Panel
	Panels  []Panel
	Markers []Marker
}

// New builds the fixed scene. All depths and marker positions are constants so
// downstream metrics always compare against exact ground truth.
func New() *Scene {
	wall := Panel{ID: 0, Center: geo.Vec3{Z: 8}, Width: 14, Height: 9, CheckerPerUnit: 1.2}
	panels := []Panel{
		// Horizontal separation is chosen so the nearer panel's screen
		// footprint never covers a farther panel's markers from the origin.
		{ID: 1, Center: geo.Vec3{X: -1.1, Y: 0.3, Z: 3.6}, Width: 0.9, Height: 1.0, CheckerPerUnit: 2.0},
		{ID: 2, Center: geo.Vec3{X: 0.0, Y: -0.2, Z: 4.8}, Width: 1.2, Height: 1.1, CheckerPerUnit: 1.6},
		{ID: 3, Center: geo.Vec3{X: 1.25, Y: 0.4, Z: 6.0}, Width: 1.2, Height: 1.2, CheckerPerUnit: 1.3},
	}

	var markers []Marker
	id := 0
	fracs := []float64{0.2, 0.5, 0.8}
	for _, p := range panels {
		for _, fy := range fracs {
			for _, fx := range fracs {
				markers = append(markers, Marker{
					ID:     id,
					Center: geo.Vec3{X: p.Center.X - p.Width/2 + fx*p.Width, Y: p.Center.Y - p.Height/2 + fy*p.Height, Z: p.Center.Z},
					Radius: 0.055,
					Panel:  p.ID,
				})
				id++
			}
		}
		// Extra near-edge markers: they are the first to enter disocclusions
		// during lateral motion, exercising cross-eye fill behavior.
		edgeXs := []float64{p.Center.X - p.Width/2 + 0.22, p.Center.X + p.Width/2 - 0.22}
		for _, ex := range edgeXs {
			markers = append(markers, Marker{
				ID:     id,
				Center: geo.Vec3{X: ex, Y: p.Center.Y, Z: p.Center.Z},
				Radius: 0.055,
				Panel:  p.ID,
			})
			id++
		}
	}
	return &Scene{Wall: wall, Panels: panels, Markers: markers}
}

// Shade is the procedural material: checkerboard in panel-local coordinates,
// with distinct gray bands per panel so the two eyes never match by accident.
func Shade(p Panel, u, v float64) float64 {
	ix := int(u*p.Width*p.CheckerPerUnit) + int(p.ID)*7
	iy := int(v*p.Height*p.CheckerPerUnit) + int(p.ID)*3
	if (ix+iy)&1 == 0 {
		return 150
	}
	return 80
}
