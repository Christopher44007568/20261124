package calib

import "math"

// Residual pairs an observed pixel with its ideal pixel.
type Residual struct {
	Ideal    Vec2
	Observed Vec2
}

func (r Residual) delta() Vec2 { return r.Observed.Sub(r.Ideal) }

// ObserveAll collects residuals for one eye over the given points.
func (r Rig) ObserveAll(points []Vec3, e Eye, pert Perturbation) []Residual {
	out := make([]Residual, len(points))
	for i, p := range points {
		out[i] = Residual{Ideal: r.Ideal(p, e), Observed: r.Observe(p, e, pert)}
	}
	return out
}

// RMS returns the root-mean-square residual magnitude in pixels.
func RMS(res []Residual) float64 {
	if len(res) == 0 {
		return 0
	}
	var sum float64
	for _, r := range res {
		d := r.delta()
		sum += d.X*d.X + d.Y*d.Y
	}
	return math.Sqrt(sum / float64(len(res)))
}

// Max returns the largest residual magnitude in pixels.
func Max(res []Residual) float64 {
	var m float64
	for _, r := range res {
		if d := r.delta().Len(); d > m {
			m = d
		}
	}
	return m
}

// FitTranslation finds the single best shared translation for the
// residuals (the mean delta) and reports the RMS left over after
// applying it. A large leftover means the error is not a pure
// translation and shifting the whole image cannot hide it.
func FitTranslation(res []Residual) (shift Vec2, leftoverRMS float64) {
	if len(res) == 0 {
		return Vec2{}, 0
	}
	for _, r := range res {
		shift = shift.Add(r.delta())
	}
	shift = shift.Scale(1 / float64(len(res)))
	var sum float64
	for _, r := range res {
		d := r.delta().Sub(shift)
		sum += d.X*d.X + d.Y*d.Y
	}
	return shift, math.Sqrt(sum / float64(len(res)))
}

// DisparityError compares observed and ideal binocular disparity
// vectors (left minus right, both axes) for matched world points,
// returning the RMS disparity error in pixels.
func DisparityError(rig Rig, points []Vec3, pert Perturbation) float64 {
	var sum float64
	for _, p := range points {
		ideal := rig.Ideal(p, Left).Sub(rig.Ideal(p, Right))
		obs := rig.Observe(p, Left, pert).Sub(rig.Observe(p, Right, pert))
		d := obs.Sub(ideal)
		sum += d.X*d.X + d.Y*d.Y
	}
	if len(points) == 0 {
		return 0
	}
	return math.Sqrt(sum / float64(len(points)))
}

// DepthSensitivity measures how strongly the residual magnitude
// varies with depth: the spread of per-point residual magnitudes
// after removing the best shared translation. Viewpoint (parallax)
// errors scale with 1/z, so this stays large for them.
func DepthSensitivity(res []Residual) float64 {
	_, leftover := FitTranslation(res)
	return leftover
}
