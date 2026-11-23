package calib

// CenterKind names one of the three centers that must not be
// confused with one another.
type CenterKind int

const (
	PanelCenter CenterKind = iota
	LensCenter
	RenderCenter
)

func (k CenterKind) String() string {
	switch k {
	case PanelCenter:
		return "panel"
	case LensCenter:
		return "lens"
	default:
		return "render"
	}
}

// CenterKinds lists all three centers.
func CenterKinds() [3]CenterKind {
	return [3]CenterKind{PanelCenter, LensCenter, RenderCenter}
}

func (r Rig) centerFor(k CenterKind, e Eye) Vec2 {
	switch k {
	case PanelCenter:
		return r.Centers.Panel
	case LensCenter:
		return r.Centers.Lens[e]
	default:
		return r.Centers.Render
	}
}

// ConfusionError returns the RMS pixel error across the given points
// that results from treating center `assumed` as center `actual`.
// Any nonzero value means the two centers are distinguishable and
// confusing them is a calibration error.
func (r Rig) ConfusionError(points []Vec3, e Eye, assumed, actual CenterKind) float64 {
	res := make([]Residual, len(points))
	for i, p := range points {
		res[i] = Residual{
			Ideal:    r.Project(p, e, r.centerFor(actual, e)),
			Observed: r.Project(p, e, r.centerFor(assumed, e)),
		}
	}
	return RMS(res)
}

// FaultKind classifies a perturbation after translation fitting.
type FaultKind int

const (
	None FaultKind = iota
	PureTranslation
	Rotation
	EyeSpecificBias
)

func (k FaultKind) String() string {
	switch k {
	case None:
		return "none"
	case PureTranslation:
		return "pure-translation"
	case Rotation:
		return "rotation-or-viewpoint"
	default:
		return "eye-specific-bias"
	}
}

// Diagnosis summarizes what the marker residuals say about a
// perturbation.
type Diagnosis struct {
	Kind FaultKind
	// SharedShift is the best single translation for both eyes.
	SharedShift Vec2
	// SharedLeftover is the RMS residual after SharedShift; large
	// means translation alone cannot explain the error.
	SharedLeftover float64
	// PerEyeLeftover is the RMS residual after fitting each eye
	// independently; large means the fault is not any translation.
	PerEyeLeftover [2]float64
	// Disparity is the binocular disparity error in pixels.
	Disparity float64
}

// Diagnose fits translations and classifies the fault. tol is the
// pixel tolerance below which a fit is considered exact.
func (r Rig) Diagnose(points []Vec3, pert Perturbation, tol float64) Diagnosis {
	var d Diagnosis
	var combined []Residual
	perEye := make(map[Eye][]Residual, 2)
	for _, e := range Eyes() {
		perEye[e] = r.ObserveAll(points, e, pert)
		combined = append(combined, perEye[e]...)
	}
	d.SharedShift, d.SharedLeftover = FitTranslation(combined)
	d.Disparity = DisparityError(r, points, pert)
	allTranslation := true
	for _, e := range Eyes() {
		_, leftover := FitTranslation(perEye[e])
		d.PerEyeLeftover[e] = leftover
		if leftover > tol {
			allTranslation = false
		}
	}
	switch {
	case d.SharedLeftover <= tol && d.Disparity <= tol:
		d.Kind = PureTranslation
	case allTranslation:
		d.Kind = EyeSpecificBias
	default:
		d.Kind = Rotation
	}
	if d.SharedLeftover <= tol && d.Disparity <= tol &&
		d.SharedShift.Len() <= tol {
		d.Kind = None
	}
	return d
}
