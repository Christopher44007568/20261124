package stereo

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
)

const areaEpsilon = 1e-8

// Material describes how a scene mesh is drawn. Opacity never changes the
// clipping math; transparent sheets are clipped exactly like opaque meshes.
type Material struct {
	Color       string  `json:"color"`
	Opacity     float64 `json:"opacity"`
	Transparent bool    `json:"transparent"`
}

// Mesh is a triangle soup in world space.
type Mesh struct {
	Name      string    `json:"name"`
	Material  Material  `json:"material"`
	Triangles [][3]Vec3 `json:"-"`
}

// Scene contains geometry and two independently configured eye cameras.
type Scene struct {
	ID          string
	Title       string
	Description string
	Notes       []string
	Meshes      []Mesh
	Eyes        [2]Camera
	ViewportW   int
	ViewportH   int
}

// Metrics is shared by per-object rows and per-eye totals.
type Metrics struct {
	SourceTriangles         int     `json:"sourceTriangles"`
	RejectedTriangles       int     `json:"rejectedTriangles"`
	AcceptedSourceTriangles int     `json:"acceptedSourceTriangles"`
	ClippedPolygons         int     `json:"clippedPolygons"`
	NearClippedTriangles    int     `json:"nearClippedTriangles"`
	VisibleSourceTriangles  int     `json:"visibleSourceTriangles"`
	VisibleScreenTriangles  int     `json:"visibleScreenTriangles"`
	RejectedObjects         int     `json:"rejectedObjects"`
	SourceArea3D            float64 `json:"sourceArea3D"`
	ClippedArea3D           float64 `json:"clippedArea3D"`
	ScreenAreaPx            float64 `json:"screenAreaPx"`
	CoveragePct             float64 `json:"coveragePct"`
	NearEdgeLengthPx        float64 `json:"nearEdgeLengthPx"`
}

// PolygonReport is one source triangle after per-eye near and screen clipping.
type PolygonReport struct {
	SourceTriangle int       `json:"sourceTriangle"`
	ViewPolygon    []Vec3    `json:"viewPolygon"`
	ScreenPolygon  []Vec2    `json:"screenPolygon"`
	NearEdges      [][2]Vec2 `json:"nearEdges"`
	SourceArea3D   float64   `json:"sourceArea3D"`
	ClippedArea3D  float64   `json:"clippedArea3D"`
	ScreenAreaPx   float64   `json:"screenAreaPx"`
	NearClipped    bool      `json:"nearClipped"`
	ScreenClipped  bool      `json:"screenClipped"`
}

// ObjectEyeReport keeps each mesh separate so comparisons cannot accidentally
// pool left and right eye results.
type ObjectEyeReport struct {
	Mesh     string          `json:"mesh"`
	Material Material        `json:"material"`
	Metrics  Metrics         `json:"metrics"`
	Polygons []PolygonReport `json:"polygons"`
}

// EyeReport is the complete result for exactly one eye.
type EyeReport struct {
	Eye             string            `json:"eye"`
	Camera          Camera            `json:"camera"`
	NearPlaneCenter Vec3              `json:"nearPlaneCenter"`
	Objects         []ObjectEyeReport `json:"objects"`
	Totals          Metrics           `json:"totals"`
	Signature       string            `json:"signature"`
}

// Comparison summarizes the two eye results without ever combining their
// clipping planes or reusing one eye's polygons for the other.
type Comparison struct {
	ScreenAreaDeltaPx   float64 `json:"screenAreaDeltaPx"`
	ClippedAreaDelta3D  float64 `json:"clippedAreaDelta3D"`
	NearEdgeDeltaPx     float64 `json:"nearEdgeDeltaPx"`
	DistinctNearCenters bool    `json:"distinctNearCenters"`
	DistinctClipResults bool    `json:"distinctClipResults"`
	DistinctCoverage    bool    `json:"distinctCoverage"`
}

