package main

import (
	"fmt"
	"math"
	"os"
)

// Stereo rig constants shared by the scenarios.
const (
	ipd      = 0.2
	fovY     = math.Pi / 2
	aspect   = 1.5
	nearDist = 0.1
	farDist  = 100.0
	tanHalfV = 1.0 // tan(fovY/2)
	tanHalfH = tanHalfV * aspect
	renderW  = 96
	renderH  = 64
)

// Rig holds both eyes' cameras and frusta.
type Rig struct {
	Cams   [2]Camera
	Frusta [2]Frustum
}

func NewRig() Rig {
	var r Rig
	proj := Perspective(fovY, aspect, nearDist, farDist)
	for i, ex := range [2]float64{-ipd / 2, ipd / 2} {
		eye := Vec3{ex, 0, 0}
		view := LookAt(eye, eye.Add(Vec3{0, 0, -1}), Vec3{0, 1, 0})
		r.Cams[i] = Camera{Eye: eye, View: view, Proj: proj, Near: nearDist, Far: farDist, FovY: fovY, Aspect: aspect}
		r.Frusta[i] = ExtractFrustum(Mul(proj, view))
	}
	return r
}

func main() {
	rig := NewRig()
	grand := Counts{}
	ok := true
	for _, sc := range allScenarios() {
		id := 0
		bound := Flatten(sc.build(&id))
		records, counts := ClassifyAll(bound, rig.Frusta)
		fmt.Printf("== 场景: %s (实例数 %d) ==\n", sc.name, len(bound))
		for _, r := range records {
			fmt.Printf("  [%-5s] %-20s 左眼=%s 右眼=%s\n",
				r.Class, r.Inst.Name, r.Evidence[EyeLeft], r.Evidence[EyeRight])
		}
		fmt.Printf("  计数: 双眼=%d 仅左=%d 仅右=%d 双眼不可见=%d 合计=%d/%d\n",
			counts.Both, counts.LeftOnly, counts.RightOnly, counts.Neither, counts.Sum(), counts.Total)
		if !counts.Closed() {
			fmt.Println("  闭合校验: 失败!")
			ok = false
		} else {
			fmt.Println("  闭合校验: OK")
		}
		grand.Both += counts.Both
		grand.LeftOnly += counts.LeftOnly
		grand.RightOnly += counts.RightOnly
		grand.Neither += counts.Neither
		grand.Total += counts.Total

		for eye := 0; eye < 2; eye++ {
			dl := DrawList(records, eye)
			fmt.Printf("  %s轻量绘制列表(%d): %v\n", eyeNames[eye], len(dl), drawNames(records, dl))
		}
		if !compareWithRaster(rig, bound, records) {
			ok = false
		}
		fmt.Println()
	}
	fmt.Printf("总计: 双眼=%d 仅左=%d 仅右=%d 双眼不可见=%d 合计=%d/%d\n",
		grand.Both, grand.LeftOnly, grand.RightOnly, grand.Neither, grand.Sum(), grand.Total)
	if !grand.Closed() {
		fmt.Println("总计闭合校验: 失败!")
		ok = false
	} else {
		fmt.Println("总计闭合校验: OK")
	}
	if !ok {
		os.Exit(1)
	}
}

func drawNames(records []Record, ids []int) []string {
	byID := map[int]string{}
	for _, r := range records {
		byID[r.Inst.ID] = r.Inst.Name
	}
	names := make([]string, len(ids))
	for i, id := range ids {
		names[i] = byID[id]
	}
	return names
}

// compareWithRaster rasterizes both eyes and matches coverage against the
// lightweight draw lists. Returns false on any MismatchMiss (culling bug).
func compareWithRaster(rig Rig, bound []BoundInstance, records []Record) bool {
	ok := true
	for eye := 0; eye < 2; eye++ {
		coverage, _ := RasterizeEye(rig.Cams[eye], bound, renderW, renderH)
		verdicts := CompareEye(rig.Cams[eye], eye, records, bound, coverage)
		sum := Summarize(verdicts)
		fmt.Printf("  %s光栅对比: 一致-可见=%d 一致-不可见=%d 失配-遮挡/亚像素=%d 失配-漏画=%d\n",
			eyeNames[eye], sum[AgreeVisible], sum[AgreeHidden], sum[MismatchOccluded], sum[MismatchMiss])
		for _, v := range verdicts {
			if v.Kind == MismatchOccluded || v.Kind == MismatchMiss {
				fmt.Println(v)
			}
			if v.Kind == MismatchMiss {
				ok = false
			}
		}
	}
	return ok
}
