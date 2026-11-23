package scene

import (
	"testing"

	"timewarp/geom"
)

func TestRenderRegionsAndMissingDepth(t *testing.T) {
	r := NewRenderer(200, 120)
	from := geom.NewPose(0, 0, geom.Vec3{})
	src := r.Render(from, true)
	dst := r.Render(geom.NewPose(0.1, -0.02, geom.Vec3{Z: 0.2}), false)

	counts := map[Region]int{}
	invalid := 0
	for i := range src.Pix {
		counts[src.Region[i]]++
		if !src.DepthValid[i] {
			invalid++
		}
		if dst.DepthValid[i] && dst.Depth[i] <= 0 {
			t.Fatalf("target pixel %d has non-positive valid depth %g", i, dst.Depth[i])
		}
	}
	for _, reg := range []Region{RegionRoom, RegionPole, RegionSpecular} {
		if counts[reg] == 0 {
			t.Fatalf("region %s missing from render: %+v", reg, counts)
		}
	}
	if invalid == 0 {
		t.Fatal("expected a missing-depth background patch")
	}
	for i := range src.Pix {
		if !src.DepthValid[i] && src.Region[i] == RegionPole {
			t.Fatal("near pole must keep depth")
		}
	}
}

func TestSourceMissingPatchIsBackground(t *testing.T) {
	r := NewRenderer(200, 120)
	f := r.Render(geom.NewPose(0, 0, geom.Vec3{}), true)
	for i := range f.Pix {
		if !f.DepthValid[i] && f.Region[i] != RegionRoom {
			t.Fatalf("invalid depth outside room at %d: %s", i, f.Region[i])
		}
	}
}
