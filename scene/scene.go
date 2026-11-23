// Package scene renders a small analytic room used to compare rotational
// timewarp against depth-aware translation reprojection.
package scene

import (
	"image/color"

	"timewarp/geom"
)

// Region labels let the report score the room, the near pole, and the
// world-anchored specular glints independently.
type Region uint8

const (
	RegionRoom Region = iota
	RegionPole
	RegionSpecular
)

func (r Region) String() string {
	switch r {
	case RegionPole:
		return "near pole"
	case RegionSpecular:
		return "static specular"
	default:
		return "room"
	}
}

// Frame is a rendered color, depth, and semantic-region buffer.
type Frame struct {
	W, H       int
	Pix        []color.RGBA
	Depth      []float64
	DepthValid []bool
	Region     []Region
}

func newFrame(w, h int) *Frame {
	n := w * h
	return &Frame{
		W: w, H: h,
		Pix:        make([]color.RGBA, n),
		Depth:      make([]float64, n),
		DepthValid: make([]bool, n),
		Region:     make([]Region, n),
	}
}

func (f *Frame) At(x, y int) color.RGBA { return f.Pix[y*f.W+x] }

// Glint is a world-anchored highlight. It is intentionally not a view-space
// bloom: translation must move it with the room geometry.
type Glint struct {
	P     geom.Vec3
	Color [3]float64
	Sigma float64
	Gain  float64
}

// Glints are the static specular targets checked by the report.
var Glints = []Glint{
	{P: geom.Vec3{X: -1.15, Y: 0.32, Z: 7.48}, Color: [3]float64{1.0, 0.86, 0.52}, Sigma: 0.10, Gain: 3.4},
	{P: geom.Vec3{X: 1.55, Y: -0.72, Z: 7.48}, Color: [3]float64{0.55, 0.78, 1.0}, Sigma: 0.12, Gain: 2.8},
	{P: geom.Vec3{X: 1.05, Y: -2.19, Z: 4.1}, Color: [3]float64{1.0, 0.68, 0.85}, Sigma: 0.14, Gain: 3.0},
}

const (
	roomMinX = -3.2
	roomMaxX = 3.2
	roomMinY = -2.2
	roomMaxY = 2.2
	roomMinZ = -2.0
	roomMaxZ = 7.5

	poleX  = 0.62
	poleZ  = 1.58
	poleR  = 0.105
	poleY0 = roomMinY
	poleY1 = roomMaxY
)

// PoleCenter is the world-space point the visual report zooms around.
var PoleCenter = geom.Vec3{X: poleX, Y: 0.15, Z: poleZ}

// Renderer turns camera poses into color/depth frames.
type Renderer struct {
	In           geom.Intrinsics
	Supersamples int
	FOVY         float64
}

func NewRenderer(w, h int) *Renderer {
	const fovY = 52.0 * 3.14159265358979323846 / 180.0
	return &Renderer{
		In:           geom.NewIntrinsics(w, h, fovY),
		Supersamples: 2,
		FOVY:         fovY,
	}
}

// Render traces the room. dropBackgroundDepth simulates a sensor that has
// color but no depth over one patch of the far wall.
func (r *Renderer) Render(pose geom.Pose, dropBackgroundDepth bool) *Frame {
	f := newFrame(r.In.W, r.In.H)
	ss := r.Supersamples
	if ss < 1 {
		ss = 1
	}
	for y := 0; y < f.H; y++ {
		for x := 0; x < f.W; x++ {
			var sum [3]float64
			hits := 0
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					u := float64(x) + (float64(sx)+0.5)/float64(ss) - 0.5
					v := float64(y) + (float64(sy)+0.5)/float64(ss) - 0.5
					s := r.trace(pose, u, v)
					if s.ok {
						sum[0] += s.col[0]
						sum[1] += s.col[1]
						sum[2] += s.col[2]
						hits++
					}
				}
			}
			center := r.trace(pose, float64(x), float64(y))
			i := y*f.W + x
			f.Region[i] = center.region
			if hits > 0 {
				f.Pix[i] = toRGBA(sum[0]/float64(hits), sum[1]/float64(hits), sum[2]/float64(hits))
			}
			if center.ok {
				f.Depth[i] = center.depth
				f.DepthValid[i] = true
				if dropBackgroundDepth && center.missingDepth {
					f.DepthValid[i] = false
					f.Depth[i] = 0
				}
			}
		}
	}
	return f
}

type hitSample struct {
	ok           bool
	col          [3]float64
	depth        float64
	region       Region
	missingDepth bool
}

func (r *Renderer) trace(pose geom.Pose, u, v float64) hitSample {
	rayCam := r.In.CamRay(u, v)
	dir := pose.R.Transpose().MulVec(rayCam)
	t, normal, region, ok := intersect(pose.P, dir)
	if !ok {
		return hitSample{}
	}
	hit := pose.P.Add(dir.Mul(t))
	col, glintWeight := shade(hit, normal, region, pose.P)
	if glintWeight > 0.16 {
		region = RegionSpecular
	}
	return hitSample{
		ok:           true,
		col:          col,
		depth:        t,
		region:       region,
		missingDepth: onMissingDepthPatch(hit, normal),
	}
}
