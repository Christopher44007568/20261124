package gazedenoise

import (
	"math"
	"testing"
)

func TestPartitionNeverBorrowsCentre(t *testing.T) {
	scene, err := GenerateScene(SceneCombined, 128, 96, 0.04, 31)
	if err != nil {
		t.Fatal(err)
	}
	params := DefaultParams()
	result, err := Denoise(scene.Noisy, ModePartitioned, params)
	if err != nil {
		t.Fatal(err)
	}
	if result.CrossPartitionTaps != 0 || result.CrossPartitionWeight != 0 {
		t.Fatalf("partition borrowed across mask: taps=%d weight=%f",
			result.CrossPartitionTaps, result.CrossPartitionWeight)
	}
	if result.UnfilledHoles != 0 {
		t.Fatalf("partition left %d holes unfilled", result.UnfilledHoles)
	}
	for i, inside := range scene.Noisy.Mask {
		if !inside || !scene.Noisy.Valid[i] {
			continue
		}
		if result.Output.Color[i] != scene.Noisy.Color[i] {
			t.Fatalf("centre pixel %d changed under partition mode", i)
		}
	}
	for _, prov := range result.Provenance {
		if len(prov.Sources) == 0 {
			t.Fatalf("hole %d has no sources", prov.Pixel)
		}
		for _, source := range prov.Sources {
			if scene.Noisy.Mask[source.Pixel] != scene.Noisy.Mask[prov.Pixel] {
				t.Fatalf("hole %d borrowed pixel %d across mask", prov.Pixel, source.Pixel)
			}
			if !scene.Noisy.Valid[source.Pixel] {
				t.Fatalf("hole %d uses invalid pixel %d", prov.Pixel, source.Pixel)
			}
		}
	}
	metrics, err := Evaluate(scene, result)
	if err != nil {
		t.Fatal(err)
	}
	if !metrics.ProvenanceValid {
		t.Fatalf("provenance invalid: %s", metrics.ProvenanceError)
	}
}

func TestFullModeIsLeakageBaseline(t *testing.T) {
	f, err := NewFrame(9, 9)
	if err != nil {
		t.Fatal(err)
	}
	for i := range f.Color {
		f.Color[i] = Vec3{X: 0.4, Y: 0.5, Z: 0.6}
		f.Mask[i] = i%9 >= 4
	}
	f.RecomputeLuma()

	partitioned, err := Denoise(f, ModePartitioned, DefaultParams())
	if err != nil {
		t.Fatal(err)
	}
	full, err := Denoise(f, ModeFull, DefaultParams())
	if err != nil {
		t.Fatal(err)
	}
	if partitioned.CrossPartitionTaps != 0 {
		t.Fatalf("partitioned mode leaked %d taps", partitioned.CrossPartitionTaps)
	}
	if full.CrossPartitionTaps == 0 {
		t.Fatal("full-image mode did not exercise cross-boundary taps")
	}
}

func TestHoleProvenanceReconstructsOutput(t *testing.T) {
	f, err := NewFrame(15, 11)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < f.Height; y++ {
		for x := 0; x < f.Width; x++ {
			i := f.Index(x, y)
			f.Color[i] = Vec3{X: 0.2 + 0.03*float64(x), Y: 0.3 + 0.02*float64(y), Z: 0.5}
			f.Normal[i] = Vec3{X: 0.1 * float64(x%3), Z: 1}.Normalized()
			f.Depth[i] = 2 + 0.03*float64(x+y)
			f.Mask[i] = x >= 7
			f.Valid[i] = true
		}
	}
	f.RecomputeLuma()
	f.Valid[f.Index(2, 2)] = false
	f.Valid[f.Index(7, 5)] = false
	f.Valid[f.Index(8, 5)] = false

	for _, mode := range []Mode{ModePartitioned, ModeFull} {
		result, err := Denoise(f, mode, DefaultParams())
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Provenance) != 3 {
			t.Fatalf("%s recorded %d holes, want 3", mode, len(result.Provenance))
		}
		for _, prov := range result.Provenance {
			var total float64
			var rebuilt Vec3
			for _, source := range prov.Sources {
				if source.Pixel < 0 || source.Pixel >= len(f.Color) {
					t.Fatalf("%s source %d out of range", mode, source.Pixel)
				}
				if !f.Valid[source.Pixel] {
					t.Fatalf("%s used invalid source %d", mode, source.Pixel)
				}
				if mode == ModePartitioned && f.Mask[source.Pixel] != f.Mask[prov.Pixel] {
					t.Fatalf("%s crossed partition for hole %d", mode, prov.Pixel)
				}
				total += source.Weight
				rebuilt = rebuilt.Add(f.Color[source.Pixel].Mul(source.Weight))
			}
			if math.Abs(total-1) > 1e-12 {
				t.Fatalf("%s weights sum to %.12f", mode, total)
			}
			if rebuilt.Sub(result.Output.Color[prov.Pixel]).Length() > 1e-12 {
				t.Fatalf("%s hole %d does not match recorded sources", mode, prov.Pixel)
			}
		}
	}
}
