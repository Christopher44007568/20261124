package metrics

import (
	"image/color"
	"testing"

	"timewarp/scene"
	"timewarp/warp"
)

func TestComputePerfectAndRejected(t *testing.T) {
	gt := &scene.Frame{
		W: 2, H: 2,
		Pix: []color.RGBA{
			{10, 20, 30, 255}, {40, 50, 60, 255},
			{70, 80, 90, 255}, {100, 110, 120, 255},
		},
		Region: []scene.Region{
			scene.RegionRoom, scene.RegionPole,
			scene.RegionRoom, scene.RegionSpecular,
		},
	}
	out := &warp.Result{
		W: 2, H: 2,
		Pix:    append([]color.RGBA(nil), gt.Pix...),
		Reject: make([]warp.RejectReason, 4),
		Stats:  warp.Stats{Evaluated: 4, Valid: 4},
	}
	s := Compute("copy", out, gt)
	if s.MAE != 0 || s.Coverage != 1 || s.BadPixelRate != 0 {
		t.Fatalf("perfect warp scored %+v", s)
	}
	if len(s.Regions) != 3 {
		t.Fatalf("regions = %+v", s.Regions)
	}

	out.Reject[3] = warp.RejectNoCoverage
	out.Pix[0].R += 25
	s = Compute("changed", out, gt)
	if s.Valid != 3 || s.Rejected != 1 {
		t.Fatalf("counts = %+v", s)
	}
	if s.MAE <= 0 || s.MAE > 10 {
		t.Fatalf("MAE = %g", s.MAE)
	}
	if s.Coverage != 0.75 {
		t.Fatalf("coverage = %g", s.Coverage)
	}
	for _, r := range s.Regions {
		if r.Name == "static specular" && r.Valid != 0 {
			t.Fatalf("rejected specular counted valid: %+v", r)
		}
	}
}