// SceneInfo is lightweight data for scenario switching in the UI.
type SceneInfo struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// SceneReport is the full JSON payload for one scene.
type SceneReport struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Notes       []string     `json:"notes"`
	ViewportW   int          `json:"viewportWidth"`
	ViewportH   int          `json:"viewportHeight"`
	Eyes        [2]EyeReport `json:"eyes"`
	Comparison  Comparison   `json:"comparison"`
}

// Analyze runs the same near-clip and coverage pipeline independently for
// every eye. No camera, near plane, polygon, or area is shared between them.
func Analyze(scene Scene) (SceneReport, error) {
	if len(scene.Eyes) != 2 {
		return SceneReport{}, fmt.Errorf("scene %q requires two eyes", scene.ID)
	}
	report := SceneReport{
		ID:          scene.ID,
		Title:       scene.Title,
		Description: scene.Description,
		Notes:       append([]string(nil), scene.Notes...),
		ViewportW:   scene.ViewportW,
		ViewportH:   scene.ViewportH,
	}

	for eyeIndex, cam := range scene.Eyes {
		eye, err := analyzeEye(scene, cam)
		if err != nil {
			return SceneReport{}, err
		}
		report.Eyes[eyeIndex] = eye
	}
	report.Comparison = compareEyes(report.Eyes[0], report.Eyes[1])
	return report, nil
}

func analyzeEye(scene Scene, cam Camera) (EyeReport, error) {
	eye := EyeReport{
		Eye:             cam.Name,
		Camera:          cam,
		NearPlaneCenter: cam.NearPlaneCenter(),
		Objects:         make([]ObjectEyeReport, 0, len(scene.Meshes)),
	}
	viewportArea := float64(cam.ViewportW * cam.ViewportH)

	for _, mesh := range scene.Meshes {
		obj, err := analyzeMesh(cam, mesh, viewportArea)
		if err != nil {
			return EyeReport{}, err
		}
		eye.Objects = append(eye.Objects, obj)
		addMetrics(&eye.Totals, &obj.Metrics)
	}
	if viewportArea > 0 {
		eye.Totals.CoveragePct = eye.Totals.ScreenAreaPx / viewportArea * 100
	}
	eye.Signature = eyeSignature(eye)
	return eye, nil
}

func analyzeMesh(cam Camera, mesh Mesh, viewportArea float64) (ObjectEyeReport, error) {
	obj := ObjectEyeReport{
		Mesh:     mesh.Name,
		Material: mesh.Material,
		Polygons: make([]PolygonReport, 0, len(mesh.Triangles)),
	}
	obj.Metrics.SourceTriangles = len(mesh.Triangles)

	for sourceIndex, tri := range mesh.Triangles {
		sourceArea := triangleArea3D(tri)
		obj.Metrics.SourceArea3D += sourceArea

		clipped := ClipTriangleNear(cam, tri)
		if len(clipped) < 3 {
			obj.Metrics.RejectedTriangles++
			continue
		}

		obj.Metrics.ClippedPolygons++
		obj.Metrics.NearClippedTriangles += len(clipped) - 2
		worldPoly := make([]Vec3, len(clipped))
		viewPoly := make([]Vec3, len(clipped))
		nearClipped := false
		for i, v := range clipped {
			worldPoly[i] = v.World
			viewPoly[i] = v.View
			nearClipped = nearClipped || v.Near
		}
		clippedArea := Area3D(worldPoly)
		obj.Metrics.ClippedArea3D += clippedArea

		screenNDC := ProjectPolygon(cam, clipped)
		screenClipped := needsScreenClip(cam, clipped)
		screenPixels := PixelPolygon(cam, screenNDC)
		screenArea := Area2D(screenPixels)
		if screenArea <= areaEpsilon {
			continue
		}

		obj.Metrics.VisibleSourceTriangles++
		obj.Metrics.VisibleScreenTriangles += len(screenNDC) - 2
		obj.Metrics.ScreenAreaPx += screenArea
		edges := NearEdges(cam, screenNDC)
		for _, edge := range edges {
			obj.Metrics.NearEdgeLengthPx += edge[0].Dist(edge[1])
		}
		obj.Polygons = append(obj.Polygons, PolygonReport{
			SourceTriangle: sourceIndex,
			ViewPolygon:    viewPoly,
			ScreenPolygon:  screenPixels,
			NearEdges:      edges,
			SourceArea3D:   sourceArea,
			ClippedArea3D:  clippedArea,
			ScreenAreaPx:   screenArea,
			NearClipped:    nearClipped,
			ScreenClipped:  screenClipped,
		})
	}

	if viewportArea > 0 {
		obj.Metrics.CoveragePct = obj.Metrics.ScreenAreaPx / viewportArea * 100
	}
	if len(mesh.Triangles) > 0 && obj.Metrics.RejectedTriangles == len(mesh.Triangles) {
		obj.Metrics.RejectedObjects = 1
	}
	obj.Metrics.AcceptedSourceTriangles = obj.Metrics.SourceTriangles - obj.Metrics.RejectedTriangles
	return obj, nil
}

