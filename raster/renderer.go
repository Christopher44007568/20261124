package raster

import (
	"sort"

	"stereowarp/geom"
)

type projected struct {
	u, v, z float64
	inside  bool
}

// Renderer is a tiny CPU rasterizer with no external dependencies.
type Renderer struct{}

func (Renderer) project(m *Mesh, cam geom.Camera) []projected {
	out := make([]projected, len(m.Vertices))
	for i, p := range m.Vertices {
		u, v, z, inside := cam.Project(p)
		out[i] = projected{u, v, z, inside}
	}
	return out
}

func edge(ax, ay, bx, by, px, py float64) float64 {
	return (bx-ax)*(py-ay) - (by-ay)*(px-ax)
}

// Render draws every mesh into fb as seen by cam. Opaque meshes are depth
// tested first; glass meshes are blended back to front. Only pixels inside
// scissor are touched. When clearMask is non-nil, mask pixels in the scissor
// have their depth reset first so a full re-render can replace stale warp data.
func (r Renderer) Render(fb *Framebuffer, cam geom.Camera, meshes []*Mesh, scissor Scissor, clearMask []bool) {
	if clearMask != nil {
		for y := scissor.Y0; y < scissor.Y1; y++ {
			for x := scissor.X0; x < scissor.X1; x++ {
				idx := fb.At(x, y)
				if clearMask[idx] {
					fb.Depth[idx] = 0
					fb.Color[idx] = fb.Background
					fb.glassOwner[idx] = -1
				}
			}
		}
	}
	glass := make([]*Mesh, 0)
	for _, m := range meshes {
		if m.Material.Transparent() {
			glass = append(glass, m)
			continue
		}
		r.renderOpaque(fb, cam, m, scissor)
	}
	r.renderGlass(fb, cam, glass, scissor)
}

func (r Renderer) renderOpaque(fb *Framebuffer, cam geom.Camera, m *Mesh, scissor Scissor) {
	pr := r.project(m, cam)
	for _, tri := range m.Triangles {
		a, b, c := pr[tri[0]], pr[tri[1]], pr[tri[2]]
		if !a.inside || !b.inside || !c.inside {
			continue
		}
		r.raster(fb, a, b, c, m.Material.Color, scissor, false, -1)
	}
}

type glassTri struct {
	a, b, c projected
	color   RGBA
	depth   float64
	meshID  int
}

func (r Renderer) renderGlass(fb *Framebuffer, cam geom.Camera, meshes []*Mesh, scissor Scissor) {
	tris := make([]glassTri, 0)
	for meshID, m := range meshes {
		pr := r.project(m, cam)
		for _, tri := range m.Triangles {
			a, b, c := pr[tri[0]], pr[tri[1]], pr[tri[2]]
			if !a.inside || !b.inside || !c.inside {
				continue
			}
			tris = append(tris, glassTri{a, b, c, m.Material.Color, (a.z + b.z + c.z) / 3, meshID})
		}
	}
	sort.SliceStable(tris, func(i, j int) bool { return tris[i].depth > tris[j].depth })
	for _, t := range tris {
		r.raster(fb, t.a, t.b, t.c, t.color, scissor, true, t.meshID)
	}
}

func (r Renderer) raster(fb *Framebuffer, a, b, c projected, col RGBA, scissor Scissor, blendIt bool, blendMeshID int) {
	minX := int(min(a.u, b.u, c.u))
	maxX := int(max(a.u, b.u, c.u)) + 1
	minY := int(min(a.v, b.v, c.v))
	maxY := int(max(a.v, b.v, c.v)) + 1
	minX, minY, maxX, maxY = scissor.clip(minX, minY, maxX, maxY, fb.Width, fb.Height)

	area := edge(a.u, a.v, b.u, b.v, c.u, c.v)
	if area == 0 {
		return
	}
	invA := 1.0 / area
	// Perspective-correct linear depth weights.
	wa, wb, wc := 1/a.z, 1/b.z, 1/c.z

	for y := minY; y < maxY; y++ {
		py := float64(y) + 0.5
		for x := minX; x < maxX; x++ {
			px := float64(x) + 0.5
			l0 := edge(b.u, b.v, c.u, c.v, px, py) * invA
			l1 := edge(c.u, c.v, a.u, a.v, px, py) * invA
			l2 := edge(a.u, a.v, b.u, b.v, px, py) * invA
			if l0 < 0 || l1 < 0 || l2 < 0 {
				continue
			}
			invZ := l0*wa + l1*wb + l2*wc
			z := 1.0 / invZ
			idx := fb.At(x, y)
			if fb.Depth[idx] != 0 && z >= fb.Depth[idx] {
				continue
			}
			if blendIt {
				if fb.glassOwner[idx] == blendMeshID {
					continue // second triangle of the same glass sheet
				}
				fb.Color[idx] = blend(fb.Color[idx], col)
				fb.glassOwner[idx] = blendMeshID
			} else {
				fb.Color[idx] = col
				fb.Depth[idx] = z
			}
		}
	}
}
