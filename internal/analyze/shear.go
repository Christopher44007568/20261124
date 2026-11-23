package analyze

import (
	"math"

	"posebench/internal/motion"
	"posebench/internal/scan"
)

type ShearFrame struct {
	DX            float64 `json:"dx_px"`
	DY            float64 `json:"dy_px"`
	ShiftPx       float64 `json:"shift_px"`
	ShearAngleDeg float64 `json:"shear_angle_deg"`
	RMSShiftPx    float64 `json:"rms_landmark_shift_px"`
	MaxShiftPx    float64 `json:"max_landmark_shift_px"`
	AffineBX      float64 `json:"affine_shear_x_per_row"`
	AffineCY      float64 `json:"affine_shear_y_per_row"`
	Landmarks     int     `json:"landmarks"`
	HasAffine     bool    `json:"has_affine"`
}

// MeasureShear compares a true rolling-shutter projection with a global-shutter
// projection at the frame midpoint. Landmark rows are refined because a moving
// point can land on a different sensor row than it did in the reference pose.
func MeasureShear(profile motion.Profile, sh scan.Shutter, frameStart float64) ShearFrame {
	mid := sh.MidTime(frameStart)
	refPose := profile.PoseAt(mid)
	landmarks := scan.FaceLandmarks()
	ref := make([]scan.Point2, len(landmarks))
	rolling := make([]scan.Point2, len(landmarks))
	var shifts []float64
	valid := 0
	for _, landmark := range landmarks {
		refProj := sh.Camera.Project(refPose, landmark.P)
		if !refProj.Valid {
			continue
		}
		row := refProj.Y
		var proj scan.Projection
		for iter := 0; iter < 4; iter++ {
			t := sh.RowTimeAtY(frameStart, row)
			proj = sh.Camera.Project(profile.PoseAt(t), landmark.P)
			if !proj.Valid {
				break
			}
			row = clamp(proj.Y, 0, float64(sh.Camera.Height-1))
		}
		if !proj.Valid {
			continue
		}
		ref[valid] = scan.Point2{X: refProj.X, Y: refProj.Y}
		rolling[valid] = scan.Point2{X: proj.X, Y: proj.Y}
		shifts = append(shifts, math.Hypot(proj.X-refProj.X, proj.Y-refProj.Y))
		valid++
	}

	center := scan.FaceCenter()
	top := sh.Camera.Project(profile.PoseAt(sh.RowTime(frameStart, 0)), center)
	bottom := sh.Camera.Project(profile.PoseAt(sh.RowTime(frameStart, sh.Camera.Height-1)), center)
	out := ShearFrame{
		Landmarks: valid,
	}
	if top.Valid && bottom.Valid {
		out.DX = bottom.X - top.X
		out.DY = bottom.Y - top.Y
		out.ShiftPx = math.Hypot(out.DX, out.DY)
		out.ShearAngleDeg = math.Atan2(out.ShiftPx, float64(sh.Camera.Height-1)) * 180 / math.Pi
	}
	if len(shifts) > 0 {
		var sumSq float64
		for _, v := range shifts {
			sumSq += v * v
			out.MaxShiftPx = maxFloat(out.MaxShiftPx, v)
		}
		out.RMSShiftPx = math.Sqrt(sumSq / float64(len(shifts)))
	}
	if valid >= 3 {
		if affine, ok := scan.FitAffine(ref[:valid], rolling[:valid]); ok {
			out.AffineBX = affine.B
			out.AffineCY = affine.C
			out.HasAffine = true
		}
	}
	return out
}

type ShearSummary struct {
	Samples           int     `json:"samples"`
	MeanShiftPx       float64 `json:"mean_top_bottom_shift_px"`
	MaxShiftPx        float64 `json:"max_top_bottom_shift_px"`
	MeanAbsDX         float64 `json:"mean_abs_dx_px"`
	MeanAbsDY         float64 `json:"mean_abs_dy_px"`
	MeanAngleDeg      float64 `json:"mean_shear_angle_deg"`
	MaxAngleDeg       float64 `json:"max_shear_angle_deg"`
	MeanLandmarkRMS   float64 `json:"mean_landmark_rms_px"`
	MaxLandmarkRMS    float64 `json:"max_landmark_rms_px"`
	MeanAffineShearBX float64 `json:"mean_abs_affine_shear_x_per_row"`
	MeanAffineShearCY float64 `json:"mean_abs_affine_shear_y_per_row"`
	MaxAffineShear    float64 `json:"max_affine_shear_magnitude"`
}

func SummarizeShear(frames []ShearFrame) ShearSummary {
	out := ShearSummary{Samples: len(frames)}
	if len(frames) == 0 {
		return out
	}
	var sumShift, sumDX, sumDY, sumAngle, sumRMS, sumBX, sumCY float64
	var affineCount float64
	for _, f := range frames {
		sumShift += f.ShiftPx
		sumDX += math.Abs(f.DX)
		sumDY += math.Abs(f.DY)
		sumAngle += f.ShearAngleDeg
		sumRMS += f.RMSShiftPx
		out.MaxShiftPx = maxFloat(out.MaxShiftPx, f.ShiftPx)
		out.MaxAngleDeg = maxFloat(out.MaxAngleDeg, f.ShearAngleDeg)
		out.MaxLandmarkRMS = maxFloat(out.MaxLandmarkRMS, f.RMSShiftPx)
		if f.HasAffine {
			sumBX += math.Abs(f.AffineBX)
			sumCY += math.Abs(f.AffineCY)
			affineCount++
			out.MaxAffineShear = maxFloat(out.MaxAffineShear, math.Hypot(f.AffineBX, f.AffineCY))
		}
	}
	n := float64(len(frames))
	out.MeanShiftPx = sumShift / n
	out.MeanAbsDX = sumDX / n
	out.MeanAbsDY = sumDY / n
	out.MeanAngleDeg = sumAngle / n
	out.MeanLandmarkRMS = sumRMS / n
	if affineCount > 0 {
		out.MeanAffineShearBX = sumBX / affineCount
		out.MeanAffineShearCY = sumCY / affineCount
	}
	return out
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
