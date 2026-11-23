package binocular

import "fmt"

// Eye identifies one side of a stereo frame.
type Eye uint8

const (
	// EyeLeft is the left eye.
	EyeLeft Eye = iota
	// EyeRight is the right eye.
	EyeRight
)

// String implements fmt.Stringer.
func (e Eye) String() string {
	switch e {
	case EyeLeft:
		return "left"
	case EyeRight:
		return "right"
	default:
		return fmt.Sprintf("eye(%d)", e)
	}
}

// Camera contains render-target dimensions plus the matrices owned by one eye.
// The matrices are copied into each batch's uniform upload; they are never
// cached across eyes.
type Camera struct {
	View       Mat4
	Projection Mat4
	Width      int
	Height     int
	Clear      RGB
}

// NewPerspectiveCamera builds a camera looking from eye toward target.
func NewPerspectiveCamera(
	eye, target, up Vec3,
	fovYRadians, aspect, near, far float64,
	width, height int,
	clear RGB,
) Camera {
	return Camera{
		View:       LookAt(eye, target, up),
		Projection: Perspective(fovYRadians, aspect, near, far),
		Width:      width,
		Height:     height,
		Clear:      clear,
	}
}

// Material contains the parameters that belong to a single eye's draw item.
type Material struct {
	BaseColor Vec3
}

const uploadedMaterialBytes = 3 * 8

// DrawItem binds an instance transform and material to shared geometry. Each
// eye pass has its own DrawItem values, so rotation and material overrides stay
// isolated.
type DrawItem struct {
	Geometry *Geometry
	Model    Mat4
	Material Material
}

// EyePass is the complete per-eye state for a stereo frame.
type EyePass struct {
	Eye    Eye
	Camera Camera
	Items  []DrawItem
}

func (pass EyePass) validate() error {
	if pass.Camera.Width <= 0 || pass.Camera.Height <= 0 {
		return fmt.Errorf("binocular: %s eye has invalid target size %dx%d",
			pass.Eye, pass.Camera.Width, pass.Camera.Height)
	}
	for i, item := range pass.Items {
		if item.Geometry == nil {
			return fmt.Errorf("binocular: %s eye item %d has nil geometry", pass.Eye, i)
		}
		if err := item.Geometry.Validate(); err != nil {
			return fmt.Errorf("binocular: %s eye item %d: %w", pass.Eye, i, err)
		}
	}
	return nil
}

func clonePass(pass EyePass) EyePass {
	cloned := pass
	cloned.Items = make([]DrawItem, len(pass.Items))
	copy(cloned.Items, pass.Items)
	copies := make(map[*Geometry]*Geometry)
	for i := range cloned.Items {
		original := cloned.Items[i].Geometry
		geometry, ok := copies[original]
		if !ok {
			geometry = original.clone()
			copies[original] = geometry
		}
		cloned.Items[i].Geometry = geometry
	}
	return cloned
}
