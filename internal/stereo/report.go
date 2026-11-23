package stereo

type Report struct {
	Config        ReportConfig   `json:"config"`
	Runs          []RunReport    `json:"runs"`
	Broken        RunReport      `json:"broken"`
	FailureChecks []FailureCheck `json:"failure_checks"`
}

type ReportConfig struct {
	ImageWidth      int       `json:"image_width"`
	ImageHeight     int       `json:"image_height"`
	FocalPixels     float64   `json:"focal_pixels"`
	PrincipalX      float64   `json:"principal_x"`
	PrincipalY      float64   `json:"principal_y"`
	IPDValuesMM     []float64 `json:"ipd_values_mm"`
	BrokenReference float64   `json:"broken_reference_depth_mm"`
	MatchingWindow  int       `json:"matching_window"`
}

type RunReport struct {
	Label             string            `json:"label"`
	Variant           string            `json:"variant"`
	IPDMM             float64           `json:"ipd_mm"`
	LeftEyeMM         float64           `json:"left_eye_x_mm"`
	RightEyeMM        float64           `json:"right_eye_x_mm"`
	Images            ImageFiles        `json:"images"`
	Match             MatchStats        `json:"match"`
	Probes            []ProbeResult     `json:"probes"`
	Occlusion         OcclusionStats    `json:"occlusion"`
	OutOfBoundsPixels int               `json:"out_of_bounds_pixels"`
	CenterRayErrorMM  float64           `json:"center_ray_error_mm"`
	ProjectionOffsets ProjectionOffsets `json:"projection_offsets"`
}

type ImageFiles struct {
	Left  string `json:"left"`
	Right string `json:"right"`
	Depth string `json:"depth"`
}

type MatchStats struct {
	TotalPixels           int     `json:"total_pixels"`
	ValidPixels           int     `json:"valid_pixels"`
	ValidRatio            float64 `json:"valid_ratio"`
	ComparedPixels        int     `json:"compared_pixels"`
	MedianAbsDepthErrorMM float64 `json:"median_abs_depth_error_mm"`
	MeanAbsDepthErrorMM   float64 `json:"mean_abs_depth_error_mm"`
	RMSEMM                float64 `json:"rmse_mm"`
	P95AbsDepthErrorMM    float64 `json:"p95_abs_depth_error_mm"`
	WithinFivePercent     float64 `json:"within_5_percent"`
	MeanBlockCost         float64 `json:"mean_block_cost"`
}

type ProbeResult struct {
	Name              string  `json:"name"`
	AuthoredPoint     Vec3    `json:"authored_point_mm"`
	AnalyticHitPoint  Vec3    `json:"analytic_hit_point_mm"`
	LeftPixel         Pixel   `json:"left_pixel"`
	ExpectedDisparity float64 `json:"expected_disparity_px"`
	ObservedDisparity float64 `json:"observed_disparity_px"`
	RecoveredPoint    Vec3    `json:"recovered_point_mm"`
	DepthErrorMM      float64 `json:"depth_error_mm"`
	PositionErrorMM   float64 `json:"position_error_mm"`
	Valid             bool    `json:"valid"`
}

type Pixel struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type OcclusionStats struct {
	BackPixelsLeft   int `json:"back_plate_pixels_left"`
	BackPixelsRight  int `json:"back_plate_pixels_right"`
	BackVisibleDelta int `json:"back_plate_visibility_delta"`
}

type ProjectionOffsets struct {
	LeftAtFrontPlatePx  float64 `json:"left_at_front_plate_px"`
	RightAtFrontPlatePx float64 `json:"right_at_front_plate_px"`
	LeftAtWallPx        float64 `json:"left_at_wall_px"`
	RightAtWallPx       float64 `json:"right_at_wall_px"`
}

type FailureCheck struct {
	Name           string `json:"name"`
	CorrectPass    bool   `json:"correct_passes"`
	BrokenDetected bool   `json:"broken_detected"`
	CorrectDetail  string `json:"correct_detail"`
	BrokenDetail   string `json:"broken_detail"`
}
