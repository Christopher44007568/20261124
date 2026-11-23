package lensdistortion

// AnalyticMap evaluates the calibrated lens model directly. The inverse
// direction uses the bounded radial solver from Lens; no lookup table is
// involved.
type AnalyticMap struct {
	lens Lens
}

func NewAnalyticMap(lens Lens) (*AnalyticMap, error) {
	if err := lens.validate(); err != nil {
		return nil, err
	}
	if err := validateMonotonic(lens.Coefficients, lens.MaxRadius); err != nil {
		return nil, err
	}
	lens.Coefficients = append([]float64(nil), lens.Coefficients...)
	return &AnalyticMap{lens: lens}, nil
}

func (m *AnalyticMap) SourceFromDisplay(display Point) (Point, bool) {
	source, err := m.lens.distortedFromUndistorted(display)
	return source, err == nil
}

func (m *AnalyticMap) DisplayFromSource(source Point) (Point, bool) {
	display, err := m.lens.undistortedFromDistorted(source)
	return display, err == nil
}

func (m *AnalyticMap) Center() Point { return m.lens.Center }
func (m *AnalyticMap) Kind() MapKind { return KindAnalytic }
func (m *AnalyticMap) Calibration() Lens {
	lens := m.lens
	lens.Coefficients = append([]float64(nil), lens.Coefficients...)
	return lens
}
