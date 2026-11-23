package binocular

import (
	"math"
)

type clipVertex struct {
	position Vec4
	color    Vec3
}

type screenVertex struct {
	x, y  float64
	depth float64
	invW  float64
	color Vec3
}

func drawTriangle(
	image *Image,
	depthBuffer []float64,
	stats *EyeStats,
	modelViewProjection Mat4,
	vertices [3]Vertex,
	material Material,
) {
	var polygon [3]clipVertex
	for i, vertex := range vertices {
		polygon[i] = clipVertex{
			position: modelViewProjection.Transform(Vec4{
				X: vertex.Position.X,
				Y: vertex.Position.Y,
				Z: vertex.Position.Z,
				W: 1,
			}),
			color: vertex.Color,
		}
	}

	clipped, wasClipped := clipToView(polygon[:])
	if wasClipped {
		stats.ClippedTriangles++
	}
	if len(clipped) < 3 {
		return
	}

	screen := make([]screenVertex, len(clipped))
	valid := true
	for i, vertex := range clipped {
		w := vertex.position.W
		if math.Abs(w) < 1e-12 {
			valid = false
			break
		}
		ndcX := vertex.position.X / w
		ndcY := vertex.position.Y / w
		ndcZ := vertex.position.Z / w
		screen[i] = screenVertex{
			x:     (ndcX*0.5 + 0.5) * float64(image.Width),
			y:     (0.5 - ndcY*0.5) * float64(image.Height),
			depth: ndcZ,
			invW:  1 / w,
			color: vertex.color,
		}
	}
	if !valid {
		return
	}

	for i := 1; i+1 < len(screen); i++ {
		if rasterize(image, depthBuffer, stats, screen[0], screen[i], screen[i+1], material) {
			stats.RasterizedTriangles++
		}
	}
}

func clipToView(polygon []clipVertex) ([]clipVertex, bool) {
	planes := [...]func(Vec4) float64{
		func(v Vec4) float64 { return v.X + v.W }, // left
		func(v Vec4) float64 { return v.W - v.X }, // right
		func(v Vec4) float64 { return v.Y + v.W }, // bottom
		func(v Vec4) float64 { return v.W - v.Y }, // top
		func(v Vec4) float64 { return v.Z + v.W }, // near
		func(v Vec4) float64 { return v.W - v.Z }, // far
	}

	current := polygon
	wasClipped := false
	for _, plane := range planes {
		next := make([]clipVertex, 0, len(current)+1)
		for i := range current {
			j := (i + 1) % len(current)
			a, b := current[i], current[j]
			da, db := plane(a.position), plane(b.position)
			if da < 0 {
				wasClipped = true
			}
			aInside, bInside := da >= 0, db >= 0
			if aInside != bInside {
				t := da / (da - db)
				next = append(next, interpolateClipVertex(a, b, t))
			}
			if bInside {
				next = append(next, b)
			}
		}
		current = next
		if len(current) < 3 {
			return nil, wasClipped
		}
	}
	return current, wasClipped
}

func interpolateClipVertex(a, b clipVertex, t float64) clipVertex {
	lerp := func(x, y float64) float64 { return x + (y-x)*t }
	return clipVertex{
		position: Vec4{
			X: lerp(a.position.X, b.position.X),
			Y: lerp(a.position.Y, b.position.Y),
			Z: lerp(a.position.Z, b.position.Z),
			W: lerp(a.position.W, b.position.W),
		},
		color: Vec3{
			X: lerp(a.color.X, b.color.X),
			Y: lerp(a.color.Y, b.color.Y),
			Z: lerp(a.color.Z, b.color.Z),
		},
	}
}

func rasterize(
	image *Image,
	depthBuffer []float64,
	stats *EyeStats,
	a, b, c screenVertex,
	material Material,
) bool {
	area := edge(a, b, c)
	if math.Abs(area) < 1e-12 {
		return false
	}

	minX := max(0, int(math.Floor(min3(a.x, b.x, c.x))))
	maxX := min(image.Width-1, int(math.Ceil(max3(a.x, b.x, c.x))))
	minY := max(0, int(math.Floor(min3(a.y, b.y, c.y))))
	maxY := min(image.Height-1, int(math.Ceil(max3(a.y, b.y, c.y))))

	const epsilon = 1e-9
	drew := false
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			sample := screenVertex{x: float64(x) + 0.5, y: float64(y) + 0.5}
			w0 := edge(b, c, sample) / area
			w1 := edge(c, a, sample) / area
			w2 := edge(a, b, sample) / area
			if w0 < -epsilon || w1 < -epsilon || w2 < -epsilon {
				continue
			}

			depth := w0*a.depth + w1*b.depth + w2*c.depth
			pixel := y*image.Width + x
			if depth >= depthBuffer[pixel] {
				continue
			}

			denominator := w0*a.invW + w1*b.invW + w2*c.invW
			if math.Abs(denominator) < 1e-12 {
				continue
			}
			color := Vec3{
				X: (w0*a.color.X*a.invW + w1*b.color.X*b.invW +
					w2*c.color.X*c.invW) / denominator,
				Y: (w0*a.color.Y*a.invW + w1*b.color.Y*b.invW +
					w2*c.color.Y*c.invW) / denominator,
				Z: (w0*a.color.Z*a.invW + w1*b.color.Z*b.invW +
					w2*c.color.Z*c.invW) / denominator,
			}
			color = color.Mul(material.BaseColor)
			depthBuffer[pixel] = depth
			image.Set(x, y, toRGB(color))
			stats.Fragments++
			drew = true
		}
	}
	return drew
}

func edge(a, b, p screenVertex) float64 {
	return (p.x-a.x)*(b.y-a.y) - (p.y-a.y)*(b.x-a.x)
}

func min3(a, b, c float64) float64 {
	return min(a, min(b, c))
}

func max3(a, b, c float64) float64 {
	return max(a, max(b, c))
}

func toRGB(color Vec3) RGB {
	return RGB{
		R: colorChannel(color.X),
		G: colorChannel(color.Y),
		B: colorChannel(color.Z),
	}
}

func colorChannel(value float64) uint8 {
	value = min(1, max(0, value))
	return uint8(value*255 + 0.5)
}
