// Package harness drives the synthetic stereo sequence, the re-projection
// variants under test and the acceptance verdict.
package harness

import (
	"stereoval/internal/geo"
	"stereoval/internal/render"
	"stereoval/internal/scene"
)

// Mode selects which re-projection behavior is exercised.
type Mode string

const (
	ModeHealthy    Mode = "healthy"
	ModeRejectHist Mode = "reject_history"
	ModeCrossFill  Mode = "cross_eye_fill"
	ModePoseSkew   Mode = "pose_version_skew"
)

// Config fixes one deterministic run.
type Config struct {
	Mode     Mode
	NFrames  int
	Width    int
	Height   int
	Focal    float64
	Baseline float64
	Path     geo.Path
	Rig      geo.Rig
}

// DefaultConfig returns the shared camera/path setup for all scenarios.
func DefaultConfig() Config {
	return Config{
		NFrames:  18,
		Width:    320,
		Height:   240,
		Focal:    360,
		Baseline: 0.12,
		Rig:      geo.Rig{Baseline: 0.12},
		Path: geo.Path{
			Duration:     3.0,
			PosAmplitude: geo.Vec3{X: 0.18, Y: 0.04, Z: 0.06},
			YawAmp:       0.018,
			PitchAmp:     0.009,
		},
	}
}

// eyeFrame bundles everything produced for one eye at one frame.
type eyeFrame struct {
	gt     *render.Frame
	warped *render.WarpResult
	pose   geo.Pose
	tag    string
}

// stereoFrame is the full pair at one timestamp.
type stereoFrame struct {
	idx   int
	head  geo.HeadSample
	left  eyeFrame
	right eyeFrame
}

// history stores a rendered eye frame plus the pose version that made it.
type history struct {
	frame *render.Frame
	pose  geo.Pose
	tag   string
}

// Run renders the whole sequence and applies the mode-specific warp policy.
func Run(cfg Config, sc *scene.Scene) ([]stereoFrame, geo.Intrinsics) {
	// Cross-eye fill needs pronounced disocclusions: a stronger lateral
	// motion opens visible gaps that the other eye is then asked to fill.
	if cfg.Mode == ModeCrossFill {
		cfg.Path.PosAmplitude = geo.Vec3{X: 0.55, Y: 0.03, Z: 0.05}
		cfg.Path.YawAmp, cfg.Path.PitchAmp = 0.01, 0.005
	}
	k := geo.Intrinsics{
		W: cfg.Width, H: cfg.Height, Fx: cfg.Focal, Fy: cfg.Focal,
		Cx: float64(cfg.Width) / 2, Cy: float64(cfg.Height) / 2,
	}
	frames := make([]stereoFrame, cfg.NFrames)
	var prevL, prevR *history

	for i := 0; i < cfg.NFrames; i++ {
		t := cfg.Path.Duration * float64(i) / float64(cfg.NFrames-1)
		head := cfg.Path.Sample(t)
		curPoseL := head.EyePose(cfg.Rig, geo.Left)
		curPoseR := head.EyePose(cfg.Rig, geo.Right)
		gtL := render.Render(sc, k, curPoseL)
		gtR := render.Render(sc, k, curPoseR)

		sf := stereoFrame{idx: i, head: head}

		// Default policy: warp previous frame into the current geometry using
		// the current pose version (fresh estimate per eye).
		sf.left = makeEye(k, curPoseL, prevL, gtL, "v"+itoa(i))
		sf.right = makeEye(k, curPoseR, prevR, gtR, "v"+itoa(i))

		switch cfg.Mode {
		case ModeRejectHist:
			// Right eye rejects history every other frame: nothing to warp,
			// the holes are exposed rather than filled.
			if i%2 == 1 {
				sf.right = rejectedEye(k, curPoseR, gtR, "v"+itoa(i))
			}
		case ModePoseSkew:
			// Left uses the current pose version, right is built with a stale
			// version (two frames old) while still warping current pixels.
			if i >= 2 {
				oldHead := cfg.Path.Sample(cfg.Path.Duration * float64(i-2) / float64(cfg.NFrames-1))
				stalePose := oldHead.EyePose(cfg.Rig, geo.Right)
				sf.right.warped = render.ForwardWarpDepth(prevR.frame, gtR, k, prevR.pose, stalePose, "v"+itoa(i-2))
				sf.right.pose, sf.right.tag = curPoseR, "v"+itoa(i-2)
			}
		}

		if cfg.Mode == ModeCrossFill && i > 0 {
			render.CrossEyeFill(sf.left.warped, prevR.frame, prevR.pose, k, geo.Right)
			render.CrossEyeFill(sf.right.warped, prevL.frame, prevL.pose, k, geo.Left)
		}

		frames[i] = sf
		prevL = &history{frame: gtL, pose: curPoseL, tag: "v" + itoa(i)}
		prevR = &history{frame: gtR, pose: curPoseR, tag: "v" + itoa(i)}
	}
	return frames, k
}

func makeEye(k geo.Intrinsics, cur geo.Pose, prev *history, gt *render.Frame, tag string) eyeFrame {
	if prev == nil {
		return eyeFrame{gt: gt, warped: render.ForwardWarpDepth(gt, gt, k, cur, cur, tag), pose: cur, tag: tag}
	}
	return eyeFrame{gt: gt, warped: render.ForwardWarpDepth(prev.frame, gt, k, prev.pose, cur, tag), pose: cur, tag: tag}
}

func rejectedEye(k geo.Intrinsics, cur geo.Pose, gt *render.Frame, tag string) eyeFrame {
	// No accepted history: the warp output is entirely holes.
	empty := render.ForwardWarpDepth(gt, gt, k, cur, cur, tag)
	for i := range empty.Gray {
		empty.Gray[i] = nan()
		empty.Depth[i] = inf()
		empty.Hole[i] = true
	}
	return eyeFrame{gt: gt, warped: empty, pose: cur, tag: tag + ":rejected"}
}

func nan() float64 { return nanVal }
func inf() float64 { return infVal }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
