package headpose

// Frame identifies one coordinate system in the tracking pipeline.
type Frame string

const (
	// World is the fixed, physical reference frame.
	World Frame = "world"
	// Device is the tracked head or device rigid-body frame.
	Device Frame = "device"
	// Camera is the pinhole camera frame, with +Z pointing behind the camera.
	Camera Frame = "camera"
	// Screen is the image pixel frame, with +Z pointing into the screen.
	Screen Frame = "screen"
)

// Unit is the length unit used by a frame.
type Unit string

const (
	Meter Unit = "m"
	Pixel Unit = "px"
)

// Handedness describes the basis orientation of a frame.
type Handedness string

const (
	RightHanded Handedness = "right-handed"
)

// QuaternionOrder documents the component order used by Quat.
type QuaternionOrder string

// ScalarFirstQuaternionOrder means (w, x, y, z), with w stored first.
const ScalarFirstQuaternionOrder QuaternionOrder = "WXYZ"

// FrameSpec is the complete, machine-readable contract for a frame.
type FrameSpec struct {
	ID           Frame
	Unit         Unit
	Handedness   Handedness
	QuatOrder    QuaternionOrder
	XAxis        string
	YAxis        string
	ZAxis        string
	Origin       string
	RotationRole string
}

// FrameOrder is the deterministic order used for traversal and reports.
var FrameOrder = []Frame{World, Device, Camera, Screen}

var frameSpecs = map[Frame]FrameSpec{
	World: {
		ID:         World,
		Unit:       Meter,
		Handedness: RightHanded,
		QuatOrder:  ScalarFirstQuaternionOrder,
		XAxis:      "+X east/right",
		YAxis:      "+Y up",
		ZAxis:      "+Z south/backward",
		Origin:     "fixed site origin",
		RotationRole: "R_world_from_device and R_world_from_camera map local axes " +
			"into World; points use p_world = R*p_local + t",
	},
	Device: {
		ID:         Device,
		Unit:       Meter,
		Handedness: RightHanded,
		QuatOrder:  ScalarFirstQuaternionOrder,
		XAxis:      "+X device right",
		YAxis:      "+Y device up",
		ZAxis:      "+Z toward the user/out of the display",
		Origin:     "tracked head or device rigid-body origin",
		RotationRole: "device pose maps Device coordinates to World: " +
			"p_world = R_world_from_device * p_device + t_world_device",
	},
	Camera: {
		ID:         Camera,
		Unit:       Meter,
		Handedness: RightHanded,
		QuatOrder:  ScalarFirstQuaternionOrder,
		XAxis:      "+X image right",
		YAxis:      "+Y image up",
		ZAxis:      "+Z backward, opposite the optical viewing direction",
		Origin:     "pinhole optical center",
		RotationRole: "camera pose maps Camera coordinates to World: " +
			"p_world = R_world_from_camera * p_camera + t_world_camera",
	},
	Screen: {
		ID:         Screen,
		Unit:       Pixel,
		Handedness: RightHanded,
		QuatOrder:  ScalarFirstQuaternionOrder,
		XAxis:      "+X pixel right",
		YAxis:      "+Y pixel down",
		ZAxis:      "+Z into the screen; depth is expressed in pixels",
		Origin:     "image top-left pixel",
		RotationRole: "the Camera-to-Screen map is an affine similarity at a " +
			"declared pixels-per-meter scale; it preserves recoverable depth",
	},
}

// Spec returns the declared contract for f.
func (f Frame) Spec() (FrameSpec, bool) {
	spec, ok := frameSpecs[f]
	return spec, ok
}

// Valid reports whether f is one of the four supported frames.
func (f Frame) Valid() bool {
	_, ok := frameSpecs[f]
	return ok
}

// FrameSpecs returns a copy of all frame contracts in FrameOrder.
func FrameSpecs() []FrameSpec {
	specs := make([]FrameSpec, 0, len(FrameOrder))
	for _, frame := range FrameOrder {
		specs = append(specs, frameSpecs[frame])
	}
	return specs
}
