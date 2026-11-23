// Package foveal implements a foveated temporal accumulation denoiser.
//
// The fovea (gaze neighborhood) keeps a long history so static content can
// converge over many frames, while the periphery discards old samples much
// faster. History is invalidated with three distinct ranges: eye motion
// invalidates a saccade corridor plus the landing disk, object motion
// invalidates only locally divergent/disoccluded pixels, and exposure changes
// are compensated in place so still-valid regions keep their history, with a
// full reset reserved for extreme exposure jumps.
package foveal

// Vec2 is a 2D vector expressed in pixel units.
type Vec2 struct {
	X, Y float32
}

// Zone classifies a pixel for history-length selection.
type Zone uint8

const (
	// ZoneFovea covers the gaze disk and its guard band.
	ZoneFovea Zone = iota
	// ZonePeriphery covers everything outside the guarded fovea.
	ZonePeriphery
)

// ResetReason is a bit mask describing why a pixel discarded its history.
type ResetReason uint8

const (
	ReasonNone         ResetReason = 0
	ReasonEye          ResetReason = 1 << iota // saccade corridor / landing disk
	ReasonMotion                               // local motion-vector divergence
	ReasonDisocclusion                         // revealed background / invalid hint
	ReasonExposure                             // extreme global exposure jump
	ReasonHighlight                            // dynamic specular highlight
	ReasonBorder                               // reprojection left the frame
	ReasonZone                                 // fovea/periphery ownership changed
)

// Frame is one denoiser input: linear-light RGB plus motion metadata.
type Frame struct {
	Width  int
	Height int

	// RGB holds linear light, 3 floats per pixel (R,G,B row-major). Ordinary
	// surfaces stay around [0,1]; specular highlights are allowed to exceed 1.
	RGB []float32

	// Gaze is the current fixation point in pixel coordinates.
	Gaze Vec2

	// Motion vectors in pixels per frame: the content now at pixel p was at
	// p-MV[p] in the previous frame.
	MV []Vec2

	// Depth is an optional linear depth per pixel (smaller = nearer). Nil
	// disables depth-based disocclusion detection.
	Depth []float32

	// Invalid is an optional per-pixel disocclusion hint supplied by the
	// caller (for example an analytical renderer). Nil disables it.
	Invalid []bool

	// ExposureScale is currentExposure/previousExposure; 1 means unchanged.
	ExposureScale float32
}

// Params configures a Denoiser. Zero values are replaced by DefaultParams.
type Params struct {
	Width  int
	Height int

	// FoveaRadius is the gaze disk radius in pixels.
	FoveaRadius float32
	// GuardWidth extends foveal treatment around the disk so boundary pixels
	// do not flip ownership on tiny gaze jitter.
	GuardWidth float32

	// FovealHistory / PeripheralHistory are the equivalent sample counts the
	// per-pixel EMA converges to (alpha = 1/history).
	FovealHistory     int
	PeripheralHistory int

	// SaccadeSpeed is the gaze speed (px/frame) above which an eye move is
	// treated as a saccade.
	SaccadeSpeed float32
	// SaccadeCorridor is the half-width of the saccade invalidation corridor.
	SaccadeCorridor float32

	// MotionThreshold flags |MV| large enough to require validity checks.
	MotionThreshold float32
	// DivergenceThreshold flags pixels whose MV differs from its neighbors.
	DivergenceThreshold float32
	// DepthDisocclusionThreshold flags a reprojected source whose depth is
	// much farther than the current pixel's depth.
	DepthDisocclusionThreshold float32

	// ExposureResetThreshold is the |log2(scale)| beyond which the whole frame
	// is reset instead of compensated.
	ExposureResetThreshold float32

	// HighlightThreshold marks pixels as highlight candidates by luminance.
	HighlightThreshold float32
	// HighlightTolerance is the relative history/current mismatch that turns a
	// highlight candidate into an invalidation.
	HighlightTolerance float32

	// ClampExpand relaxes the 3x3 neighborhood min/max envelope by this
	// fraction of its span before history is clamped.
	ClampExpand float32
}

// Stats summarizes one Process call.
type Stats struct {
	Pixels int

	ResetEye          int
	ResetMotion       int
	ResetDisocclusion int
	ResetExposure     int
	ResetHighlight    int
	ResetBorder       int
	ResetZone         int

	FovealPixels     int
	PeripheralPixels int
	MeanFovealAge    float64
	MeanPeriphAge    float64
}

// Result is the output of Process.
type Result struct {
	// RGB is the denoised frame, same layout as Frame.RGB.
	RGB []float32
	// Reset is the per-pixel invalidation bit mask (0 = history reused).
	Reset []ResetReason
	// Zones is the per-pixel ownership used for this frame.
	Zones []Zone
	Stats Stats
}
