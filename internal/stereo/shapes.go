package stereo

import "math"

func addGrid(mesh *Mesh, point func(u, v float64) Vec3, cols, rows int) {
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			u0 := float64(col) / float64(cols)
			u1 := float64(col+1) / float64(cols)
			v0 := float64(row) / float64(rows)
			v1 := float64(row+1) / float64(rows)
			p00 := point(u0, v0)
			p10 := point(u1, v0)
			p11 := point(u1, v1)
			p01 := point(u0, v1)
			mesh.Triangles = append(mesh.Triangles, [3]Vec3{p00, p10, p11})
			mesh.Triangles = append(mesh.Triangles, [3]Vec3{p00, p11, p01})
		}
	}
}

func addCylinder(mesh *Mesh, start, end Vec3, radius float64, sides int) {
	axis := end.Sub(start)
	length := axis.Len()
	if sides < 3 || length == 0 || radius <= 0 {
		return
	}
	axis = axis.Mul(1 / length)
	reference := Vec3{0, 1, 0}
	if math.Abs(axis.Dot(reference)) > 0.95 {
		reference = Vec3{1, 0, 0}
	}
	u := axis.Cross(reference).Normalized()
	v := axis.Cross(u).Normalized()

	ring := func(center Vec3) []Vec3 {
		pts := make([]Vec3, sides)
		for i := range pts {
			a := 2 * math.Pi * float64(i) / float64(sides)
			pts[i] = center.Add(u.Mul(math.Cos(a) * radius)).Add(v.Mul(math.Sin(a) * radius))
		}
		return pts
	}
	a := ring(start)
	b := ring(end)
	for i := 0; i < sides; i++ {
		j := (i + 1) % sides
		mesh.Triangles = append(mesh.Triangles, [3]Vec3{a[i], b[i], b[j]})
		mesh.Triangles = append(mesh.Triangles, [3]Vec3{a[i], b[j], a[j]})
		mesh.Triangles = append(mesh.Triangles, [3]Vec3{start, a[j], a[i]})
		mesh.Triangles = append(mesh.Triangles, [3]Vec3{end, b[i], b[j]})
	}
}
