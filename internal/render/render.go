package render

import (
	"math"

	"curvelayers/internal/geom"
	"curvelayers/internal/tex"
)

// EyeBuffer holds one rendered eye: frame, nearest depth and owner per pixel.
type EyeBuffer struct {
	Width, Height int
	RGB           []float64
	Depth         []float64
	Owner         []int // surface ID of the nearest hit, -1 on background
	NaiveRGB      []float64
	NaiveOwner    []int // surface picked by fixed painter order, -1 on background
}

// NaiveOrder is the deliberately wrong fixed compositing order used only for
// the ordering-error diagnostic: sphere painted over everything, then
// cylinder, then plane, regardless of actual ray distance.
var NaiveOrder = []int{2, 1, 0}

func newEyeBuffer(w, h int) *EyeBuffer {
	n := w * h
	b := &EyeBuffer{
		Width: w, Height: h,
		RGB:        make([]float64, n*3),
		Depth:      make([]float64, n),
		Owner:      make([]int, n),
		NaiveRGB:   make([]float64, n*3),
		NaiveOwner: make([]int, n),
	}
	for i := range b.Depth {
		b.Depth[i] = math.Inf(1)
		b.Owner[i] = -1
		b.NaiveOwner[i] = -1
	}
	return b
}

// RenderEye ray-traces one eye. Surfaces stay fully opaque; the marker decal
// replaces body color where its alpha is non-zero.
func RenderEye(cam geom.Camera, layers []Layer, marker *tex.Marker) *EyeBuffer {
	buf := newEyeBuffer(cam.WidthPx, cam.HeightPx)
	byID := make(map[int]int, len(layers))
	for i := range layers {
		byID[layers[i].Surface.ID] = i
	}
	for y := 0; y < cam.HeightPx; y++ {
		for x := 0; x < cam.WidthPx; x++ {
			origin, dir := cam.RayFor(x, y)
			idx := y*cam.WidthPx + x
			buf.RGB[idx*3], buf.RGB[idx*3+1], buf.RGB[idx*3+2] = background(x, y, cam.WidthPx, cam.HeightPx)
			buf.NaiveRGB[idx*3], buf.NaiveRGB[idx*3+1], buf.NaiveRGB[idx*3+2] =
				buf.RGB[idx*3], buf.RGB[idx*3+1], buf.RGB[idx*3+2]

			var nearest *geom.Hit
			for li := range layers {
				hit, ok := layers[li].Surface.Intersect(origin, dir)
				if !ok {
					continue
				}
				if nearest == nil || hit.T < nearest.T {
					nearest = &hit
				}
			}
			if nearest != nil {
				layer := layers[byID[nearest.Surface.ID]]
				rgb, _ := paintLayer(layer, *nearest)
				r, g, bl, a, decal := marker.SamplePlacement(nearest.U, nearest.V, layer.Placement)
				if decal {
					rgb = blendOver(rgb, [3]float64{r, g, bl}, a)
				}
				buf.Depth[idx] = nearest.T
				buf.Owner[idx] = nearest.Surface.ID
				buf.RGB[idx*3], buf.RGB[idx*3+1], buf.RGB[idx*3+2] = rgb[0], rgb[1], rgb[2]
			}
			renderNaive(buf, idx, origin, dir, layers, marker)
		}
	}
	return buf
}

// renderNaive composites every hit in fixed order (last one wins per that
// order), ignoring distance: the mistake the ordering-error map exposes.
func renderNaive(buf *EyeBuffer, idx int, origin, dir geom.Vec3, layers []Layer, marker *tex.Marker) {
	for _, id := range NaiveOrder {
		if id >= len(layers) || layers[id].Surface.ID != id {
			continue // single-reference renders only contain a subset of layers
		}
		hit, ok := layers[id].Surface.Intersect(origin, dir)
		if !ok {
			continue
		}
		rgb, _ := paintLayer(layers[id], hit)
		r, g, b, a, decal := marker.SamplePlacement(hit.U, hit.V, layers[id].Placement)
		if decal {
			rgb = blendOver(rgb, [3]float64{r, g, b}, a)
		}
		buf.NaiveRGB[idx*3], buf.NaiveRGB[idx*3+1], buf.NaiveRGB[idx*3+2] = rgb[0], rgb[1], rgb[2]
		buf.NaiveOwner[idx] = id
	}
}

func blendOver(dst [3]float64, src [3]float64, alpha float64) [3]float64 {
	return [3]float64{
		src[0]*alpha + dst[0]*(1-alpha),
		src[1]*alpha + dst[1]*(1-alpha),
		src[2]*alpha + dst[2]*(1-alpha),
	}
}

func background(x, y, w, h int) (float64, float64, float64) {
	fx := float64(x) / float64(w)
	fy := float64(y) / float64(h)
	v := 0.05 + 0.05*(1-fy)
	vignette := 1 - 0.35*math.Hypot(fx-0.5, fy-0.5)
	return v * vignette, v * 1.1 * vignette, v * 1.35 * vignette
}
