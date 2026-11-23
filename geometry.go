package main

import "math"

type Vec3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

func (v Vec3) Add(o Vec3) Vec3      { return Vec3{v.X + o.X, v.Y + o.Y, v.Z + o.Z} }
func (v Vec3) Sub(o Vec3) Vec3      { return Vec3{v.X - o.X, v.Y - o.Y, v.Z - o.Z} }
func (v Vec3) Scale(s float64) Vec3 { return Vec3{v.X * s, v.Y * s, v.Z * s} }
func (v Vec3) Dot(o Vec3) float64   { return v.X*o.X + v.Y*o.Y + v.Z*o.Z }

func (v Vec3) Cross(o Vec3) Vec3 {
	return Vec3{
		v.Y*o.Z - v.Z*o.Y,
		v.Z*o.X - v.X*o.Z,
		v.X*o.Y - v.Y*o.X,
	}
}

func (v Vec3) Length() float64 { return math.Sqrt(v.Dot(v)) }

func (v Vec3) Normalize() Vec3 {
	length := v.Length()
	if length < 1e-12 {
		return Vec3{}
	}
	return v.Scale(1 / length)
}

func triangleNormal(points [3]Vec3) Vec3 {
	return points[1].Sub(points[0]).Cross(points[2].Sub(points[0])).Normalize()
}

type Triangle struct {
	V0, V1, V2 int
	Active     bool
}

type Mesh struct {
	Name      string
	Vertices  []Vec3
	Triangles []Triangle
	Boundary  []bool
}

func makeAuditMesh() *Mesh {
	const rows = 21
	const cols = 33
	vertices := make([]Vec3, rows*cols)
	boundary := make([]bool, rows*cols)

	for row := 0; row < rows; row++ {
		u := float64(row) / (rows - 1)
		for col := 0; col < cols; col++ {
			v := float64(col) / (cols - 1)
			index := row*cols + col
			x := 2*u - 1
			y := 2*v - 1
			z := 0.22 * math.Sin(math.Pi*x) * math.Sin(math.Pi*y)
			z += 0.07 * math.Sin(5*math.Pi*u) * math.Sin(3*math.Pi*v) * math.Sin(math.Pi*u) * math.Sin(math.Pi*v)
			vertices[index] = Vec3{x, y, z}
			boundary[index] = row == 0 || row == rows-1 || col == 0 || col == cols-1
		}
	}

	triangles := make([]Triangle, 0, 2*(rows-1)*(cols-1))
	for row := 0; row < rows-1; row++ {
		for col := 0; col < cols-1; col++ {
			a := row*cols + col
			b := row*cols + col + 1
			c := (row+1)*cols + col
			d := (row+1)*cols + col + 1
			triangles = append(triangles, Triangle{V0: a, V1: c, V2: b, Active: true}, Triangle{V0: b, V1: c, V2: d, Active: true})
		}
	}

	return &Mesh{Name: "displaced-grid", Vertices: vertices, Triangles: triangles, Boundary: boundary}
}
