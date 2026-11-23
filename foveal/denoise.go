package foveal

// pixelState is the per-pixel history. All values refer to the latest frame
// position of the pixel; reprojection moves them between pixels each frame.
type pixelState struct {
	hist        [3]float32 // accumulated color in current-exposure light
	age         float32    // effective sample count, capped by zone history
	zone        Zone
	initialized bool
}

// Denoiser is a stateful foveated temporal accumulator. It is safe for
// sequential use only; feed frames one at a time via Process.
type Denoiser struct {
	params Params

	state []pixelState

	hasPrevGaze bool
	prevGaze    Vec2

	// scratch buffers reused across frames to avoid allocations.
	zoneMask []Zone
	reset    []ResetReason
	out      []float32
}

// New creates a Denoiser. A zero-valued Params value is filled with defaults.
func New(params Params) (*Denoiser, error) {
	if err := params.normalize(); err != nil {
		return nil, err
	}
	n := params.Width * params.Height
	d := &Denoiser{
		params:   params,
		state:    make([]pixelState, n),
		zoneMask: make([]Zone, n),
		reset:    make([]ResetReason, n),
		out:      make([]float32, n*3),
	}
	return d, nil
}

// Reset discards all accumulated history (for example on scene changes).
func (d *Denoiser) Reset() {
	for i := range d.state {
		d.state[i] = pixelState{}
	}
	d.hasPrevGaze = false
}

// Params returns the active configuration.
func (d *Denoiser) Params() Params { return d.params }

// Process denoises one frame.
func (d *Denoiser) Process(f *Frame) (*Result, error) {
	if f == nil {
		return nil, errFrame("nil frame")
	}
	if f.Width != d.params.Width || f.Height != d.params.Height {
		return nil, errFrame("frame size does not match denoiser")
	}
	n := f.Width * f.Height
	if len(f.RGB) != n*3 {
		return nil, errFrame("RGB buffer has wrong length")
	}
	if len(f.MV) != n {
		return nil, errFrame("motion-vector buffer has wrong length")
	}
	if f.Depth != nil && len(f.Depth) != n {
		return nil, errFrame("depth buffer has wrong length")
	}
	if f.Invalid != nil && len(f.Invalid) != n {
		return nil, errFrame("invalid buffer has wrong length")
	}

	// 1. Zones decide the two independent history lengths.
	d.buildZones(f)

	// 2. Invalidation ranges, per trigger source.
	for i := range d.reset {
		d.reset[i] = ReasonNone
	}
	extremeExposure := log2Abs(f.ExposureScale) > d.params.ExposureResetThreshold
	d.invalidateEye(f)
	d.invalidateMotionAndDisocclusion(f)
	if extremeExposure {
		for i := range d.reset {
			d.reset[i] |= ReasonExposure
		}
	}

	// 3. Reproject, compensate, clamp and accumulate per pixel.
	stats := Stats{Pixels: n}
	for y := 0; y < f.Height; y++ {
		for x := 0; x < f.Width; x++ {
			p := y*f.Width + x
			d.processPixel(f, p, x, y, extremeExposure, &stats)
		}
	}

	d.prevGaze = f.Gaze
	d.hasPrevGaze = true

	return &Result{
		RGB:   d.out,
		Reset: d.reset,
		Zones: d.zoneMask,
		Stats: stats,
	}, nil
}

// buildZones marks a guarded gaze disk as fovea, everything else periphery.
func (d *Denoiser) buildZones(f *Frame) {
	r := d.params.FoveaRadius + d.params.GuardWidth
	r2 := r * r
	for y := 0; y < f.Height; y++ {
		dy := float32(y) - f.Gaze.Y
		for x := 0; x < f.Width; x++ {
			dx := float32(x) - f.Gaze.X
			p := y*f.Width + x
			if dx*dx+dy*dy <= r2 {
				d.zoneMask[p] = ZoneFovea
			} else {
				d.zoneMask[p] = ZonePeriphery
			}
		}
	}
}

type frameError string

func (e frameError) Error() string { return "foveal: " + string(e) }

func errFrame(msg string) error { return frameError(msg) }
