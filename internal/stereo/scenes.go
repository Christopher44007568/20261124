package stereo

import "fmt"

const (
	viewportWidth  = 640
	viewportHeight = 480
	nearPlane      = 0.16
	ipd            = 0.064
)

// SceneIDs lists the built-in acceptance scenes.
var SceneIDs = []string{"large-plane", "long-rod", "near-sheet"}

// Scenes builds fresh scene geometry. Every call creates both eye cameras and
// all meshes independently, which keeps server requests free of shared state.
func Scenes() ([]Scene, error) {
	largePlane := Scene{
		ID:    "large-plane",
		Title: "大平面 · 仅单眼穿近面",
		Description: "倾斜的大平面完整位于左眼近面前方；右眼因会聚视角从平面中穿过，" +
			"裁掉大部分三角面。",
		Notes: []string{
			"左眼视锥 [-0.548, 0.612]；右眼视锥 [-0.612, 0.548]。",
			"双眼会聚，因此近面中心分别计算且彼此不同。",
		},
		Meshes: []Mesh{largePlaneMesh()},
	}
	rod := Scene{
		ID:    "long-rod",
		Title: "长杆 · 从相机后方插入",
		Description: "十二边长杆从相机后方延伸至场景深处。双眼都会裁剪，但截环位置、" +
			"屏幕覆盖和可见边缘不同。",
		Notes: []string{
			"杆起点为 z=-0.10，不会被误当作近面透明覆盖片。",
			"侧面与端盖三角形都按眼独立裁剪。",
		},
		Meshes: []Mesh{longRodMesh()},
	}
	sheet := Scene{
		ID:          "near-sheet",
		Title:       "透明近面片 · 半入裁剪面",
		Description: "宽透明片横跨近裁剪面。透明度只影响显示，近面裁剪、屏幕覆盖与面积仍按眼精确计算。",
		Notes: []string{
			"28% 透明度的屏幕多边形仍按完整几何面积计入覆盖。",
			"画布红色截边与近面边长指标来自同一条裁剪边。",
		},
		Meshes: []Mesh{nearSheetMesh()},
	}

	scenes := []Scene{largePlane, rod, sheet}
	for i := range scenes {
		cameras, err := eyeCameras()
		if err != nil {
			return nil, err
		}
		scenes[i].Eyes = cameras
		scenes[i].ViewportW = viewportWidth
		scenes[i].ViewportH = viewportHeight
	}
	return scenes, nil
}

// SceneInfos returns stable scenario metadata for the browser.
func SceneInfos() ([]SceneInfo, error) {
	scenes, err := Scenes()
	if err != nil {
		return nil, err
	}
	out := make([]SceneInfo, len(scenes))
	for i, scene := range scenes {
		out[i] = SceneInfo{ID: scene.ID, Title: scene.Title, Description: scene.Description}
	}
	return out, nil
}

// FindScene returns one built-in scene by ID.
func FindScene(id string) (Scene, error) {
	scenes, err := Scenes()
	if err != nil {
		return Scene{}, err
	}
	for _, scene := range scenes {
		if scene.ID == id {
			return scene, nil
		}
	}
	return Scene{}, fmt.Errorf("unknown scene %q", id)
}

func eyeCameras() ([2]Camera, error) {
	target := Vec3{0, 0, 1.2}
	up := Vec3{0, 1, 0}
	leftFrustum := Frustum{Left: -0.548, Right: 0.612, Bottom: -0.42, Top: 0.42}
	rightFrustum := Frustum{Left: -0.612, Right: 0.548, Bottom: -0.42, Top: 0.42}
	left, err := NewCamera("left", Vec3{-ipd / 2, 0, 0}, target, up, nearPlane, 12, leftFrustum, viewportWidth, viewportHeight)
	if err != nil {
		return [2]Camera{}, err
	}
	right, err := NewCamera("right", Vec3{ipd / 2, 0, 0}, target, up, nearPlane, 12, rightFrustum, viewportWidth, viewportHeight)
	if err != nil {
		return [2]Camera{}, err
	}
	return [2]Camera{left, right}, nil
}

func largePlaneMesh() Mesh {
	mesh := Mesh{
		Name: "large tilted plane",
		Material: Material{
			Color:   "#7c8cff",
			Opacity: 0.88,
		},
	}
	const cols, rows = 8, 6
	x0, x1 := 0.0, 1.34
	y0, y1 := -1.05, 1.05
	point := func(u, v float64) Vec3 {
		x := x0 + (x1-x0)*u
		y := y0 + (y1-y0)*v
		z := nearPlane - 0.0005*x - 0.00012*y
		return Vec3{x, y, z}
	}
	addGrid(&mesh, point, cols, rows)
	return mesh
}

func longRodMesh() Mesh {
	mesh := Mesh{
		Name: "long rod",
		Material: Material{
			Color:   "#ffb547",
			Opacity: 1,
		},
	}
	start := Vec3{-0.18, 0.01, -0.10}
	end := Vec3{0.28, 0.20, 1.30}
	addCylinder(&mesh, start, end, 0.024, 12)
	return mesh
}

func nearSheetMesh() Mesh {
	mesh := Mesh{
		Name: "transparent near sheet",
		Material: Material{
			Color:       "#50e3c2",
			Opacity:     0.28,
			Transparent: true,
		},
	}
	const cols, rows = 7, 5
	x0, x1 := -0.78, 0.78
	y0, y1 := -0.56, 0.56
	point := func(u, v float64) Vec3 {
		x := x0 + (x1-x0)*u
		y := y0 + (y1-y0)*v
		z := nearPlane - 0.001 + 0.003*x + 0.001*y
		return Vec3{x, y, z}
	}
	addGrid(&mesh, point, cols, rows)
	return mesh
}
