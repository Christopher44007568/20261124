package checker

import "math"

// TestSize is the benchmark frame size; it is deliberately even so the 2:2
// lattice is exercised exactly, while the center rectangle uses odd insets so
// its edges cut through both checker phases.
const TestSize = 160

// TestCenter is the fully-shaded per-pixel center region.
var TestCenter = Rect{X0: 52, Y0: 52, X1: 108, Y1: 108}

// Metrics summarizes reconstruction quality over the peripheral pixels.
type Metrics struct {
	// LumaRMSE / LumaMAE measure brightness error.
	LumaRMSE, LumaMAE float64
	// HueMAE is the mean angular hue error (radians) on chromatic pixels.
	HueMAE float64
	// CoverageMAE is the foreground-mask disagreement rate over the whole
	// periphery; CoverageDice is the mask overlap (1 = perfect coverage).
	CoverageMAE, CoverageDice float64
	// Bias is the mean signed luma error normalized by RMSE. It detects a
	// systematic lightening/darkening drift; a purely alternating checker
	// artifact has near-zero bias and is caught by PhaseDelta instead.
	Bias float64
	// CheckerEnergy is the normalized correlation of the error with the sign
	// pattern of the 2:2 checkerboard lattice (the sign itself is zero-mean).
	// It is a cross-check for the primary periodicity probe, PhaseDelta.
	CheckerEnergy float64
	// PeriodAC is the normalized autocorrelation of the reconstruction error
	// at the 2:2 checkerboard block period (a four-pixel shift along each
	// axis). A fixed checkerboard artifact repeats at exactly these lags, so a
	// periodic reconstruction scores close to one while an aperiodic one stays
	// near zero.
	PeriodAC float64
	// PhaseDelta is the mean absolute luma difference between reconstructions
	// of the two complementary shading phases.
	PhaseDelta float64
	// CenterRMSE must be exactly zero: center pixels are shaded per-pixel and
	// the peripheral rule may never touch them.
	CenterRMSE float64
}

// Evaluate compares a reconstruction against ground truth on a scene. The
// foreground test is scene-specific so the coverage metric is not tied to a
// single brightness threshold.
func Evaluate(truth, recon *Image, mask *Mask, isForeground func(RGB) bool) Metrics {
	var m Metrics
	var se, ae, signedSum, n float64
	var hueSum, hueN float64
	var covErr, covInter, covUnion float64
	var ceNum, ceDen float64
	var pn float64

	for y := 0; y < truth.H; y++ {
		for x := 0; x < truth.W; x++ {
			idx := y*truth.W + x
			if mask.InCenter[idx] {
				d := Luma(truth.Px[idx]) - Luma(recon.Px[idx])
				m.CenterRMSE += d * d
				continue
			}
			t := truth.Px[idx]
			r := recon.Px[idx]
			lt, lr := Luma(t), Luma(r)

			// Coverage is evaluated on every peripheral pixel.
			tf := 0.0
			if isForeground(t) {
				tf = 1
			}
			rf := 0.0
			if isForeground(r) {
				rf = 1
			}
			covErr += math.Abs(tf - rf)
			covInter += tf * rf
			covUnion += math.Max(tf, rf)
			pn++

			if mask.Shaded[idx] {
				continue
			}
			d := lr - lt
			se += d * d
			ae += math.Abs(d)
			signedSum += d
			n++

			ht, ct := Hue(t)
			hr, cr := Hue(r)
			if ct > 0.25 && cr > 0.25 {
				hueSum += HueDistance(ht, hr)
				hueN++
			}

			// The probe sign alternates with the actual 2:2 checker lattice.
			sign := 1.0
			if !phase0Shaded(x, y) {
				sign = -1.0
			}
			ceNum += sign * d
			ceDen += d * d
		}
	}

	if n > 0 {
		m.LumaRMSE = math.Sqrt(se / n)
		m.LumaMAE = ae / n
		m.Bias = (signedSum / n) / math.Max(m.LumaRMSE, 1e-9)
		m.CheckerEnergy = math.Abs(ceNum/n) / math.Max(math.Sqrt(ceDen/n), 1e-9)
	}
	if hueN > 0 {
		m.HueMAE = hueSum / hueN
	}
	if pn > 0 {
		m.CoverageMAE = covErr / pn
	}
	if covInter > 0 || covUnion > 0 {
		m.CoverageDice = 2 * covInter / (2*covInter + (covUnion - covInter))
	}
	m.CenterRMSE = math.Sqrt(m.CenterRMSE)
	return m
}

// ErrorAutocorrelation computes the normalized autocorrelation of the
// peripheral reconstruction error at pixel lag (lx,ly). Shaded and center
// pixels are excluded so the statistic describes only the reconstructed
// samples the pipeline is responsible for.
func ErrorAutocorrelation(truth, recon *Image, mask *Mask, lx, ly int) float64 {
	var num, d0, d1 float64
	for y := 0; y < truth.H; y++ {
		for x := 0; x < truth.W; x++ {
			x2, y2 := x+lx, y+ly
			if x2 < 0 || y2 < 0 || x2 >= truth.W || y2 >= truth.H {
				continue
			}
			i1 := y*truth.W + x
			i2 := y2*truth.W + x2
			if mask.Shaded[i1] || mask.Shaded[i2] || mask.InCenter[i1] || mask.InCenter[i2] {
				continue
			}
			e1 := Luma(recon.Px[i1]) - Luma(truth.Px[i1])
			e2 := Luma(recon.Px[i2]) - Luma(truth.Px[i2])
			num += e1 * e2
			d0 += e1 * e1
			d1 += e2 * e2
		}
	}
	return num / math.Sqrt(d0*d1+1e-12)
}

// ReconstructNaive is the deliberately-broken baseline: every skipped pixel
// copies its nearest shaded neighbor. It locks a fixed checkerboard period into
// the result and must fail every periodicity gate.
func ReconstructNaive(shaded *Image, mask *Mask) *Image {
	out := shaded.Clone()
	for y := 0; y < out.H; y++ {
		for x := 0; x < out.W; x++ {
			idx := y*out.W + x
			if mask.Shaded[idx] {
				continue
			}
			if c, ok := nearestShadedColor(out, mask, x, y); ok {
				out.Px[idx] = c
			}
		}
	}
	return out
}

// RunScene renders one scene under both shading phases and returns the
// phase-zero metrics plus the two reconstructions for inspection.
func RunScene(sc Scene) (Metrics, *Image, *Image, *Mask) {
	truth := sc.Render(TestSize, TestSize)
	mask0 := BuildMask(TestSize, TestSize, TestCenter, 0)
	mask1 := BuildMask(TestSize, TestSize, TestCenter, 1)
	rec0 := Reconstruct(Shade(truth, mask0), mask0)
	rec1 := Reconstruct(Shade(truth, mask1), mask1)

	m := Evaluate(truth, rec0, mask0, sc.Foreground)
	var delta, dn float64
	for i := range truth.Px {
		if mask0.InCenter[i] {
			continue
		}
		delta += math.Abs(Luma(rec0.Px[i]) - Luma(rec1.Px[i]))
		dn++
	}
	if dn > 0 {
		m.PhaseDelta = delta / dn
	}
	m.PeriodAC = math.Max(
		math.Abs(ErrorAutocorrelation(truth, rec0, mask0, 4, 0)),
		math.Abs(ErrorAutocorrelation(truth, rec0, mask0, 0, 4)),
	)
	return m, rec0, truth, mask0
}
