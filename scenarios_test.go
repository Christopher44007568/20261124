package motion

import (
	"math"
	"testing"
)

const eps = 1e-9

func camParams() WeightParams {
	return WeightParams{VelScale: 20, DepthSigma: 0.5, RadiusGain: 1.0, MaxRadius: 0.5}
}

// 动态对象使用独立的、更严格的权重参数。
func objParams() WeightParams {
	return WeightParams{VelScale: 10, DepthSigma: 0.2, RadiusGain: 2.0, MaxRadius: 1.0}
}

func lookCam(eye, target Vec3) Camera {
	return Camera{
		View: LookAt(eye, target, Vec3{0, 1, 0}),
		Proj: Perspective(math.Pi/3, 1, 0.1, 100),
	}
}

func stillTracker() *Tracker {
	cam := lookCam(Vec3{0, 0, 0}, Vec3{0, 0, -1})
	return &Tracker{PrevCam: cam, CurrCam: cam, CamParams: camParams(), ObjParams: objParams()}
}

func almost(a, b float64) bool { return math.Abs(a-b) < eps }

// 对照 1:静止目标 + 静止相机,两个来源都应为零速度、满权重。
func TestStaticTarget(t *testing.T) {
	tr := stillTracker()
	obj := Object{Translate(Vec3{0, 0, -5}), Translate(Vec3{0, 0, -5})}

	cs := tr.CameraSource(Vec3{0, 0, -5})
	os := tr.ObjectSource(Vec3{0, 0, 0}, obj)

	if cs.Velocity.Len() > eps || os.Velocity.Len() > eps {
		t.Fatalf("静止场景速度应为零: cam=%v obj=%v", cs.Velocity, os.Velocity)
	}
	if !almost(cs.HistoryWeight, 1) || !almost(os.HistoryWeight, 1) {
		t.Fatalf("静止场景权重应为 1: cam=%v obj=%v", cs.HistoryWeight, os.HistoryWeight)
	}
	if cs.InvalidRadius != 0 || os.InvalidRadius != 0 {
		t.Fatalf("静止场景失效范围应为 0: cam=%v obj=%v", cs.InvalidRadius, os.InvalidRadius)
	}
}

// 对照 2:相机静止、物体平移。只有物体来源有速度,
// 且其失效范围按 ObjParams 独立计算。
func TestMovingObject(t *testing.T) {
	tr := stillTracker()
	obj := Object{Translate(Vec3{0, 0, -5}), Translate(Vec3{0.5, 0, -5})}
	local := Vec3{0, 0, 0}

	cs := tr.CameraSource(Vec3{0.5, 0, -5})
	os := tr.ObjectSource(local, obj)

	if cs.Velocity.Len() > eps {
		t.Fatalf("相机静止时相机来源速度应为零, got %v", cs.Velocity)
	}
	if !almost(cs.HistoryWeight, 1) {
		t.Fatalf("相机来源权重不应受物体运动影响, got %v", cs.HistoryWeight)
	}
	if os.Velocity.X <= 0 {
		t.Fatalf("物体向 +X 运动,物体来源速度应为 +X, got %v", os.Velocity)
	}
	if os.HistoryWeight >= 1 {
		t.Fatalf("运动物体权重应下降, got %v", os.HistoryWeight)
	}
	// 失效范围独立计算:与 ObjParams 手算结果一致。
	wantR := objParams().RadiusGain * os.Velocity.Len()
	if wantR > objParams().MaxRadius {
		wantR = objParams().MaxRadius
	}
	if !almost(os.InvalidRadius, wantR) {
		t.Fatalf("物体失效范围应按 ObjParams 独立计算: got %v want %v", os.InvalidRadius, wantR)
	}
	if os.InvalidRadius <= cs.InvalidRadius {
		t.Fatalf("运动物体失效范围应大于静止相机来源: obj=%v cam=%v", os.InvalidRadius, cs.InvalidRadius)
	}
}

