package offaxis

// StereoMatrix bundles the per-eye projection with the head-space view
// matrix so callers can feed both straight into the rasterizer.
type StereoMatrix struct {
	Eye        Eye
	Projection Mat4
	View       Mat4
}

// BuildStereo validates both eyes and rejects pairs whose center rays are
// not parallel in head space (a toed-in / converging setup) or whose depth
// ranges differ. Parallel-axis asymmetric frusta are the only supported
// stereo configuration because they keep vertical parallax at zero.
func BuildStereo(left, right Eye) (StereoMatrix, StereoMatrix, error) {
	lp, err := left.Bounds.ProjectionMatrix()
	if err != nil {
		return StereoMatrix{}, StereoMatrix{}, err
	}
	rp, err := right.Bounds.ProjectionMatrix()
	if err != nil {
		return StereoMatrix{}, StereoMatrix{}, err
	}
	if !sameDepthRange(left.Bounds, right.Bounds) {
		return StereoMatrix{}, StereoMatrix{}, ErrCenterRay
	}
	if !parallelCenterRays(left, right) {
		return StereoMatrix{}, StereoMatrix{}, ErrCenterRay
	}
	return StereoMatrix{Eye: left, Projection: lp, View: viewFromPose(left.Pose)},
		StereoMatrix{Eye: right, Projection: rp, View: viewFromPose(right.Pose)},
		nil
}

func sameDepthRange(a, b Bounds) bool {
	return approx(a.Near, b.Near, validationEps) && approx(a.Far, b.Far, validationEps)
}

// parallelCenterRays maps each eye-space center ray into head space and
// requires the directions to match. Pure eye translation (the normal HMD
// case) passes; any relative yaw fails.
func parallelCenterRays(left, right Eye) bool {
	lDir := headSpaceDirection(left.Pose, centerRayDirection(left.Bounds))
	rDir := headSpaceDirection(right.Pose, centerRayDirection(right.Bounds))
	dot := lDir.Normalize().Dot(rDir.Normalize())
	return approx(dot, 1, 1e-8)
}

// centerRayDirection returns the eye-space direction from the apex through
// the near-plane center. For a valid frustum this is (cx/n, cy/n, -1)
// normalized.
func centerRayDirection(b Bounds) Vec3 {
	cx := (b.Left + b.Right) / 2
	cy := (b.Bottom + b.Top) / 2
	return Vec3{X: cx / b.Near, Y: cy / b.Near, Z: -1}.Normalize()
}

func headSpaceDirection(p Pose, dir Vec3) Vec3 {
	if isIdentity(p.Rotation) {
		return dir
	}
	v := p.Rotation.Transform(Vec4{X: dir.X, Y: dir.Y, Z: dir.Z, W: 0})
	return Vec3{X: v.X, Y: v.Y, Z: v.Z}
}

// viewFromPose builds V = R^T * T(-position). For the identity pose this is
// simply the identity matrix, which keeps the single-eye demos trivial.
func viewFromPose(p Pose) Mat4 {
	translation := Identity()
	translation[0][3] = -p.Position.X
	translation[1][3] = -p.Position.Y
	translation[2][3] = -p.Position.Z
	if isIdentity(p.Rotation) {
		return translation
	}
	rot, ok := p.Rotation.Inverse()
	if !ok {
		return Identity()
	}
	return Mul(rot, translation)
}

func isIdentity(m Mat4) bool {
	if m == (Mat4{}) {
		return true
	}
	id := Identity()
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			if m[r][c] != id[r][c] {
				return false
			}
		}
	}
	return true
}
