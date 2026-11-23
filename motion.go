package motion

import "math"

// Camera 是一帧的相机状态。相机矩阵中绝不混入物体模型矩阵:
// 物体矩阵只在 ObjectSource / Combined 里按帧各应用一次。
type Camera struct {
	View Mat4
	Proj Mat4
}

// ViewProj 返回 Proj*View。
func (c Camera) ViewProj() Mat4 { return c.Proj.Mul(c.View) }

// Project 把世界点投到 NDC,并返回视图空间深度(相机前方为正)。
func (c Camera) Project(world Vec3) (ndc Vec2, depth float64) {
	clip := c.ViewProj().TransformPoint(world)
	v := c.View.TransformPoint(world)
	return Vec2{clip.X, clip.Y}, -v.Z
}

// Object 是动态对象在相邻两帧的模型矩阵。
// 静止对象 PrevModel == CurrModel == Identity。
type Object struct {
	PrevModel Mat4
	CurrModel Mat4
}

// StaticObject 返回静止对象。
func StaticObject() Object { return Object{Identity(), Identity()} }

// WeightParams 控制历史权重与失效范围的衰减,相机与物体各持有一份,
// 因此动态对象的失效范围是独立计算的。
type WeightParams struct {
	VelScale   float64 // 速度对权重的衰减系数
	DepthSigma float64 // 深度差容忍度
	RadiusGain float64 // 失效范围随速度的增益
	MaxRadius  float64 // 失效范围上限(NDC)
}

// SourceSample 是单个独立运动源在一帧的记录。
type SourceSample struct {
	Velocity      Vec2    // 屏幕空间速度(NDC/帧)
	DepthDelta    float64 // 视图空间深度差( curr - prev )
	HistoryWeight float64 // [0,1],越小越应拒绝历史
	InvalidRadius float64 // 历史失效范围(NDC),按来源独立计算
}

func makeSample(prevNdc, currNdc Vec2, prevDepth, currDepth float64, p WeightParams) SourceSample {
	v := currNdc.Sub(prevNdc)
	dz := currDepth - prevDepth
	w := 1 / (1 + p.VelScale*v.Len())
	w *= math.Exp(-math.Abs(dz) / p.DepthSigma)
	r := p.RadiusGain*v.Len() + math.Abs(dz)/p.DepthSigma*p.MaxRadius*0.5
	if r > p.MaxRadius {
		r = p.MaxRadius
	}
	return SourceSample{Velocity: v, DepthDelta: dz, HistoryWeight: w, InvalidRadius: r}
}

// Tracker 用相邻两帧的相机状态,把运动拆成独立来源。
type Tracker struct {
	PrevCam Camera
	CurrCam Camera

	CamParams WeightParams // 相机(头动)来源的权重参数
	ObjParams WeightParams // 动态对象来源的权重参数(独立)
}

// CameraSource 只反映相机运动:对同一个世界点,
// 用前一帧与当前帧的相机矩阵分别投影取差。
// 不读取任何物体矩阵,因此物体运动不可能泄漏进相机来源。
func (t *Tracker) CameraSource(world Vec3) SourceSample {
	pn, pd := t.PrevCam.Project(world)
	cn, cd := t.CurrCam.Project(world)
	return makeSample(pn, cn, pd, cd, t.CamParams)
}

// ObjectSource 只反映物体自身运动:相机冻结在当前帧,
// 同一局部点分别经 PrevModel / CurrModel 变换后投影取差。
func (t *Tracker) ObjectSource(local Vec3, obj Object) SourceSample {
	pw := obj.PrevModel.TransformPoint(local)
	cw := obj.CurrModel.TransformPoint(local)
	pn, pd := t.CurrCam.Project(pw)
	cn, cd := t.CurrCam.Project(cw)
	return makeSample(pn, cn, pd, cd, t.ObjParams)
}

// Combined 是实际重投影用的合成运动:
// 前一帧 = PrevCam*(PrevModel*local),当前帧 = CurrCam*(CurrModel*local)。
// 每帧的物体矩阵只应用一次,不会叠加进相机矩阵。
func (t *Tracker) Combined(local Vec3, obj Object) SourceSample {
	pw := obj.PrevModel.TransformPoint(local)
	cw := obj.CurrModel.TransformPoint(local)
	pn, pd := t.PrevCam.Project(pw)
	cn, cd := t.CurrCam.Project(cw)
	// 合成样本的失效范围取两个来源的较大者,权重取较小者。
	cs := t.CameraSource(cw)
	os := t.ObjectSource(local, obj)
	s := makeSample(pn, cn, pd, cd, t.CamParams)
	if os.InvalidRadius > cs.InvalidRadius {
		s.InvalidRadius = os.InvalidRadius
	} else {
		s.InvalidRadius = cs.InvalidRadius
	}
	if os.HistoryWeight < cs.HistoryWeight {
		s.HistoryWeight = os.HistoryWeight
	} else {
		s.HistoryWeight = cs.HistoryWeight
	}
	return s
}
