package preview

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"

	"headpose"
)

func testPreview(t *testing.T) Preview {
	t.Helper()
	rigid, err := headpose.NewRigid(headpose.IdentityQuat(), headpose.Vec3{})
	if err != nil {
		t.Fatalf("NewRigid: %v", err)
	}
	camera, err := headpose.NewRigid(headpose.IdentityQuat(), headpose.Vec3{X: 0.2, Y: 1.6})
	if err != nil {
		t.Fatalf("camera: %v", err)
	}
	scene, err := headpose.NewScene(rigid, camera, headpose.ScreenParams{
		PixelsPerMeter: 500,
		PrincipalX:     640,
		PrincipalY:     360,
	})
	if err != nil {
		t.Fatalf("NewScene: %v", err)
	}
	second, err := headpose.NewRigid(headpose.IdentityQuat(), headpose.Vec3{X: 1.0, Y: 1.6})
	if err != nil {
		t.Fatalf("second camera: %v", err)
	}
	return Preview{
		Scene: scene,
		Anchors: []Anchor{
			{Label: "<A1&>", Position: headpose.Vec3{X: 0.1, Y: 1.5, Z: -2}},
			{Label: "A2", Position: headpose.Vec3{X: -0.3, Y: 1.7, Z: -1.8}, Color: "not-a-color"},
		},
		CameraTrajectory: []CameraSample{
			{Label: "start", CameraToWorld: camera},
			{Label: "end", CameraToWorld: second},
		},
	}
}

func TestRenderSVGIsWellFormedAndComplete(t *testing.T) {
	var output bytes.Buffer
	err := RenderSVG(&output, testPreview(t), Options{Title: `Preview <&>`, Width: 900, Height: 600})
	if err != nil {
		t.Fatalf("RenderSVG: %v", err)
	}
	document := output.String()
	if !strings.HasPrefix(document, `<?xml version="1.0" encoding="UTF-8"?>`) {
		t.Fatal("missing XML declaration")
	}
	if !strings.Contains(document, `Preview &lt;&amp;&gt;`) {
		t.Fatal("title was not XML-escaped")
	}
	if !strings.Contains(document, `&lt;A1&amp;&gt;`) {
		t.Fatal("anchor label was not XML-escaped")
	}
	if !strings.Contains(document, `<polyline`) || !strings.Contains(document, `class="arrow-head"`) {
		t.Fatal("camera trajectory or arrowhead is missing")
	}
	if strings.Count(document, `<circle`) < 6 {
		t.Fatal("anchor or trajectory markers are missing")
	}

	decoder := xml.NewDecoder(strings.NewReader(document))
	elements := map[string]int{}
	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatalf("invalid SVG XML: %v", err)
		}
		if start, ok := token.(xml.StartElement); ok {
			elements[start.Name.Local]++
		}
	}
	for _, name := range []string{"svg", "defs", "rect", "line", "polyline", "circle", "text"} {
		if elements[name] == 0 {
			t.Errorf("missing <%s> element", name)
		}
	}
}

func TestRenderSVGRejectsInvalidInput(t *testing.T) {
	valid := testPreview(t)
	if err := RenderSVG(&bytes.Buffer{}, Preview{}, Options{}); err == nil {
		t.Fatal("expected missing scene to fail")
	}
	invalidTrajectory := valid
	invalidTrajectory.CameraTrajectory = valid.CameraTrajectory[:1]
	if err := RenderSVG(&bytes.Buffer{}, invalidTrajectory, Options{}); err == nil {
		t.Fatal("expected short trajectory to fail")
	}
	invalidSample := valid
	invalidSample.CameraTrajectory = append([]CameraSample(nil), valid.CameraTrajectory...)
	invalidSample.CameraTrajectory[0].CameraToWorld = headpose.Transform{}
	if err := RenderSVG(&bytes.Buffer{}, invalidSample, Options{}); err == nil {
		t.Fatal("expected invalid camera transform to fail")
	}
	tooSmall := Options{Width: 400, Height: 300}
	if err := RenderSVG(&bytes.Buffer{}, valid, tooSmall); err == nil {
		t.Fatal("expected undersized preview to fail")
	}
}
