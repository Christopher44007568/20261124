package reprojection

import "math"

// WorldPixel is one late-reprojection sample.
type WorldPixel struct {
	Color    Color
	Position Vec3
	Depth    float64 // camera-space Z
	Panel    int16   // -1 means a hole / no world sample
	Near     bool
}

// WorldImage is a tiny deferred G-buffer: color, world position and depth.
type WorldImage struct {
	Width  int
	Height int
	Pixels []WorldPixel
	Panels []WorldPanel
}

func NewWorldImage(width, height int) *WorldImage {
	img := &WorldImage{
		Width:  width,
		Height: height,
		Pixels: make([]WorldPixel, width*height),
	}
	img.clear()
	return img
}

func (img *WorldImage) clear() {
	for i := range img.Pixels {
		img.Pixels[i] = WorldPixel{
			Color: RGB(7, 9, 14),
			Panel: -1,
		}
	}
}

func (img *WorldImage) At(x, y int) *WorldPixel {
	if x < 0 || y < 0 || x >= img.Width || y >= img.Height {
		return nil
	}
	return &img.Pixels[y*img.Width+x]
}

func (img *WorldImage) Coverage() float64 {
	if len(img.Pixels) == 0 {
		return 0
	}
	hit := 0
	for i := range img.Pixels {
		if img.Pixels[i].Panel >= 0 {
			hit++
		}
	}
	return float64(hit) / float64(len(img.Pixels))
}

func (img *WorldImage) NearCoverage() float64 {
	if len(img.Pixels) == 0 {
		return 0
	}
	near := 0
	for i := range img.Pixels {
		if img.Pixels[i].Panel >= 0 && img.Pixels[i].Near {
			near++
		}
	}
	return float64(near) / float64(len(img.Pixels))
}

// RenderWorld ray-casts the world at pose. Depth selection is per pixel, so
// near panels occlude farther surfaces before reprojection starts.
func RenderWorld(cam Camera, panels []WorldPanel, pose Pose) *WorldImage {
	img := NewWorldImage(cam.Width, cam.Height)
	img.Panels = panels
	dir := make([]Vec3, cam.Width)

	for y := 0; y < cam.Height; y++ {
		for x := 0; x < cam.Width; x++ {
			dir[x] = cam.Ray(float64(x)+0.5, float64(y)+0.5, pose)
		}
		for x := 0; x < cam.Width; x++ {
			origin := pose.Position
			bestT := math.Inf(1)
			bestPanel := -1
			var bestHit Vec3

			for i := range panels {
				hit, t, ok := panels[i].intersect(origin, dir[x])
				if ok && t < bestT {
					bestT = t
					bestPanel = i
					bestHit = hit
				}
			}
			if bestPanel < 0 {
				continue
			}

			camPoint := unrotateVec(bestHit.Sub(pose.Position), pose.Yaw, pose.Pitch)
			img.Pixels[y*cam.Width+x] = WorldPixel{
				Color:    panels[bestPanel].colorAt(bestHit),
				Position: bestHit,
				Depth:    camPoint.Z,
				Panel:    int16(bestPanel),
				Near:     panels[bestPanel].Class == NearField,
			}
		}
	}
	return img
}

// ReprojectWorld forward-warps each visible world sample to target pose. There
// is deliberately no z-reconstruction shortcut: each sample carries the 3D
// surface point that produced it.
func ReprojectWorld(src *WorldImage, cam Camera, target Pose) *WorldImage {
	dst := NewWorldImage(src.Width, src.Height)
	dst.Panels = src.Panels

	for i := range src.Pixels {
		p := &src.Pixels[i]
		if p.Panel < 0 {
			continue
		}
		screen, depth, ok := cam.Project(p.Position, target)
		if !ok || !cam.inside(screen) {
			continue
		}

		// A one-pixel footprint expansion avoids hairline cracks caused by
		// pure-rotation forward warping while preserving depth ordering.
		x0 := int(math.Floor(screen.X))
		y0 := int(math.Floor(screen.Y))
		for oy := 0; oy < 2; oy++ {
			for ox := 0; ox < 2; ox++ {
				q := dst.At(x0+ox, y0+oy)
				if q == nil {
					continue
				}
				if q.Panel >= 0 && q.Depth <= depth {
					continue
				}
				*q = WorldPixel{
					Color:    p.Color,
					Position: p.Position,
					Depth:    depth,
					Panel:    p.Panel,
					Near:     src.Panels[p.Panel].Class == NearField,
				}
			}
		}
	}
	return dst
}
