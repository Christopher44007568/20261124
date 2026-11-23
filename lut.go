package lensdistortion

import "sort"

// LUTKnot pairs a normalized display radius with its normalized raw-source
// radius. The source values must increase strictly so the table is invertible.
type LUTKnot struct {
	DisplayRadius float64
	SourceRadius  float64
}

// PiecewiseLUT is a piecewise-linear radial transfer table. It performs no
// extrapolation beyond the last knot in either direction.
type PiecewiseLUT struct {
	center          Point
	referenceRadius float64
	knots           []LUTKnot
}

func NewPiecewiseLUT(center Point, referenceRadius float64, knots []LUTKnot) (*PiecewiseLUT, error) {
	if !center.IsFinite() || !isFinite(referenceRadius) || referenceRadius <= 0 {
		return nil, ErrInvalidLUT
	}
	if len(knots) < 2 {
		return nil, ErrInvalidLUT
	}
	previousDisplay, previousSource := -1.0, -1.0
	for i, knot := range knots {
		if !isFinite(knot.DisplayRadius) || !isFinite(knot.SourceRadius) ||
			knot.DisplayRadius < 0 || knot.SourceRadius < 0 {
			return nil, ErrInvalidLUT
		}
		if i == 0 {
			if knot.DisplayRadius != 0 || knot.SourceRadius != 0 {
				return nil, ErrInvalidLUT
			}
		} else if knot.DisplayRadius <= previousDisplay || knot.SourceRadius <= previousSource {
			return nil, ErrInvalidLUT
		}
		previousDisplay, previousSource = knot.DisplayRadius, knot.SourceRadius
	}
	return &PiecewiseLUT{
		center:          center,
		referenceRadius: referenceRadius,
		knots:           append([]LUTKnot(nil), knots...),
	}, nil
}

func sortedFiniteRadii(radii []float64) error {
	if len(radii) == 0 {
		return ErrInvalidLUT
	}
	if radii[0] < 0 || !isFinite(radii[0]) {
		return ErrInvalidLUT
	}
	if !sort.Float64sAreSorted(radii) {
		return ErrInvalidLUT
	}
	for i := 1; i < len(radii); i++ {
		if !isFinite(radii[i]) || radii[i] <= radii[i-1] {
			return ErrInvalidLUT
		}
	}
	return nil
}

// NewLUTFromLens samples the analytic lens transfer at the supplied normalized
// display radii. Radius zero is inserted automatically when absent. At least
// two distinct radii are required after insertion.
func NewLUTFromLens(lens Lens, displayRadii []float64) (*PiecewiseLUT, error) {
	if err := lens.validate(); err != nil {
		return nil, err
	}
	radii := append([]float64(nil), displayRadii...)
	if len(radii) > 0 && radii[0] != 0 {
		radii = append([]float64{0}, radii...)
	}
	if err := sortedFiniteRadii(radii); err != nil || len(radii) < 2 {
		return nil, ErrInvalidLUT
	}
	knots := make([]LUTKnot, len(radii))
	for i, r := range radii {
		if lens.MaxRadius > 0 && r > lens.MaxRadius*(1+1e-14) {
			return nil, ErrOutOfDomain
		}
		source, err := lens.distortedFromUndistorted(lens.PointAtNormalizedRadius(r))
		if err != nil {
			return nil, err
		}
		sourceRadius := distance(source, lens.Center) / lens.ReferenceRadius
		knots[i] = LUTKnot{DisplayRadius: r, SourceRadius: sourceRadius}
	}
	return NewPiecewiseLUT(lens.Center, lens.ReferenceRadius, knots)
}

// NewUniformLUTFromLens builds segments+1 evenly spaced radial knots from the
// center through maxDisplayRadius.
func NewUniformLUTFromLens(lens Lens, maxDisplayRadius float64, segments int) (*PiecewiseLUT, error) {
	if segments < 1 || !isFinite(maxDisplayRadius) || maxDisplayRadius <= 0 {
		return nil, ErrInvalidLUT
	}
	if lens.MaxRadius > 0 && maxDisplayRadius > lens.MaxRadius*(1+1e-14) {
		return nil, ErrOutOfDomain
	}
	radii := make([]float64, segments+1)
	for i := range radii {
		radii[i] = maxDisplayRadius * float64(i) / float64(segments)
	}
	return NewLUTFromLens(lens, radii)
}

func (m *PiecewiseLUT) SourceFromDisplay(display Point) (Point, bool) {
	if !display.IsFinite() {
		return Point{}, false
	}
	delta := display.Sub(m.center)
	r := delta.Length() / m.referenceRadius
	if r == 0 {
		return m.center, true
	}
	rho, ok := m.sourceRadius(r)
	if !ok {
		return Point{}, false
	}
	return m.center.Add(delta.Mul(rho / r)), true
}

func (m *PiecewiseLUT) DisplayFromSource(source Point) (Point, bool) {
	if !source.IsFinite() {
		return Point{}, false
	}
	delta := source.Sub(m.center)
	rho := delta.Length() / m.referenceRadius
	if rho == 0 {
		return m.center, true
	}
	r, ok := m.displayRadius(rho)
	if !ok {
		return Point{}, false
	}
	return m.center.Add(delta.Mul(r / rho)), true
}

func (m *PiecewiseLUT) sourceRadius(r float64) (float64, bool) {
	if r < 0 || r > m.knots[len(m.knots)-1].DisplayRadius {
		return 0, false
	}
	i := sort.Search(len(m.knots), func(i int) bool {
		return m.knots[i].DisplayRadius >= r
	})
	if i == 0 {
		return m.knots[0].SourceRadius, true
	}
	left, right := m.knots[i-1], m.knots[i]
	t := (r - left.DisplayRadius) / (right.DisplayRadius - left.DisplayRadius)
	return left.SourceRadius + t*(right.SourceRadius-left.SourceRadius), true
}

func (m *PiecewiseLUT) displayRadius(rho float64) (float64, bool) {
	if rho < 0 || rho > m.knots[len(m.knots)-1].SourceRadius {
		return 0, false
	}
	i := sort.Search(len(m.knots), func(i int) bool {
		return m.knots[i].SourceRadius >= rho
	})
	if i == 0 {
		return m.knots[0].DisplayRadius, true
	}
	left, right := m.knots[i-1], m.knots[i]
	t := (rho - left.SourceRadius) / (right.SourceRadius - left.SourceRadius)
	return left.DisplayRadius + t*(right.DisplayRadius-left.DisplayRadius), true
}

func (m *PiecewiseLUT) Center() Point { return m.center }
func (m *PiecewiseLUT) Kind() MapKind { return KindLUT }

func (m *PiecewiseLUT) Knots() []LUTKnot {
	return append([]LUTKnot(nil), m.knots...)
}

// MaxDisplayRadius is the last sampled display radius.
func (m *PiecewiseLUT) MaxDisplayRadius() float64 {
	return m.knots[len(m.knots)-1].DisplayRadius
}

// MaxSourceRadius is the last sampled raw-source radius.
func (m *PiecewiseLUT) MaxSourceRadius() float64 {
	return m.knots[len(m.knots)-1].SourceRadius
}
