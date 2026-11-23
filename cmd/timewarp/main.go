package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"

	"timewarp/geom"
	"timewarp/metrics"
	"timewarp/report"
	"timewarp/scene"
	"timewarp/warp"
)

func main() {
	out := flag.String("out", "out", "directory for PNG diagnostics and report.html")
	width := flag.Int("width", 640, "frame width")
	height := flag.Int("height", 360, "frame height")
	serve := flag.String("serve", "", "optional HTTP listen address, for example :8080")
	flag.Parse()
	if *width < 160 || *height < 90 {
		log.Fatal("width must be >= 160 and height >= 90")
	}

	if err := build(*out, *width, *height); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote %s/report.html\n", *out)
	fmt.Printf("open with: xdg-open %s/report.html\n", *out)

	if *serve != "" {
		log.Printf("serving %s at http://localhost%s/", *out, *serve)
		log.Fatal(http.ListenAndServe(*serve, http.FileServer(http.Dir(*out))))
	}
}

func build(out string, w, h int) error {
	r := scene.NewRenderer(w, h)
	const deg = math.Pi / 180
	from := geom.NewPose(0, 0, geom.Vec3{})
	turnPose := geom.NewPose(12*deg, -2*deg, geom.Vec3{})
	dollyPose := geom.NewPose(0, 0, geom.Vec3{Z: 0.68})

	source := r.Render(from, true)
	turnGT := r.Render(turnPose, false)
	dollyGT := r.Render(dollyPose, false)
	src := warp.SourceFromFrame(source)

	turnRot := warp.RotationWarp(src, r.In, from, turnPose)
	turnDepth := warp.DepthWarp(src, r.In, from, turnPose)
	dollyRot := warp.RotationWarp(src, r.In, from, dollyPose)
	dollyDepth := warp.DepthWarp(src, r.In, from, dollyPose)

	turnRotStats := metrics.Compute("pure rotation", turnRot, turnGT)
	turnDepthStats := metrics.Compute("depth reprojection", turnDepth, turnGT)
	dollyRotStats := metrics.Compute("pure rotation", dollyRot, dollyGT)
	dollyDepthStats := metrics.Compute("depth reprojection", dollyDepth, dollyGT)

	data := report.Data{
		In:        r.In,
		FOVYDeg:   r.FOVY / deg,
		Source:    source,
		PoleWorld: scene.PoleCenter,
		Cases: []report.Case{
			{
				ID:     "turn",
				Title:  "快速转头 / pure rotational motion",
				Motion: "yaw +12.0°, pitch -2.0°, camera position unchanged",
				Note:   "快速向右转头时，旋转映射本身与深度无关。纯旋转应贴合目标；深度路径用源深度重建同一视角，并在遮挡、缺失深度与画面边缘留下拒绝孔。",
				From:   from, To: turnPose, GT: turnGT,
				Rotation: turnRot, Depth: turnDepth,
				RotStats: turnRotStats, DepthStat: turnDepthStats,
			},
			{
				ID:     "dolly",
				Title:  "头部前移 / forward translation",
				Motion: "camera position +Z 0.68, yaw = pitch = 0",
				Note:   "头部前移时，纯旋转仍输出与源帧一致的画面，近杆相对背墙不会产生视差。深度重投影按每像素深度展开：近杆放大并横移，背景保持正确的远近关系。",
				From:   from, To: dollyPose, GT: dollyGT,
				Rotation: dollyRot, Depth: dollyDepth,
				RotStats: dollyRotStats, DepthStat: dollyDepthStats,
			},
		},
	}
	if err := report.Build(out, data); err != nil {
		return err
	}
	if _, err := os.Stat(out); err != nil {
		return err
	}

	fmt.Printf("turn   rotation: MAE %.2f coverage %.1f%% OOB %d\n",
		turnRotStats.MAE, turnRotStats.Coverage*100, turnRotStats.OutOfBounds)
	fmt.Printf("turn   depth:    MAE %.2f coverage %.1f%% holes %d sourceOffscreen %d\n",
		turnDepthStats.MAE, turnDepthStats.Coverage*100, turnDepthStats.NoCoverage, turnDepthStats.SourceOffscreen)
	fmt.Printf("dolly  rotation: MAE %.2f nearPole %.2f coverage %.1f%%\n",
		dollyRotStats.MAE, regionMAE(dollyRotStats, "near pole"), dollyRotStats.Coverage*100)
	fmt.Printf("dolly  depth:    MAE %.2f nearPole %.2f coverage %.1f%% holes %d\n",
		dollyDepthStats.MAE, regionMAE(dollyDepthStats, "near pole"), dollyDepthStats.Coverage*100, dollyDepthStats.NoCoverage)
	fmt.Printf("depth  OOB samples: turn=%d dolly=%d (must be zero)\n",
		turnDepthStats.OutOfBounds, dollyDepthStats.OutOfBounds)
	return nil
}

func regionMAE(s metrics.Summary, name string) float64 {
	for _, r := range s.Regions {
		if r.Name == name {
			return r.MAE
		}
	}
	return 0
}
