package main

type Quadric [4][4]float64

func planeQuadric(a, b, c Vec3) Quadric {
	normal := b.Sub(a).Cross(c.Sub(a)).Normalize()
	d := -normal.Dot(a)
	plane := [4]float64{normal.X, normal.Y, normal.Z, d}
	var q Quadric
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			q[i][j] = plane[i] * plane[j]
		}
	}
	return q
}

func (q Quadric) Add(o Quadric) Quadric {
	var result Quadric
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			result[i][j] = q[i][j] + o[i][j]
		}
	}
	return result
}

func (q Quadric) Evaluate(point Vec3) float64 {
	p := [4]float64{point.X, point.Y, point.Z, 1}
	var result float64
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			result += p[i] * q[i][j] * p[j]
		}
	}
	return result
}

func calculateQuadrics(mesh *Mesh) []Quadric {
	quadrics := make([]Quadric, len(mesh.Vertices))
	for _, triangle := range mesh.Triangles {
		if !triangle.Active {
			continue
		}
		q := planeQuadric(mesh.Vertices[triangle.V0], mesh.Vertices[triangle.V1], mesh.Vertices[triangle.V2])
		quadrics[triangle.V0] = quadrics[triangle.V0].Add(q)
		quadrics[triangle.V1] = quadrics[triangle.V1].Add(q)
		quadrics[triangle.V2] = quadrics[triangle.V2].Add(q)
	}
	return quadrics
}

type edgeKey struct {
	a, b int
}

func makeEdgeKey(a, b int) edgeKey {
	if a > b {
		a, b = b, a
	}
	return edgeKey{a, b}
}

type collapseEdge struct {
	key    edgeKey
	cost   float64
	target Vec3
	faces  []int
}