// 对照 3:快速转头、物体静止。只有相机来源有速度。
func TestFastHeadTurn(t *testing.T) {
	prev := lookCam(Vec3{0, 0, 0}, Vec3{0, 0, -1})
	yaw := 0.3
	curr := lookCam(Vec3{0, 0, 0}, Vec3{math.Sin(yaw), 0, -math.Cos(yaw)})
	tr := &Tracker{PrevCam: prev, CurrCam: curr, CamParams: camParams(), ObjParams: objParams()}

	p := Vec3{0, 0, -5}
	cs := tr.CameraSource(p)
	os := tr.ObjectSource(Vec3{0, 0, 0}, Object{Translate(p), Translate(p)})

	if os.Velocity.Len() > eps {
		t.Fatalf("物体静止时物体来源速度应为零, got %v", os.Velocity)
	}
	if !almost(os.HistoryWeight, 1) {
		t.Fatalf("物体来源权重不应受转头影响, got %v", os.HistoryWeight)
	}
	if cs.Velocity.Len() < 0.3 {
		t.Fatalf("快速转头应产生大的相机速度, got %v", cs.Velocity)
	}
	if cs.HistoryWeight > 0.5 {
		t.Fatalf("快速转头时相机来源权重应显著下降, got %v", cs.HistoryWeight)
	}
	if cs.InvalidRadius <= 0 {
		t.Fatalf("快速转头时相机失效范围应为正, got %v", cs.InvalidRadius)
	}
}

// 对照 4:相机 +X、物体 -X 反向运动。两个来源方向一致(屏幕上都偏 -X),
// 合成运动大于任一单源,且相机来源与物体是否运动无关。
func TestCounterMotion(t *testing.T) {
	prev := lookCam(Vec3{0, 0, 0}, Vec3{0, 0, -1})
	curr := lookCam(Vec3{0.3, 0, 0}, Vec3{0.3, 0, -1})
	tr := &Tracker{PrevCam: prev, CurrCam: curr, CamParams: camParams(), ObjParams: objParams()}

	obj := Object{Translate(Vec3{0, 0, -5}), Translate(Vec3{-0.3, 0, -5})}
	local := Vec3{0, 0, 0}

	cs := tr.CameraSource(Vec3{-0.3, 0, -5})
	os := tr.ObjectSource(local, obj)
	comb := tr.Combined(local, obj)

	if cs.Velocity.X >= 0 || os.Velocity.X >= 0 {
		t.Fatalf("反向运动时两源屏幕速度都应为 -X: cam=%v obj=%v", cs.Velocity, os.Velocity)
	}
	if comb.Velocity.X >= cs.Velocity.X || comb.Velocity.X >= os.Velocity.X {
		t.Fatalf("合成速度应比任一单源更负: comb=%v cam=%v obj=%v", comb.Velocity, cs.Velocity, os.Velocity)
	}
	// 相机来源不读物体矩阵:物体动与不动,结果必须一致。
	csStill := tr.CameraSource(Vec3{-0.3, 0, -5})
	if csStill != cs {
		t.Fatalf("相机来源不应受物体运动影响")
	}
	// 合成权重取两源较小者,失效范围取较大者。
	if comb.HistoryWeight > cs.HistoryWeight || comb.HistoryWeight > os.HistoryWeight {
		t.Fatalf("合成权重不应高于任一单源: comb=%v cam=%v obj=%v",
			comb.HistoryWeight, cs.HistoryWeight, os.HistoryWeight)
	}
}

// 契约:物体矩阵每帧只应用一次,绝不叠加进相机矩阵。
func TestObjectMatrixNotDoubleApplied(t *testing.T) {
	prev := lookCam(Vec3{0, 0, 0}, Vec3{0, 0, -1})
	curr := lookCam(Vec3{0.1, 0, 0}, Vec3{0.1, 0, -1})
	tr := &Tracker{PrevCam: prev, CurrCam: curr, CamParams: camParams(), ObjParams: objParams()}

	obj := Object{Translate(Vec3{0, 0, -5}), Translate(Vec3{0.4, 0, -5})}
	local := Vec3{0.2, 0.1, 0}

	comb := tr.Combined(local, obj)

	// 正确路径:每帧各应用一次模型矩阵。
	pn, _ := tr.PrevCam.Project(obj.PrevModel.TransformPoint(local))
	cn, _ := tr.CurrCam.Project(obj.CurrModel.TransformPoint(local))
	want := cn.Sub(pn)
	if comb.Velocity.Sub(want).Len() > eps {
		t.Fatalf("合成速度应等于单次应用模型矩阵的重投影: got %v want %v", comb.Velocity, want)
	}

	// 错误路径(重复叠加):当前帧多应用一次模型矩阵,结果必须不同。
	double := obj.CurrModel.TransformPoint(obj.CurrModel.TransformPoint(local))
	badN, _ := tr.CurrCam.Project(double)
	bad := badN.Sub(pn)
	if bad.Sub(comb.Velocity).Len() < 1e-3 {
		t.Fatalf("物体矩阵疑似被重复叠加到当前帧: comb=%v double=%v", comb.Velocity, bad)
	}
}
