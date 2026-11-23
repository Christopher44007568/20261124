package optics

import (
	"fmt"
	"math"
)

// Point is a normalized 2D coordinate.
type Point struct {
	X float64
	Y float64
}

// RoundtripTolerance is the max allowed error for inverse(forward(p)) checks.
const RoundtripTolerance = 1e-6

// newtonIterations caps the inverse solver.
const newtonIterations = 32

// distortRadius applies the radial polynomial to a unit-offset radius.
func distortRadius(r float64, coeffs []float64) float64 {
	scale := 1.0
	r2 := r * r
	rpow := r2
	for _, k := range coeffs {
		scale += k * rpow
		rpow *= r2
	}
	return r * scale
}

// distortRadiusDerivative is d(r')/dr of distortRadius.
func distortRadiusDerivative(r float64, coeffs []float64) float64 {
	deriv := 1.0
	r2 := r * r
	rpow := r2
	for i, k := range coeffs {
		deriv += float64(2*i+3) * k * rpow
		rpow *= r2
	}
	return deriv
}

// Forward maps an undistorted panel point to its distorted screen point
// for the given color channel (0=R, 1=G, 2=B).
func Forward(eye *EyeParams, channel int, p Point) Point {
	c := eye.PanelCenter
	dx, dy := p.X-c.X, p.Y-c.Y
	r := math.Hypot(dx, dy)
	if r == 0 {
		return Point{X: c.X, Y: c.Y}
	}
	rd := distortRadius(r, eye.Distortion.Coefficients) * eye.ChromaticAberration.Scale[channel]
	s := rd / r
	return Point{X: c.X + dx*s, Y: c.Y + dy*s}
}

// Inverse maps a distorted screen point back to the undistorted panel
// point using Newton iteration on the radial model.
func Inverse(eye *EyeParams, channel int, p Point) (Point, error) {
	c := eye.PanelCenter
	dx, dy := p.X-c.X, p.Y-c.Y
	rd := math.Hypot(dx, dy)
	if rd == 0 {
		return Point{X: c.X, Y: c.Y}, nil
	}
	target := rd / eye.ChromaticAberration.Scale[channel]
	coeffs := eye.Distortion.Coefficients

	r := target // initial guess: identity distortion
	for i := 0; i < newtonIterations; i++ {
		f := distortRadius(r, coeffs) - target
		if math.Abs(f) < 1e-12 {
			break
		}
		d := distortRadiusDerivative(r, coeffs)
		if d == 0 || math.IsNaN(d) || math.IsInf(d, 0) {
			return Point{}, fmt.Errorf("inverse mapping diverged at radius %g", r)
		}
		r -= f / d
		if r < 0 {
			r = 0
		}
	}
	if math.Abs(distortRadius(r, coeffs)-target) > RoundtripTolerance {
		return Point{}, fmt.Errorf("inverse mapping did not converge for radius %g", target)
	}
	s := r / rd
	return Point{X: c.X + dx*s, Y: c.Y + dy*s}, nil
}

// MappingReport summarizes one roundtrip validation run.
type MappingReport struct {
	Samples    int
	MaxError   float64
	WorstPoint Point
}

// CheckRoundtrip samples the safety boundary of each eye and verifies
// that inverse(forward(p)) returns to p within tolerance for all channels.
func CheckRoundtrip(cfg *Config, samplesPerAxis int) (*MappingReport, error) {
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	report := &MappingReport{}
	for _, eye := range []*EyeParams{cfg.Left, cfg.Right} {
		b := eye.SafetyBoundary
		for channel := 0; channel < 3; channel++ {
			for i := 0; i < samplesPerAxis; i++ {
				for j := 0; j < samplesPerAxis; j++ {
					p := Point{
						X: b.MinX + (b.MaxX-b.MinX)*float64(i)/float64(samplesPerAxis-1),
						Y: b.MinY + (b.MaxY-b.MinY)*float64(j)/float64(samplesPerAxis-1),
					}
					back, err := Inverse(eye, channel, Forward(eye, channel, p))
					if err != nil {
						return nil, fmt.Errorf("roundtrip failed at %+v: %w", p, err)
					}
					e := math.Hypot(back.X-p.X, back.Y-p.Y)
					if e > report.MaxError {
						report.MaxError = e
						report.WorstPoint = p
					}
					report.Samples++
					if e > RoundtripTolerance {
						return nil, fmt.Errorf("roundtrip error %g exceeds tolerance at %+v", e, p)
					}
				}
			}
		}
	}
	return report, nil
}
