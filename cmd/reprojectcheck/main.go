package main

import (
	"fmt"

	"reprojectcheck/internal/relight"
)

func main() {
	scene := relight.BuildScene()
	oldCamera := relight.OldCamera()
	currentCamera := relight.CurrentCamera()
	oldLight := relight.OldLight()
	currentLight := relight.CurrentLight()

	oldFrame := relight.RenderFrame(scene, oldCamera, oldLight)
	truth := relight.RenderFrame(scene, currentCamera, currentLight)
	currentDepth := &relight.DepthImage{
		Width:  truth.Width,
		Height: truth.Height,
		Depth:  truth.Depth,
	}

	naive := relight.ReprojectUnifiedWarp(scene, oldFrame, oldCamera, currentCamera, currentDepth, false, 0.35)
	depthTestedWarp := relight.ReprojectUnifiedWarp(scene, oldFrame, oldCamera, currentCamera, currentDepth, true, 0.35)
	separated := relight.ReprojectSeparated(scene, oldFrame, oldCamera, currentCamera, currentDepth, currentLight, 0.35)

	rows := []relight.Metrics{
		relight.Compare("unified-warp/no-depth", scene, naive, truth, oldCamera, currentCamera),
		relight.Compare("unified-warp/depth-tested", scene, depthTestedWarp, truth, oldCamera, currentCamera),
		relight.Compare("separated-reprojection", scene, separated, truth, oldCamera, currentCamera),
	}

	fmt.Println("method                         mean      max       primary   shadow    reflect   stale  occluders")
	for _, row := range rows {
		fmt.Printf("%-30s %-9.5f %-9.5f %-9.5f %-9.5f %-9.5f %-6d %d\n",
			row.Name,
			row.MeanColorError,
			row.MaxColorError,
			row.PrimaryError,
			row.ShadowError,
			row.ReflectionError,
			row.StalePixels,
			row.BlockedSamples,
		)
	}
	fmt.Printf("old reflection luminance %.5f\n", relight.ReflectionLuminanceMax(oldFrame))
	fmt.Printf("separated reflection luminance %.5f\n", relight.ReflectionLuminanceMax(separated))
	fmt.Printf("current reflection luminance %.5f\n", relight.ReflectionLuminanceMax(truth))
}
