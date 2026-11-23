package main

// QueryResult holds per-frame culling output for both eyes.
type QueryResult struct {
	CulledShared map[int]bool // culled by the shared layer (applies to both eyes)
	CulledPerEye [2]map[int]bool
	Metrics      FrameMetrics
}

const (
	maxFallbackExpansions = 3
	expansionStep         = 0.5 // radius multiplier increment per expansion
)

// RunQuery executes the two-layer occlusion query for one stereo frame.
//
// Layer 1 (shared): thick walls, must be conservative for BOTH eyes; an
// object is only culled when occluded from each eye individually.
// Layer 2 (complement): thin pillars / transparent sheets, evaluated per
// eye. When a per-eye occluder only partially covers an object (the
// single-eye occluder "fails"), the query range is expanded and retried;
// if still not fully covered the object is conservatively kept visible.
func RunQuery(scene *Scene, rig *StereoRig, frame int) *QueryResult {
	res := &QueryResult{
		CulledShared: map[int]bool{},
		CulledPerEye: [2]map[int]bool{{}, {}},
	}
	res.Metrics.Frame = frame
	shared := scene.SharedSet()
	complement := scene.ComplementSet()
	eyes := [2]Vec3{rig.EyePos(0), rig.EyePos(1)}

	// Layer 1: shared occluder set.
	survivors := make([]Object, 0, len(scene.Objects))
	for _, obj := range scene.Objects {
		occL := occludedFromEye(eyes[0], obj, shared, &res.Metrics.SharedTraversal)
		occR := occludedFromEye(eyes[1], obj, shared, &res.Metrics.SharedTraversal)
		if occL && occR {
			res.CulledShared[obj.ID] = true
		} else {
			survivors = append(survivors, obj)
		}
	}

	// Layer 2: per-eye complement.
	for eye := 0; eye < 2; eye++ {
		for _, obj := range survivors {
			culled := false
			for _, occ := range complement {
				res.Metrics.ComplementTraversal++
				cov := testOcclusion(eyes[eye], obj, occ, 1.0)
				if cov == coverFull {
					culled = true
					break
				}
				if cov == coverPartial {
					// Single-eye occluder failed to fully cover: expand the
					// query range and retry conservatively.
					covered := false
					for e := 1; e <= maxFallbackExpansions; e++ {
						res.Metrics.ConservativeFallback++
						res.Metrics.ComplementTraversal++
						scale := 1.0 + expansionStep*float64(e)
						if testOcclusion(eyes[eye], obj, occ, scale) == coverFull {
							covered = true
							break
						}
					}
					if covered {
						culled = true
						break
					}
					// Still not covered: conservatively keep the object.
				}
			}
			if culled {
				res.CulledPerEye[eye][obj.ID] = true
			}
		}
	}
	return res
}

// GroundTruthVisible casts dense sample rays to decide whether obj is
// actually visible from eye against ALL occluders in the scene.
func GroundTruthVisible(scene *Scene, eye Vec3, obj Object) bool {
	for _, dir := range silhouetteDirs(eye, obj, 0.9) {
		ray := Ray{Orig: eye, Dir: dir}
		tSphere := raySphere(ray, obj.Center, obj.Radius)
		if tSphere < 0 {
			continue
		}
		blocked := false
		for _, occ := range scene.Occluders {
			tBox := rayBox(ray, occ.Min, occ.Max)
			if tBox > 0 && tBox < tSphere {
				blocked = true
				break
			}
		}
		if !blocked {
			return true
		}
	}
	return false
}

// Validate compares the query result against ground truth and fills in the
// false-cull and shared-failure metrics.
func Validate(scene *Scene, rig *StereoRig, res *QueryResult) {
	eyes := [2]Vec3{rig.EyePos(0), rig.EyePos(1)}
	gt := [2]map[int]bool{{}, {}}
	for eye := 0; eye < 2; eye++ {
		for _, obj := range scene.Objects {
			gt[eye][obj.ID] = GroundTruthVisible(scene, eyes[eye], obj)
		}
	}
	for _, obj := range scene.Objects {
		for eye := 0; eye < 2; eye++ {
			culled := res.CulledShared[obj.ID] || res.CulledPerEye[eye][obj.ID]
			if culled && gt[eye][obj.ID] {
				res.Metrics.FalseCulled++
			}
			if !culled && gt[eye][obj.ID] {
				if eye == 0 {
					res.Metrics.VisibleLeft++
				} else {
					res.Metrics.VisibleRight++
				}
			}
		}
		// Shared layer contract: never cull an object visible to either eye.
		if res.CulledShared[obj.ID] && (gt[0][obj.ID] || gt[1][obj.ID]) {
			res.Metrics.SharedFailure = true
		}
	}
}
