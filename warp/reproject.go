package warp

import (
	"stereowarp/geom"
	"stereowarp/raster"
)

// Reproject reuses the source eye to synthesize the target eye. Valid samples
// are unprojected with the source camera and reprojected through the target
// camera; every sample failing the target frustum or depth sanity test is
// dropped. Remaining holes and every region covered by a transparent surface
// are repainted per target tile with the real rasterizer.
func Reproject(cfg Config, meshes []*raster.Mesh) (*raster.Framebuffer, *Result, error) {
	if err := validate(cfg); err != nil {
		return nil, nil, err
	}

	src, dst := cfg.Source, cfg.Target
	w, h := dst.Width, dst.Height
	fb := raster.NewFramebuffer(w, h, raster.RGBA{R: 24, G: 26, B: 30, A: 255})
	valid := make([]bool, w*h)
	res := &Result{}
	sourceGlass := raster.CoverageMask(src.Camera, meshes, true)
	// Target coverage removes ghost samples and identifies real disocclusions:
	// only pixels where the target eye geometry actually exists are reusable.
	targetCovered := raster.CoverageMask(dst, meshes, false)
	refFB := raster.NewFramebuffer(w, h, raster.RGBA{R: 24, G: 26, B: 30, A: 255})
	raster.Renderer{}.Render(refFB, dst, meshes, raster.FullScissor(w, h), nil)
	// Target depth is the ground-truth visibility reference. A warped sample
	// whose depth disagrees with it belongs to a surface occluded in the target
	// eye and must be dropped even if its projection stays inside coverage.
	targetDepth := refFB.Depth

	// 1. Forward warp: source pixel -> world point -> target pixel. A 3x3
	// splat kernel keeps dense surfaces dense after the viewpoint shift.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			z := src.Depth[idx]
			if z == 0 || sourceGlass[idx] {
				continue // background or blended glass pixels are never reused
			}
			world := src.Camera.Unproject(float64(x), float64(y), z)
			u, v, targetZ, inside := dst.Project(world)
			if !inside || targetZ <= dst.Near || targetZ >= dst.Far {
				res.Outside++
				continue
			}
			c := src.Color[idx]
			color := raster.RGBA{R: toByte(c.X), G: toByte(c.Y), B: toByte(c.Z), A: 255}
			cx, cy := int(u), int(v)
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					tx, ty := cx+dx, cy+dy
					if tx < 0 || ty < 0 || tx >= w || ty >= h {
						continue
					}
					tid := ty*w + tx
					if !targetCovered[tid] {
						res.Outside++
						continue
					}
					if td := targetDepth[tid]; td != 0 && absF(td-targetZ) > depthTolerance(td) {
						res.Rejected++
						continue
					}
					if fb.Depth[tid] != 0 && fb.Depth[tid] <= targetZ {
						res.Rejected++ // closer warped sample already owns this pixel
						continue
					}
					if !valid[tid] {
						res.Reused++ // count unique target pixels filled by reuse
					}
					fb.Color[tid] = color
					fb.Depth[tid] = targetZ
					valid[tid] = true
				}
			}
		}
	}

	// 2. A hole is a target pixel covered by geometry that the warp missed.
	hole := make([]bool, w*h)
	for i := range hole {
		if targetCovered[i] && !valid[i] {
			hole[i] = true
		}
	}

	// 3. One-pixel dilation fills sub-pixel splat gaps without inventing scene.
	dilated := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			if !hole[idx] || fb.Depth[idx] != 0 {
				continue
			}
			if c, d, ok := nearestNeighbor(fb, valid, dilated, x, y); ok {
				fb.Color[idx] = c
				fb.Depth[idx] = d // 2D-expanded depth: no background gap remains
				dilated[idx] = true
				res.Dilated++
			} else {
				res.Holes++
			}
		}
	}

	// 4. Repaint mask: remaining disocclusion holes and every target pixel
	// covered by glass, whose blended color cannot be copied from the source.
	mask := buildRepaintMask(w, h, hole, dilated, dst, meshes)
	tiles := dirtyTiles(w, h, mask)
	res.Tiles = len(tiles)
	for _, m := range mask {
		if m {
			res.Repaint++
		}
	}

	// 5. Repaint only dirty tiles. Re-rendering through the target camera
	// restores disocclusions and replaces any warped transparent blending.
	rend := raster.Renderer{}
	for _, t := range tiles {
		scissor := raster.Scissor{X0: t.x0, Y0: t.y0, X1: t.x1, Y1: t.y1}
		rend.Render(fb, dst, meshes, scissor, mask)
	}

	glass := raster.CoverageMask(dst, meshes, true)
	for _, g := range glass {
		if g {
			res.Glass++
		}
	}
	return fb, res, nil
}

func nearestNeighbor(fb *raster.Framebuffer, valid, dilated []bool, x, y int) (raster.RGBA, float64, bool) {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			nx, ny := x+dx, y+dy
			if nx < 0 || ny < 0 || nx >= fb.Width || ny >= fb.Height {
				continue
			}
			nid := ny*fb.Width + nx
			if valid[nid] || dilated[nid] {
				return fb.Color[nid], fb.Depth[nid], true
			}
		}
	}
	return raster.RGBA{}, 0, false
}

func buildRepaintMask(w, h int, hole, dilated []bool, cam geom.Camera, meshes []*raster.Mesh) []bool {
	mask := make([]bool, w*h)
	for i := range mask {
		if hole[i] {
			mask[i] = true // disocclusions (including dilated cells) get repainted
		}
	}
	glass := raster.CoverageMask(cam, meshes, true)
	for i, g := range glass {
		if g {
			mask[i] = true
		}
	}
	return mask
}

type tile struct{ x0, y0, x1, y1 int }

func dirtyTiles(w, h int, mask []bool) []tile {
	var out []tile
	for y0 := 0; y0 < h; y0 += TileSize {
		y1 := min(y0+TileSize, h)
		for x0 := 0; x0 < w; x0 += TileSize {
			x1 := min(x0+TileSize, w)
			dirty := false
			for y := y0; y < y1 && !dirty; y++ {
				for x := x0; x < x1; x++ {
					if mask[y*w+x] {
						dirty = true
						break
					}
				}
			}
			if dirty {
				out = append(out, tile{x0, y0, x1, y1})
			}
		}
	}
	return out
}

func toByte(v float64) uint8 {
	v = geom.Clamp(v, 0, 1)
	return uint8(v*255 + 0.5)
}

func absF(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// depthTolerance grows with depth: distant surfaces naturally carry larger
// reconstruction error while near objects must match tightly.
func depthTolerance(depth float64) float64 {
	return 0.02 + 0.03*depth
}
