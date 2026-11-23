package lensdistortion

import "math"

// MapKind identifies how a display-to-source mapping is evaluated.
type MapKind string

const (
	KindAnalytic   MapKind = "analytic"
	KindPolynomial MapKind = "polynomial"
	KindLUT        MapKind = "lut"
)

// Mapper is an inverse-warp lookup. SourceFromDisplay is the direction used
// while rendering a corrected image: for each distortion-free output pixel it
// returns the raw source coordinate to sample. DisplayFromSource is the
// opposite direction and is useful for projection and verification.
//
// A false return means the point is outside the mapper's calibrated domain.
// Implementations must not silently extrapolate.
type Mapper interface {
	SourceFromDisplay(display Point) (Point, bool)
	DisplayFromSource(source Point) (Point, bool)
	Center() Point
	Kind() MapKind
}

// Lens is a Brown-Conrady style radial lens model.
//
// For an undistorted point p, let u = |p-Center|/ReferenceRadius and
// f(u) = 1 + K1*u^2 + K2*u^4 + ... . The raw distorted source point is
//
//	Center + (p-Center)*f(u).
//
// Negative K1 therefore describes barrel distortion and positive K1 describes
// pincushion distortion. Coefficients are expressed in normalized-radius
// units, not pixels. MaxRadius is the largest accepted normalized display
// radius; zero means no explicit cap. For a corner at normalized radius
// sqrt(2), MaxRadius must be at least sqrt(2).
type Lens struct {
	Center          Point
	ReferenceRadius float64
	Coefficients    []float64
	MaxRadius       float64
}

func (l Lens) validate() error {
	if !l.Center.IsFinite() || !isFinite(l.ReferenceRadius) ||
		l.ReferenceRadius <= 0 || !isFinite(l.MaxRadius) || l.MaxRadius < 0 {
		return ErrInvalidLens
	}
	for _, coefficient := range l.Coefficients {
		if !isFinite(coefficient) {
			return ErrInvalidLens
		}
	}
	return nil
}

// NormalizedRadius converts a distance from the distortion center to the
// coefficient domain.
func (l Lens) NormalizedRadius(p Point) float64 {
	return distance(p, l.Center) / l.ReferenceRadius
}

// PointAtNormalizedRadius returns the point on the positive X ray from the
// distortion center. It is useful for building radial lookup tables.
func (l Lens) PointAtNormalizedRadius(r float64) Point {
	return Point{
		X: l.Center.X + r*l.ReferenceRadius,
		Y: l.Center.Y,
	}
}

// DistortedFromUndistorted applies the forward radial model.
func (l Lens) DistortedFromUndistorted(display Point) (Point, error) {
	if err := l.validate(); err != nil {
		return Point{}, err
	}
	return l.distortedFromUndistorted(display)
}

func (l Lens) distortedFromUndistorted(display Point) (Point, error) {
	if !display.IsFinite() {
		return Point{}, ErrInvalidPoint
	}
	delta := display.Sub(l.Center)
	r := delta.Length() / l.ReferenceRadius
	if l.MaxRadius > 0 && r > l.MaxRadius*(1+1e-14) {
		return Point{}, ErrOutOfDomain
	}
	if r == 0 {
		return l.Center, nil
	}
	factor, _ := radialFactor(l.Coefficients, r)
	if !isFinite(factor) {
		return Point{}, ErrOutOfDomain
	}
	return l.Center.Add(delta.Mul(factor)), nil
}

// UndistortedFromDistorted analytically inverts the radial model by solving
// u*f(u) = source normalized radius. Newton steps are bracketed with bisection,
// so a locally non-monotonic calibration fails instead of returning a
// plausible but ambiguous root.
func (l Lens) UndistortedFromDistorted(source Point) (Point, error) {
	if err := l.validate(); err != nil {
		return Point{}, err
	}
	return l.undistortedFromDistorted(source)
}

func (l Lens) undistortedFromDistorted(source Point) (Point, error) {
	if !source.IsFinite() {
		return Point{}, ErrInvalidPoint
	}
	delta := source.Sub(l.Center)
	rho := delta.Length() / l.ReferenceRadius
	if rho == 0 {
		return l.Center, nil
	}
	r, err := invertRadialRadius(l.Coefficients, rho, l.MaxRadius)
	if err != nil {
		return Point{}, err
	}
	return l.Center.Add(delta.Mul(r / rho)), nil
}

// Forward is an alias for DistortedFromUndistorted.
func (l Lens) Forward(p Point) (Point, error) {
	return l.DistortedFromUndistorted(p)
}

// Inverse is an alias for UndistortedFromDistorted.
func (l Lens) Inverse(p Point) (Point, error) {
	return l.UndistortedFromDistorted(p)
}

func radialFactor(coefficients []float64, r float64) (factor, derivative float64) {
	factor, derivative = 1, 1
	if r == 0 {
		return
	}
	r2 := r * r
	power := r2
	for i, coefficient := range coefficients {
		factor += coefficient * power
		derivative += float64(2*i+3) * coefficient * power
		power *= r2
	}
	return factor, derivative
}

func radialSourceRadius(coefficients []float64, r float64) (rho, dRho float64) {
	factor, derivative := radialFactor(coefficients, r)
	return r * factor, factor + r*derivative
}

func invertRadialRadius(coefficients []float64, rho, maxRadius float64) (float64, error) {
	if !isFinite(rho) || rho < 0 {
		return 0, ErrInvalidPoint
	}
	if rho == 0 {
		return 0, nil
	}
	if !isFinite(maxRadius) || maxRadius < 0 {
		return 0, ErrOutOfDomain
	}
	if maxRadius > 0 {
		limit, _ := radialSourceRadius(coefficients, maxRadius)
		if !isFinite(limit) || rho > limit*(1+1e-12) {
			return 0, ErrOutOfDomain
		}
	}

	hi := rho
	for i := 0; i < 64; i++ {
		h, _ := radialSourceRadius(coefficients, hi)
		if isFinite(h) && h >= rho {
			break
		}
		if maxRadius > 0 && hi >= maxRadius {
			return 0, ErrOutOfDomain
		}
		hi *= 2
		if !isFinite(hi) || hi > 1e12 {
			return 0, ErrNonMonotonic
		}
	}
	if maxRadius > 0 && hi > maxRadius {
		hi = maxRadius
	}

	lo := 0.0
	x := rho
	tolerance := 1e-12 * (1 + rho)
	for i := 0; i < 80; i++ {
		value, slope := radialSourceRadius(coefficients, x)
		residual := value - rho
		if math.Abs(residual) <= tolerance {
			return x, nil
		}
		if residual < 0 {
			lo = x
		} else {
			hi = x
		}
		if hi-lo <= tolerance {
			return (lo + hi) / 2, nil
		}
		if slope <= 0 || !isFinite(slope) {
			return 0, ErrNonMonotonic
		}
		next := x - residual/slope
		if !isFinite(next) || next <= lo || next >= hi {
			next = (lo + hi) / 2
		}
		x = next
	}
	return (lo + hi) / 2, nil
}

func validateMonotonic(coefficients []float64, maxRadius float64) error {
	if maxRadius <= 0 {
		return nil
	}
	const samples = 512
	for i := 0; i <= samples; i++ {
		r := maxRadius * float64(i) / samples
		if _, slope := radialSourceRadius(coefficients, r); slope <= 0 || !isFinite(slope) {
			return ErrNonMonotonic
		}
	}
	return nil
}
