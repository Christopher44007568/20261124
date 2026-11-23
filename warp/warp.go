package warp

import "stereowarp/geom"

// TileSize is the side length of repaint tiles. Small tiles keep repaint work
// close to actual reuse gaps instead of redrawing the whole right eye.
const TileSize = 16

// EyeResource is one rendered eye: its color and positive linear depth buffer
// plus the exact camera and version tags used to detect expired resources.
type EyeResource struct {
	Color       []geom.Vec3 // linear-ish RGB in 0..1, one entry per pixel
	Depth       []float64
	Camera      geom.Camera
	CameraEpoch uint64
	DepthEpoch  uint64
}

// Config carries the request to reuse one eye while repainting the other.
type Config struct {
	Source      EyeResource
	Target      geom.Camera
	CameraEpoch uint64 // epoch the target view currently belongs to
	DepthEpoch  uint64 // depth version the current frame requires
}

// Result reports what the warp+repaint pass actually did.
type Result struct {
	Reused   int
	Holes    int
	Dilated  int
	Repaint  int
	Tiles    int
	Glass    int
	Outside  int
	Rejected int
}

func validate(cfg Config) error {
	src := cfg.Source
	n := src.Camera.Width * src.Camera.Height
	if src.Color == nil || src.Depth == nil {
		return ErrNilResource
	}
	if len(src.Color) != n || len(src.Depth) != n {
		return ErrBufferSizeMismatch
	}
	if src.Camera.Width != cfg.Target.Width || src.Camera.Height != cfg.Target.Height {
		return ErrBufferSizeMismatch
	}
	if src.CameraEpoch != cfg.CameraEpoch {
		return ErrStaleCamera
	}
	if src.DepthEpoch != cfg.DepthEpoch {
		return ErrStaleDepth
	}
	for _, z := range src.Depth {
		if z != 0 && (z <= 0 || z >= src.Camera.Far) {
			return ErrDepthCorrupt
		}
	}
	return nil
}
