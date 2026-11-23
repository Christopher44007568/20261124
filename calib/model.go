package calib

// Eye identifies one side of a stereo rig.
type Eye int

const (
	Left Eye = iota
	Right
	numEyes
)

func (e Eye) String() string {
	if e == Left {
		return "left"
	}
	return "right"
}

// Eyes returns both eyes in fixed order.
func Eyes() [2]Eye { return [2]Eye{Left, Right} }

// Centers are the three panel-space centers (pixels) that a real
// system must keep distinct. They deliberately do not coincide.
type Centers struct {
	// Panel is the geometric center of the display panel.
	Panel Vec2
	// Lens is the optical (principal point) center per eye.
	Lens [2]Vec2
	// Render is the center the renderer assumes when projecting.
	Render Vec2
}

// Rig describes the stereo camera and panel geometry.
type Rig struct {
	FocalPx float64 // focal length in pixels
	IPD     float64 // interpupillary distance, meters
	PanelW  float64 // panel width in pixels (per eye)
	PanelH  float64 // panel height in pixels (per eye)
	Centers Centers
}

// DefaultRig returns a rig where panel, lens and render centers
// are all slightly different, like a real uncalibrated headset.
func DefaultRig() Rig {
	w, h := 1920.0, 1080.0
	return Rig{
		FocalPx: 1000,
		IPD:     0.064,
		PanelW:  w,
		PanelH:  h,
		Centers: Centers{
			Panel:  Vec2{w / 2, h / 2},
			Lens:   [2]Vec2{{948, 546}, {963, 537}},
			Render: Vec2{955, 542},
		},
	}
}

// EyePos returns the world-space position of an eye (z forward).
func (r Rig) EyePos(e Eye) Vec3 {
	half := r.IPD / 2
	if e == Left {
		return Vec3{-half, 0, 0}
	}
	return Vec3{half, 0, 0}
}

// Project projects a world point for one eye using the given
// principal point, returning panel pixels.
func (r Rig) Project(p Vec3, e Eye, principal Vec2) Vec2 {
	return r.ProjectFrom(p, r.EyePos(e), principal)
}

// ProjectFrom projects a world point from an explicit eye position.
func (r Rig) ProjectFrom(p Vec3, eyePos Vec3, principal Vec2) Vec2 {
	rel := p.Sub(eyePos)
	return Vec2{
		r.FocalPx*rel.X/rel.Z + principal.X,
		r.FocalPx*rel.Y/rel.Z + principal.Y,
	}
}

// Ideal projects with the lens center: the optically correct result.
func (r Rig) Ideal(p Vec3, e Eye) Vec2 {
	return r.Project(p, e, r.Centers.Lens[e])
}

// Perturbation describes everything a miscalibrated compositor can
// do to the rendered image before it lands on the panel.
type Perturbation struct {
	// ViewpointShift moves the assumed eye position in meters,
	// simulating a tracking/IPD error. Unlike a 2D translation its
	// pixel effect scales with 1/z, so known-depth points expose it.
	ViewpointShift [2]Vec3
	// Translation shifts the whole image (both eyes) in pixels.
	Translation Vec2
	// Rotation rotates the image around Pivot, in radians.
	Rotation float64
	// Pivot is the center the rotation is applied about; choosing
	// the wrong center is exactly the failure mode under test.
	Pivot Vec2
	// EyeBias is an extra per-eye offset in pixels.
	EyeBias [2]Vec2
}

// Observe renders p for one eye with the render center, applies the
// compositor perturbation, and finally the calibrated base placement
// (the known render-to-lens offset). With a zero perturbation the
// result equals the lens-centered ideal; anything left over is a
// residual calibration fault.
func (r Rig) Observe(p Vec3, e Eye, pert Perturbation) Vec2 {
	eyePos := r.EyePos(e)
	s := pert.ViewpointShift[e]
	eyePos = Vec3{eyePos.X + s.X, eyePos.Y + s.Y, eyePos.Z + s.Z}
	v := r.ProjectFrom(p, eyePos, r.Centers.Render)
	v = RotateAbout(v, pert.Pivot, pert.Rotation)
	v = v.Add(pert.Translation).Add(pert.EyeBias[e])
	v = v.Add(r.Centers.Lens[e].Sub(r.Centers.Render))
	return v
}
