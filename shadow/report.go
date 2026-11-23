package shadow

// Config controls the stress run. All strategies use the same budget.
type Config struct {
	Frames     int     `json:"frames"`
	MapSize    int     `json:"map_size"`
	TileSize   int     `json:"tile_size"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	LightTurns float64 `json:"light_turns"`
}

func DefaultConfig() Config {
	return Config{
		Frames: 36, MapSize: 96, TileSize: 8,
		Width: 128, Height: 80, LightTurns: 1.5,
	}
}

type CasterReport struct {
	Name   string  `json:"name"`
	Kind   string  `json:"kind"`
	Height float64 `json:"height"`
	Eyes   string  `json:"eyes_at_rest"`
}

type FailureProbe struct {
	DisabledEye      string  `json:"disabled_eye"`
	SurvivingEye     string  `json:"surviving_eye"`
	BeforeErrorPct   float64 `json:"before_error_pct"`
	AfterErrorPct    float64 `json:"after_error_pct"`
	ChangedPixels    int     `json:"changed_pixels"`
	EyeSplitTexels   int     `json:"eye_split_texels"`
	ExclusiveCasters int     `json:"exclusive_casters"`
	Preserved        bool    `json:"preserved"`
}

type StrategyReport struct {
	Name              string         `json:"name"`
	Frames            int            `json:"frames"`
	AvgCasters        float64        `json:"avg_caster_set"`
	PeakCasters       int            `json:"peak_caster_set"`
	AvgReusePct       float64        `json:"avg_shadow_map_reuse_pct"`
	PixelErrorPct     float64        `json:"pixel_error_pct"`
	PixelErrorCount   int            `json:"pixel_error_count"`
	ComparedPixels    int            `json:"compared_pixels"`
	PeakShadowBytes   int            `json:"peak_shadow_bytes"`
	PeakResourceBytes int            `json:"peak_resource_bytes"`
	ElapsedMS         float64        `json:"elapsed_ms"`
	Failures          []FailureProbe `json:"eye_failure_probes"`
}

type SceneReport struct {
	Casters        []CasterReport `json:"casters"`
	EyeSeparation  float64        `json:"eye_separation"`
	ExclusiveCount int            `json:"single_eye_casters"`
	CrossEyeCount  int            `json:"cross_eye_casters"`
}

type InvariantReport struct {
	SharedPreservesEyeBits bool `json:"shared_preserves_eye_bits"`
	FailureKeepsShadows    bool `json:"failure_keeps_other_eye_shadows"`
	HybridMatchesReference bool `json:"hybrid_matches_per_eye_texels"`
}

// Report is the stable JSON artifact emitted by the benchmark CLI.
type Report struct {
	Config     Config           `json:"config"`
	Scene      SceneReport      `json:"scene"`
	Strategies []StrategyReport `json:"strategies"`
	Invariants InvariantReport  `json:"invariants"`
}
