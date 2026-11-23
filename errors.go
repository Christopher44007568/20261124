package lensdistortion

import "errors"

var (
	ErrInvalidLens       = errors.New("lensdistortion: invalid lens calibration")
	ErrInvalidRegion     = errors.New("lensdistortion: invalid display region")
	ErrInvalidPoint      = errors.New("lensdistortion: non-finite point")
	ErrOutOfDomain       = errors.New("lensdistortion: point outside mapping domain")
	ErrNonMonotonic      = errors.New("lensdistortion: radial mapping is not monotonic")
	ErrInvalidPolynomial = errors.New("lensdistortion: invalid polynomial coefficients")
	ErrInvalidLUT        = errors.New("lensdistortion: invalid radial lookup table")
	ErrInvalidCoverage   = errors.New("lensdistortion: invalid coverage options")
)
