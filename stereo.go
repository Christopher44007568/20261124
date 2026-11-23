package lensdistortion

import "math"

// Eye selects one half of a stereo display.
type Eye uint8

const (
	LeftEye Eye = iota
	RightEye
)

func (e Eye) String() string {
	switch e {
	case LeftEye:
		return "left"
	case RightEye:
		return "right"
	default:
		return "invalid"
	}
}

// StereoRig keeps two independent mappers. Each mapper owns its calibration
// center; no shared-center fallback is applied.
type StereoRig struct {
	mappers [2]Mapper
}

func NewStereoRig(left, right Mapper) (*StereoRig, error) {
	if left == nil || right == nil {
		return nil, ErrInvalidLens
	}
	for _, mapper := range []Mapper{left, right} {
		if !mapper.Center().IsFinite() {
			return nil, ErrInvalidLens
		}
	}
	return &StereoRig{mappers: [2]Mapper{left, right}}, nil
}

func (r *StereoRig) Mapper(eye Eye) Mapper {
	if r == nil || int(eye) >= len(r.mappers) {
		return nil
	}
	return r.mappers[eye]
}

func (r *StereoRig) Center(eye Eye) (Point, bool) {
	mapper := r.Mapper(eye)
	if mapper == nil {
		return Point{}, false
	}
	return mapper.Center(), true
}

func (r *StereoRig) SourceFromDisplay(eye Eye, display Point) (Point, bool) {
	mapper := r.Mapper(eye)
	if mapper == nil {
		return Point{}, false
	}
	return mapper.SourceFromDisplay(display)
}

func (r *StereoRig) DisplayFromSource(eye Eye, source Point) (Point, bool) {
	mapper := r.Mapper(eye)
	if mapper == nil {
		return Point{}, false
	}
	return mapper.DisplayFromSource(source)
}

// EyeCalibration is a convenience form for constructing a mapper using one of
// the three supported evaluation strategies.
//
//   - KindAnalytic uses Lens directly.
//   - KindPolynomial uses PolynomialCoefficients, or Lens.Coefficients when nil.
//   - KindLUT needs a positive Lens.MaxRadius when LUTRadii is empty; in that
//     case 64 uniform segments are generated.
type EyeCalibration struct {
	Kind                   MapKind
	Lens                   Lens
	PolynomialCoefficients []float64
	LUTRadii               []float64
}

func NewEyeMapper(calibration EyeCalibration) (Mapper, error) {
	if err := calibration.Lens.validate(); err != nil {
		return nil, err
	}
	switch calibration.Kind {
	case KindAnalytic:
		return NewAnalyticMap(calibration.Lens)
	case KindPolynomial:
		coefficients := calibration.PolynomialCoefficients
		if coefficients == nil {
			coefficients = calibration.Lens.Coefficients
		}
		return NewPolynomialMap(calibration.Lens, coefficients)
	case KindLUT:
		if len(calibration.LUTRadii) > 0 {
			return NewLUTFromLens(calibration.Lens, calibration.LUTRadii)
		}
		maxRadius := maxPolynomialRadius(calibration.Lens, math.Sqrt2)
		return NewUniformLUTFromLens(calibration.Lens, maxRadius, 64)
	default:
		return nil, ErrInvalidLens
	}
}

func NewStereoRigFromCalibrations(left, right EyeCalibration) (*StereoRig, error) {
	leftMapper, err := NewEyeMapper(left)
	if err != nil {
		return nil, err
	}
	rightMapper, err := NewEyeMapper(right)
	if err != nil {
		return nil, err
	}
	return NewStereoRig(leftMapper, rightMapper)
}
