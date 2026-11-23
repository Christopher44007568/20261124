package stereo

func scale(s float64) Mat4 {
	m := identity()
	m[0], m[5], m[10] = s, s, s
	return m
}

func compose(pos Vec3, s float64) Mat4 {
	return mul(translate(pos.X, pos.Y, pos.Z), scale(s))
}

// BuildScene 构造覆盖三类要求的同一场景：
// 共享实例、跨眼不同裁剪、非对称投影、跨眼翻转的透明顺序。
func BuildScene(w, h int) *Scene {
	cubeA := NewCube("cubeA", Vec3{0.85, 0.35, 0.25}, false, 1)
	cubeB := NewCube("cubeB", Vec3{0.30, 0.55, 0.90}, false, 1)
	cubeC := NewCube("cubeC", Vec3{0.35, 0.80, 0.40}, false, 1)
	glassR := NewQuad("glassR", Vec3{0.95, 0.25, 0.25}, 0.55)
	glassB := NewQuad("glassB", Vec3{0.25, 0.45, 0.95}, 0.45)
	clipTri := NewClipTri("clipTri")

	sc := &Scene{Width: w, Height: h}
	// 三个不透明共享实例，两条路径拿到的是同一组 *Mesh 指针。
	sc.Instances = append(sc.Instances,
		Instance{cubeA, compose(Vec3{-1.6, 0.1, -4.2}, 0.8), Vec3{1, 1, 1}},
		Instance{cubeB, compose(Vec3{1.5, -0.2, -5.4}, 1.0), Vec3{1, 1, 1}},
		Instance{cubeC, compose(Vec3{0, 1.1, -7.0}, 1.3), Vec3{1, 1, 1}},
	)
	// 两个透明片沿 X 错开且深度接近：左右眼横向偏移后相对深度次序翻转。
	sc.Instances = append(sc.Instances,
		Instance{glassR, mul(translate(-0.55, 0, -3.4), scale(0.9)), Vec3{1, 1, 1}},
		Instance{glassB, mul(translate(0.55, 0, -3.6), scale(0.9)), Vec3{1, 1, 1}},
	)
	// 跨眼裁剪三角：左眼左顶点落在其视锥左边界之外，右眼视锥更靠左而完整保留。
	sc.Instances = append(sc.Instances,
		Instance{clipTri, mul(translate(0, 0, -3.0), scale(1.0)), Vec3{1, 1, 1}},
	)

	sc.Eyes[Left] = eyeParams(Left, 0.64, 1.0, 20.0, 60, 0.0)
	sc.Eyes[Right] = eyeParams(Right, 0.64, 1.0, 20.0, 60, 0.18)
	return sc
}

// StaleMatrices 模拟“上一帧”的双眼矩阵：整体前推且无剪切，
// 故障注入时右眼将错误地沿用它。
func StaleMatrices(sc *Scene) [2]Mat4 {
	stale := eyeParams(Right, 0.64, 1.0, 20.0, 60, 0.0)
	shift := 0.9
	drift := translate(0, 0, shift)
	return [2]Mat4{
		mul(drift, sc.Eyes[Left].ViewProj),
		mul(drift, stale.ViewProj),
	}
}

// Coverage 汇总需求覆盖证据。
type Coverage struct {
	ClipCounts        [2]int
	AsymmetricFrustum [2][6]float64 // l,r,b,t,n,f
	SharedMeshCount   int
	TransOrderL       []string
	TransOrderR       []string
}

// CollectCoverage 从场景与参考帧中提取覆盖证据。
func CollectCoverage(sc *Scene, ref PathResult) Coverage {
	cov := Coverage{
		ClipCounts:      ref.Counters.ClippedTris,
		SharedMeshCount: len(uniqueMeshes(sc)),
		TransOrderL:     ref.Frames[Left].TransOrder,
		TransOrderR:     ref.Frames[Right].TransOrder,
	}
	for e := Left; e <= Right; e++ {
		cov.AsymmetricFrustum[e] = sc.Eyes[e].Frustum
	}
	return cov
}

func uniqueMeshes(sc *Scene) map[*Mesh]bool {
	m := map[*Mesh]bool{}
	for i := range sc.Instances {
		m[sc.Instances[i].Mesh] = true
	}
	return m
}
