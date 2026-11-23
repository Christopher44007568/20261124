package raster

import "stereowarp/geom"

// Material describes a flat-shaded surface. Alpha below 255 marks it glass.
type Material struct {
	Color RGBA
}

// Transparent reports whether the material needs back-to-front alpha blending.
func (m Material) Transparent() bool { return m.Color.A < 255 }

// Mesh is a triangle list with one shared material.
type Mesh struct {
	Name      string
	Vertices  []geom.Vec3
	Triangles [][3]int
	Material  Material
}

// Quad builds a two-triangle quad from four corners in order around its edge.
func Quad(name string, a, b, c, d geom.Vec3, mat Material) *Mesh {
	return &Mesh{
		Name:      name,
		Vertices:  []geom.Vec3{a, b, c, d},
		Triangles: [][3]int{{0, 1, 2}, {0, 2, 3}},
		Material:  mat,
	}
}

// Box builds an axis-aligned cuboid from its min/max corners.
func Box(name string, min, max geom.Vec3, mat Material) *Mesh {
	x0, y0, z0 := min.X, min.Y, min.Z
	x1, y1, z1 := max.X, max.Y, max.Z
	v := []geom.Vec3{
		geom.V(x0, y0, z0), geom.V(x1, y0, z0), geom.V(x1, y1, z0), geom.V(x0, y1, z0),
		geom.V(x0, y0, z1), geom.V(x1, y0, z1), geom.V(x1, y1, z1), geom.V(x0, y1, z1),
	}
	tris := [][3]int{
		{0, 3, 2}, {0, 2, 1}, // back (-Z)
		{4, 5, 6}, {4, 6, 7}, // front (+Z)
		{0, 1, 5}, {0, 5, 4}, // bottom
		{3, 7, 6}, {3, 6, 2}, // top
		{0, 4, 7}, {0, 7, 3}, // left
		{1, 2, 6}, {1, 6, 5}, // right
	}
	return &Mesh{Name: name, Vertices: v, Triangles: tris, Material: mat}
}
