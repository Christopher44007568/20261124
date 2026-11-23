package main

import "math"

type Camera struct {
	Name     string  `json:"name"`
	Position Vec3    `json:"position"`
	Target   Vec3    `json:"target"`
	Up       Vec3    `json:"up"`
	Zoom     float64 `json:"zoom"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Occluder bool    `json:"occluder"`
}

type RenderBuffer struct {
	Width   int
	Height  int
	Covered []bool
	Depth   []float64
	Normals []Vec3
}

type Frame struct {
	Camera Camera
	Buffer RenderBuffer
}

type cameraBasis struct {
	origin Vec3
	right  Vec3
	up     Vec3
	front  Vec3
	zoom   float64
}

func newRenderBuffer(width, height int) RenderBuffer {
	return RenderBuffer{
		Width:   width,
		Height:  height,
		Covered: make([]bool, width*height),
		Depth:   make([]float64, width*height),
		Normals: make([]Vec3, width*height),
	}
}

func cameraBasisFrom(camera Camera) cameraBasis {
	front := camera.Target.Sub(camera.Position).Normalize()
	right := front.Cross(camera.Up).Normalize()
	up := right.Cross(front).Normalize()
	return cameraBasis{
		origin: camera.Position,
		right:  right,
		up:     up,
		front:  front,
		zoom:   camera.Zoom,
	}
}

func (basis cameraBasis) project(point Vec3) (float64, float64, float64) {
	relative := point.Sub(basis.origin)
	x := float64(0.5) + relative.Dot(basis.right)*basis.zoom*0.5
	y := float64(0.5) + relative.Dot(basis.up)*basis.zoom*0.5
	depth := relative.Dot(basis.front)
	return x, y, depth
}

func calculateVertexNormals(mesh *Mesh) []Vec3 {
	normals := make([]Vec3, len(mesh.Vertices))
	for _, triangle := range mesh.Triangles {
		if !triangle.Active {
			continue
		}
		points := [3]Vec3{
			mesh.Vertices[triangle.V0],
			mesh.Vertices[triangle.V1],
			mesh.Vertices[triangle.V2],
		}
		normal := triangleNormal(points)
		normals[triangle.V0] = normals[triangle.V0].Add(normal)
		normals[triangle.V1] = normals[triangle.V1].Add(normal)
		normals[triangle.V2] = normals[triangle.V2].Add(normal)
	}
	for index, normal := range normals {
		normals[index] = normal.Normalize()
	}
	return normals
}

func renderMesh(mesh *Mesh, camera Camera) RenderBuffer {
	buffer := newRenderBuffer(camera.Width, camera.Height)
	basis := cameraBasisFrom(camera)
	normals := calculateVertexNormals(mesh)

	for _, triangle := range mesh.Triangles {
		if !triangle.Active {
			continue
		}
		vertices := [3]int{triangle.V0, triangle.V1, triangle.V2}
		var px, py, depth [3]float64
		var vertexNormals [3]Vec3
		for i, vertex := range vertices {
			px[i], py[i], depth[i] = basis.project(mesh.Vertices[vertex])
			vertexNormals[i] = normals[vertex]
		}
		rasterTriangle(buffer, camera, px, py, depth, vertexNormals)
	}

	if camera.Occluder {
		applyOccluder(buffer, basis)
	}
	return buffer
}

func rasterTriangle(buffer RenderBuffer, camera Camera, px, py, depth [3]float64, vertexNormals [3]Vec3) {
	minX := maxInt(0, int(math.Floor(math.Min(math.Min(px[0], px[1]), px[2]))*float64(camera.Width)))
	maxX := minInt(camera.Width-1, int(math.Ceil(math.Max(math.Max(px[0], px[1]), px[2]))*float64(camera.Width)))
	minY := maxInt(0, int(math.Floor(math.Min(math.Min(py[0], py[1]), py[2]))*float64(camera.Height)))
	maxY := minInt(camera.Height-1, int(math.Ceil(math.Max(math.Max(py[0], py[1]), py[2]))*float64(camera.Height)))

	area := (px[1]-px[0])*(py[2]-py[0]) - (py[1]-py[0])*(px[2]-px[0])
	if math.Abs(area) < 1e-12 {
		return
	}

	for pixelY := minY; pixelY <= maxY; pixelY++ {
		for pixelX := minX; pixelX <= maxX; pixelX++ {
			sampleX := (float64(pixelX) + 0.5) / float64(camera.Width)
			sampleY := (float64(pixelY) + 0.5) / float64(camera.Height)
			w0 := edgeCross(px, py, 1, 2, sampleX, sampleY) / area
			w1 := edgeCross(px, py, 2, 0, sampleX, sampleY) / area
			w2 := edgeCross(px, py, 0, 1, sampleX, sampleY) / area
			if w0 < 0 || w1 < 0 || w2 < 0 {
				continue
			}

			pixelDepth := w0*depth[0] + w1*depth[1] + w2*depth[2]
			index := pixelY*camera.Width + pixelX
			if buffer.Covered[index] && pixelDepth >= buffer.Depth[index] {
				continue
			}

			normal := vertexNormals[0].Scale(w0).
				Add(vertexNormals[1].Scale(w1)).
				Add(vertexNormals[2].Scale(w2)).
				Normalize()
			buffer.Covered[index] = true
			buffer.Depth[index] = pixelDepth
			buffer.Normals[index] = normal
		}
	}
}

func edgeCross(px, py [3]float64, a, b int, x, y float64) float64 {
	return (px[b]-px[a])*(y-py[a]) - (py[b]-py[a])*(x-px[a])
}

func applyOccluder(buffer RenderBuffer, basis cameraBasis) {
	cx, cy, _ := basis.project(Vec3{})
	radiusX := 0.18 * basis.zoom * 0.5
	radiusY := radiusX * float64(buffer.Width) / float64(buffer.Height)
	for pixelY := 0; pixelY < buffer.Height; pixelY++ {
		for pixelX := 0; pixelX < buffer.Width; pixelX++ {
			x := (float64(pixelX)+0.5)/float64(buffer.Width) - cx
			y := (float64(pixelY)+0.5)/float64(buffer.Height) - cy
			if (x*x)/(radiusX*radiusX)+(y*y)/(radiusY*radiusY) <= 1 {
				index := pixelY*buffer.Width + pixelX
				buffer.Covered[index] = false
				buffer.Depth[index] = 0
				buffer.Normals[index] = Vec3{}
			}
		}
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
