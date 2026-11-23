package offaxis

// det3 returns the determinant of a 3x3 matrix given as nine row-major values.
func det3(a, b, c, d, e, f, g, h, i float64) float64 {
	return a*(e*i-f*h) - b*(d*i-f*g) + c*(d*h-e*g)
}

// Determinant returns det(m).
func (m Mat4) Determinant() float64 {
	return m[0][0]*det3(m[1][1], m[1][2], m[1][3], m[2][1], m[2][2], m[2][3], m[3][1], m[3][2], m[3][3]) -
		m[0][1]*det3(m[1][0], m[1][2], m[1][3], m[2][0], m[2][2], m[2][3], m[3][0], m[3][2], m[3][3]) +
		m[0][2]*det3(m[1][0], m[1][1], m[1][3], m[2][0], m[2][1], m[2][3], m[3][0], m[3][1], m[3][3]) -
		m[0][3]*det3(m[1][0], m[1][1], m[1][2], m[2][0], m[2][1], m[2][2], m[3][0], m[3][1], m[3][2])
}

// Inverse returns m^-1. It returns false when m is singular.
func (m Mat4) Inverse() (Mat4, bool) {
	var inv Mat4
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			// Minor: delete row c and column r (transpose of cofactors).
			rows := make([]int, 0, 3)
			cols := make([]int, 0, 3)
			for i := 0; i < 4; i++ {
				if i != c {
					rows = append(rows, i)
				}
				if i != r {
					cols = append(cols, i)
				}
			}
			minor := det3(
				m[rows[0]][cols[0]], m[rows[0]][cols[1]], m[rows[0]][cols[2]],
				m[rows[1]][cols[0]], m[rows[1]][cols[1]], m[rows[1]][cols[2]],
				m[rows[2]][cols[0]], m[rows[2]][cols[1]], m[rows[2]][cols[2]],
			)
			sign := 1.0
			if (r+c)%2 == 1 {
				sign = -1
			}
			inv[r][c] = sign * minor
		}
	}
	det := m.Determinant()
	if det == 0 {
		return Mat4{}, false
	}
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			inv[r][c] /= det
		}
	}
	return inv, true
}

// Unproject maps an NDC point back to an eye-space point at the given
// eye-space depth (-z, i.e. a positive distance in front of the camera).
// It returns false when the projection matrix is singular.
func (m Mat4) Unproject(ndcX, ndcY, eyeDepth float64) (Vec3, bool) {
	inv, ok := m.Inverse()
	if !ok {
		return Vec3{}, false
	}
	// Find w such that inv * (ndcX, ndcY, ndcZ, 1) * w lands on z = -depth.
	// Easier: pick the point at eye depth directly by reconstructing from
	// the inverse mapping of the ray and scaling to the requested plane.
	// The inverse maps (x, y, z, 1) to homogeneous eye coordinates; take
	// the z=0..-1 endpoints of the NDC z range to define the ray.
	near := inv.Transform(Vec4{X: ndcX, Y: ndcY, Z: -1, W: 1})
	far := inv.Transform(Vec4{X: ndcX, Y: ndcY, Z: 1, W: 1})
	if near.W == 0 || far.W == 0 {
		return Vec3{}, false
	}
	p0 := Vec3{near.X / near.W, near.Y / near.W, near.Z / near.W}
	p1 := Vec3{far.X / far.W, far.Y / far.W, far.Z / far.W}
	dir := p1.Sub(p0)
	if dir.Z == 0 {
		return Vec3{}, false
	}
	t := (-eyeDepth - p0.Z) / dir.Z
	return p0.Add(dir.Scale(t)), true
}
