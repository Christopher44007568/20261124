// demo 打印四组对照场景下,相机来源与物体来源分离后的
// 屏幕速度、深度差、历史权重与失效范围。
package main

import (
	"fmt"
	"math"

	motion "uid5-0037"
)

func v(x, y, z float64) motion.Vec3 { return motion.Vec3{X: x, Y: y, Z: z} }

func lookCam(eye, target motion.Vec3) motion.Camera {
	return motion.Camera{
		View: motion.LookAt(eye, target, v(0, 1, 0)),
		Proj: motion.Perspective(math.Pi/3, 1, 0.1, 100),
	}
}

func report(name string, tr *motion.Tracker, local motion.Vec3, obj motion.Object) {
	world := obj.CurrModel.TransformPoint(local)
	cs := tr.CameraSource(world)
	os := tr.ObjectSource(local, obj)
	cb := tr.Combined(local, obj)
	fmt.Printf("== %s ==\n", name)
	row := func(tag string, s motion.SourceSample) {
		fmt.Printf("  %-8s vel=(%+.4f,%+.4f) |v|=%.4f  dz=%+.4f  w=%.4f  R=%.4f\n",
			tag, s.Velocity.X, s.Velocity.Y, s.Velocity.Len(),
			s.DepthDelta, s.HistoryWeight, s.InvalidRadius)
	}
	row("camera", cs)
	row("object", os)
	row("combined", cb)
	fmt.Println()
}

func main() {
	camP := motion.WeightParams{VelScale: 20, DepthSigma: 0.5, RadiusGain: 1.0, MaxRadius: 0.5}
	objP := motion.WeightParams{VelScale: 10, DepthSigma: 0.2, RadiusGain: 2.0, MaxRadius: 1.0}
	still := lookCam(v(0, 0, 0), v(0, 0, -1))
	local := v(0, 0, 0)

	tr := &motion.Tracker{PrevCam: still, CurrCam: still, CamParams: camP, ObjParams: objP}
	report("静止目标", tr, local, motion.Object{
		PrevModel: motion.Translate(v(0, 0, -5)),
		CurrModel: motion.Translate(v(0, 0, -5)),
	})
	report("移动物体", tr, local, motion.Object{
		PrevModel: motion.Translate(v(0, 0, -5)),
		CurrModel: motion.Translate(v(0.5, 0, -5)),
	})

	yaw := 0.3
	turned := lookCam(v(0, 0, 0), v(math.Sin(yaw), 0, -math.Cos(yaw)))
	tr = &motion.Tracker{PrevCam: still, CurrCam: turned, CamParams: camP, ObjParams: objP}
	report("快速转头", tr, local, motion.Object{
		PrevModel: motion.Translate(v(0, 0, -5)),
		CurrModel: motion.Translate(v(0, 0, -5)),
	})

	moved := lookCam(v(0.3, 0, 0), v(0.3, 0, -1))
	tr = &motion.Tracker{PrevCam: still, CurrCam: moved, CamParams: camP, ObjParams: objP}
	report("反向运动", tr, local, motion.Object{
		PrevModel: motion.Translate(v(0, 0, -5)),
		CurrModel: motion.Translate(v(-0.3, 0, -5)),
	})
}
