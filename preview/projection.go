package preview

import (
	"math"

	"headpose"
)

type point struct {
	X float64
	Y float64
}

type rect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

func (r rect) center() point {
	return point{X: r.X + r.Width/2, Y: r.Y + r.Height/2}
}

type worldProjector struct {
	scale   float64
	offsetX float64
	offsetY float64
}

func fitWorldProjector(points []headpose.Vec3, bounds rect) worldProjector {
	raw := make([]point, len(points))
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for i, v := range points {
		raw[i] = isometricRaw(v)
		minX = math.Min(minX, raw[i].X)
		minY = math.Min(minY, raw[i].Y)
		maxX = math.Max(maxX, raw[i].X)
		maxY = math.Max(maxY, raw[i].Y)
	}
	spanX := math.Max(maxX-minX, 1e-6)
	spanY := math.Max(maxY-minY, 1e-6)
	scale := 0.9 * math.Min(bounds.Width/spanX, bounds.Height/spanY)
	center := bounds.center()
	return worldProjector{
		scale:   scale,
		offsetX: center.X - (minX+maxX)/2*scale,
		offsetY: center.Y - (minY+maxY)/2*scale,
	}
}

func (p worldProjector) project(v headpose.Vec3) point {
	raw := isometricRaw(v)
	return point{X: p.offsetX + raw.X*p.scale, Y: p.offsetY + raw.Y*p.scale}
}

func isometricRaw(v headpose.Vec3) point {
	const cosine30 = 0.8660254037844386
	return point{
		X: (v.X - v.Z) * cosine30,
		Y: (v.X+v.Z)*0.5 - v.Y,
	}
}

type screenProjector struct {
	view   rect
	scale  float64
	offset point
}

func fitScreenProjector(width, height int, bounds rect) screenProjector {
	scale := math.Min(bounds.Width/float64(width), bounds.Height/float64(height))
	usedWidth := float64(width) * scale
	usedHeight := float64(height) * scale
	return screenProjector{
		view: rect{
			X:      bounds.X + (bounds.Width-usedWidth)/2,
			Y:      bounds.Y + (bounds.Height-usedHeight)/2,
			Width:  usedWidth,
			Height: usedHeight,
		},
		scale: scale,
		offset: point{
			X: bounds.X + (bounds.Width-usedWidth)/2,
			Y: bounds.Y + (bounds.Height-usedHeight)/2,
		},
	}
}

func (p screenProjector) project(screen headpose.Vec3) point {
	return point{
		X: p.offset.X + screen.X*p.scale,
		Y: p.offset.Y + screen.Y*p.scale,
	}
}

func (p screenProjector) viewport() rect {
	return p.view
}