func needsScreenClip(cam Camera, poly []ClipVertex) bool {
	for _, v := range poly {
		ndc := cam.ProjectView(v.View)
		if math.Abs(ndc.X) > 1+1e-9 || math.Abs(ndc.Y) > 1+1e-9 {
			return true
		}
	}
	return false
}

func addMetrics(dst, src *Metrics) {
	dst.SourceTriangles += src.SourceTriangles
	dst.RejectedTriangles += src.RejectedTriangles
	dst.AcceptedSourceTriangles += src.AcceptedSourceTriangles
	dst.ClippedPolygons += src.ClippedPolygons
	dst.NearClippedTriangles += src.NearClippedTriangles
	dst.VisibleSourceTriangles += src.VisibleSourceTriangles
	dst.VisibleScreenTriangles += src.VisibleScreenTriangles
	dst.RejectedObjects += src.RejectedObjects
	dst.SourceArea3D += src.SourceArea3D
	dst.ClippedArea3D += src.ClippedArea3D
	dst.ScreenAreaPx += src.ScreenAreaPx
	dst.CoveragePct += src.CoveragePct
	dst.NearEdgeLengthPx += src.NearEdgeLengthPx
}

func triangleArea3D(tri [3]Vec3) float64 {
	a := tri[1].Sub(tri[0])
	b := tri[2].Sub(tri[0])
	return a.Cross(b).Len() * 0.5
}

func compareEyes(left, right EyeReport) Comparison {
	nearDelta := left.NearPlaneCenter.Sub(right.NearPlaneCenter)
	return Comparison{
		ScreenAreaDeltaPx:   left.Totals.ScreenAreaPx - right.Totals.ScreenAreaPx,
		ClippedAreaDelta3D:  left.Totals.ClippedArea3D - right.Totals.ClippedArea3D,
		NearEdgeDeltaPx:     left.Totals.NearEdgeLengthPx - right.Totals.NearEdgeLengthPx,
		DistinctNearCenters: nearDelta.Len() > 1e-9,
		DistinctClipResults: left.Signature != right.Signature,
		DistinctCoverage:    math.Abs(left.Totals.CoveragePct-right.Totals.CoveragePct) > 1e-9,
	}
}

func eyeSignature(eye EyeReport) string {
	h := sha256.New()
	for _, obj := range eye.Objects {
		fmt.Fprintf(h, "%s:%d:%d:", obj.Mesh, obj.Metrics.RejectedTriangles, obj.Metrics.VisibleScreenTriangles)
		for _, poly := range obj.Polygons {
			fmt.Fprintf(h, "[%d", poly.SourceTriangle)
			for _, p := range poly.ScreenPolygon {
				fmt.Fprintf(h, "%.6f,%.6f;", p.X, p.Y)
			}
			fmt.Fprint(h, "]")
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}
