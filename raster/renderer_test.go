package raster

import (
	"testing"

	"stereowarp/geom"
)

func testCam() geom.Camera {
	return geom.Camera{Pos: geom.V(0, 0, 0), Near: 0.1, Far: 20, FovY: 1.0, Width: 80, Height: 60}
}

func TestOpaqueDepthTest(t *testing.T) {
	cam := testCam()
	near := Box("near", geom.V(-0.5, -0.5, 2), geom.V(0.5, 0.5, 2.2),
		Material{Color: RGBA{R: 255, G: 0, B: 0, A: 255}})
	far := Box("far", geom.V(-0.5, -0.5, 4), geom.V(0.5, 0.5, 4.2),
		Material{Color: RGBA{R: 0, G: 255, B: 0, A: 255}})
	fb := NewFramebuffer(80, 60, RGBA{R: 0, G: 0, B: 0, A: 255})
	Renderer{}.Render(fb, cam, []*Mesh{far, near}, FullScissor(80, 60), nil)

	center := fb.At(40, 30)
	if fb.Color[center] != (RGBA{R: 255, G: 0, B: 0, A: 255}) {
		t.Fatalf("near box must occlude far box, got %v", fb.Color[center])
	}
	if fb.Depth[center] < 2 || fb.Depth[center] > 2.2 {
		t.Fatalf("center depth should sit on the near box, got %v", fb.Depth[center])
	}
}

func TestGlassAlphaBlend(t *testing.T) {
	cam := testCam()
	wall := Quad("wall",
		geom.V(-1, -1, 5), geom.V(1, -1, 5), geom.V(1, 1, 5), geom.V(-1, 1, 5),
		Material{Color: RGBA{R: 0, G: 0, B: 0, A: 255}})
	glass := Quad("glass",
		geom.V(-1, -1, 3), geom.V(1, -1, 3), geom.V(1, 1, 3), geom.V(-1, 1, 3),
		Material{Color: RGBA{R: 200, G: 200, B: 200, A: 128}})
	fb := NewFramebuffer(80, 60, RGBA{R: 0, G: 0, B: 0, A: 255})
	Renderer{}.Render(fb, cam, []*Mesh{wall, glass}, FullScissor(80, 60), nil)
	got := fb.Color[fb.At(40, 30)]
	want := RGBA{R: 100, G: 100, B: 100, A: 255} // 200 * (128/255) ≈ 100
	if got != want {
		t.Fatalf("alpha blend = %v, want %v", got, want)
	}
}

func TestScissorLeavesOutsideUntouched(t *testing.T) {
	cam := testCam()
	box := Box("b", geom.V(-2, -2, 3), geom.V(2, 2, 3.2),
		Material{Color: RGBA{R: 10, G: 20, B: 30, A: 255}})
	fb := NewFramebuffer(80, 60, RGBA{R: 1, G: 2, B: 3, A: 255})
	sc := Scissor{X0: 0, Y0: 0, X1: 10, Y1: 10}
	Renderer{}.Render(fb, cam, []*Mesh{box}, sc, nil)
	outside := fb.At(70, 50)
	if fb.Color[outside] != (RGBA{R: 1, G: 2, B: 3, A: 255}) {
		t.Fatalf("pixels outside the scissor must be untouched, got %v", fb.Color[outside])
	}
}
