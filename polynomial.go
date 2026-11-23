package lensdistortion

import "math"

// PolynomialMap is a direct radial polynomial approximation of the
// display-to-source transfer:
//
//	rho = r * (1 + C1*r^2 + C2*r^4 + ...)
//
// r is the normalized display radius and rho is the normalized raw-source
// radius. Unlike AnalyticMap, the Coefficients field of Lens is not used here;
// only its center, reference radius and MaxRadius frame are retained. This
// keeps a fitted polynomial independent from the calibration model.
type PolynomialMap struct {
	lens         Lens
	coefficients []float64
}

func NewPolynomialMap(lens Lens, coefficients []float64) (*PolynomialMap, error) {
	if err := lens.validate(); err != nil {
		return nil, ErrInvalidPolynomial
	}
	for _, coefficient := range coefficients {
		if !isFinite(coefficient) {
			return nil, ErrInvalidPolynomial
		}
	}
	if err := validateMonotonic(coefficients, lens.MaxRadius); err != nil {
		return nil, err
	}
	lens.Coefficients = nil
	return &PolynomialMap{
		lens:         lens,
		coefficients: append([]float64(nil), coefficients...),
	}, nil
}

// NewPolynomialFromLens uses the lens model coefficients as a truncated
// radial transfer polynomial. MaxRadius should normally be set on the lens.
func NewPolynomialFromLens(lens Lens) (*PolynomialMap, error) {
	return NewPolynomialMap(lens, lens.Coefficients)
}

func (m *PolynomialMap) sourceRadius(r float64) (rho float64, ok bool) {
	if !isFinite(r) || r < 0 {
		return 0, false
	}
	if m.lens.MaxRadius > 0 && r > m.lens.MaxRadius*(1+1e-14) {
		return 0, false
	}
	rho, _ = radialSourceRadius(m.coefficients, r)
	return rho, isFinite(rho) && rho >= 0
}

func (m *PolynomialMap) SourceFromDisplay(display Point) (Point, bool) {
	if !display.IsFinite() {
		return Point{}, false
	}
	delta := display.Sub(m.lens.Center)
	r := delta.Length() / m.lens.ReferenceRadius
	if r == 0 {
		return m.lens.Center, true
	}
	rho, ok := m.sourceRadius(r)
	if !ok {
		return Point{}, false
	}
	return m.lens.Center.Add(delta.Mul(rho / r)), true
}

func (m *PolynomialMap) DisplayFromSource(source Point) (Point, bool) {
	if !source.IsFinite() {
		return Point{}, false
	}
	delta := source.Sub(m.lens.Center)
	rho := delta.Length() / m.lens.ReferenceRadius
	if rho == 0 {
		return m.lens.Center, true
	}
	r, err := invertRadialRadius(m.coefficients, rho, m.lens.MaxRadius)
	if err != nil {
		return Point{}, false
	}
	return m.lens.Center.Add(delta.Mul(r / rho)), true
}

func (m *PolynomialMap) Center() Point { return m.lens.Center }
func (m *PolynomialMap) Kind() MapKind { return KindPolynomial }

func (m *PolynomialMap) Coefficients() []float64 {
	return append([]float64(nil), m.coefficients...)
}

// maxPolynomialRadius returns the largest display radius covered by the map.
// It is used when fitting a polynomial against a full analytic model.
func maxPolynomialRadius(lens Lens, fallback float64) float64 {
	if lens.MaxRadius > 0 {
		return lens.MaxRadius
	}
	if isFinite(fallback) && fallback > 0 {
		return fallback
	}
	return math.Sqrt2
}
