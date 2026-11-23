package main

import (
	"fmt"

	"posebuffer/pose"
)

const duration = pose.Timestamp(4_000_000_000) // 4 s of simulated data

func main() {
	fmt.Println("pose history-buffer evaluation")
	fmt.Printf("camera query rate : 120 Hz (period %.3f ms)\n", float64(cameraPeriod)/1e6)
	fmt.Printf("input rates       : mixed 90/120 Hz, simulated duration 4 s\n")
	fmt.Printf("loss window       : 200 ms, extrapolation budget: 25 ms\n\n")

	base := baseStream(duration, 1_000_000) // +-1 ms scheduling jitter
	for _, sc := range buildScenarios(duration) {
		stream := sc.Apply(append([]pose.Sample(nil), base...))
		results, stats := Run(sc, stream, duration)

		fmt.Printf("== %s (domain %q, %d arrivals) ==\n", sc.Name, sc.Domain, len(stream))
		fmt.Printf("buffer: inserts=%d duplicates=%d evictions=%d rollbacks=%d "+
			"gaps>limit=%d largestGap=%.1f ms unavailable=%d\n",
			stats.Inserts, stats.Duplicates, stats.Evictions, stats.ClockRollbacks,
			stats.GapsOverLimit, float64(stats.LargestGap)/1e6, stats.Unavailable)
		fmt.Printf("  %-16s %8s %10s %10s %10s %10s %9s\n",
			"estimator", "samples", "unavail", "posRMSE", "posMax", "rotRMSE", "rotMax")
		for _, r := range results {
			fmt.Printf("  %-16s %8d %10d %10.2e %10.2e %10.2e %10.2e\n",
				r.Name, r.Err.N, r.Err.Unavailable,
				r.Err.PosRMSE(), r.Err.PosMax, r.Err.RotRMSE(), r.Err.RotMax)
		}
		fmt.Println()
	}

	domainIsolationDemo()
}

func domainIsolationDemo() {
	hub := pose.NewHub(pose.Config{Capacity: 1024, Extrapolate: 25_000_000, MaxGap: 150_000_000})
	tr := NewTrajectory()

	cam, lidar := pose.Domain("camera_120hz"), pose.Domain("lidar_20hz")
	camStart := pose.Timestamp(1_000_000_000)
	camOnly := camStart + period120 // 1.008 s: only the camera domain holds data here
	for t := camStart; t < duration; t += period120 {
		hub.Insert(cam, tr.at(t))
	}
	for t := camStart + 200_000_000; t < duration; t += 50_000_000 {
		hub.Insert(lidar, tr.at(t+3_000_000)) // offset clock, never reconciled
	}

	q := pose.Timestamp(2_500_000_000) // inside both windows
	camHit := hub.Buffer(cam).Query(q).Status
	lidarMiss := hub.Buffer(lidar).Query(duration + 30_000_000).Status
	fmt.Println("== clock-domain isolation ==")
	fmt.Printf("camera-domain query @%.3fs: status %s\n", float64(q)/1e9, pose.StatusName(camHit))
	fmt.Printf("lidar-domain  query @%.3fs: status %s (cannot see camera data)\n",
		float64(duration+30_000_000)/1e9, pose.StatusName(lidarMiss))
	leakCheck := hub.Buffer(lidar).Query(camOnly).Status
	fmt.Printf("lidar-domain  query @%.3fs: status %s (camera-only instant, no cross-domain leak)\n",
		float64(camOnly)/1e9, pose.StatusName(leakCheck))
	fmt.Printf("registered domains: %v\n", hub.Domains())
}
