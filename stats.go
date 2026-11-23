package binocular

// RenderMode identifies how vertex/index data was supplied to the renderer.
type RenderMode string

const (
	// ModeShared renders both eyes from the same Geometry objects.
	ModeShared RenderMode = "shared"
	// ModeReconstructed rebuilds Geometry once per eye before rendering.
	ModeReconstructed RenderMode = "reconstructed"
)

// EyeStats records work performed for one eye.
type EyeStats struct {
	Eye                  Eye
	Batches              int
	Triangles            int
	ClippedTriangles     int
	RasterizedTriangles  int
	Fragments            int64
	GeometryAccesses     int
	VertexReads          int64
	IndexReads           int64
	TransformUploads     int
	TransformUploadBytes int64
	TransformDigest      uint64
	MaterialUploads      int
	MaterialUploadBytes  int64
}

// StereoStats records both eyes plus frame-wide resource accounting.
type StereoStats struct {
	Mode                        RenderMode
	Left                        EyeStats
	Right                       EyeStats
	Batches                     int
	SharedGeometryAccesses      int
	VertexReads                 int64
	IndexReads                  int64
	GeometryUploads             int
	GeometryUploadBytes         int64
	TransformUploads            int
	TransformUploadBytes        int64
	MaterialUploads             int
	MaterialUploadBytes         int64
	SharedVertexCount           int
	SharedIndexCount            int
	ReconstructedGeometryCopies int
}

func newEyeStats(eye Eye) EyeStats {
	return EyeStats{
		Eye:             eye,
		TransformDigest: 14695981039346656037,
	}
}

func (stats *EyeStats) noteTransform(uploaded ...Mat4) {
	const (
		prime64 = uint64(1099511628211)
	)
	stats.TransformUploads += len(uploaded)
	stats.TransformUploadBytes += int64(len(uploaded) * 16 * 8)
	for _, matrix := range uploaded {
		fingerprint := matrix.Fingerprint()
		for shift := 0; shift < 64; shift += 8 {
			stats.TransformDigest ^= (fingerprint >> shift) & 0xff
			stats.TransformDigest *= prime64
		}
	}
}

func (stats *EyeStats) noteMaterial(material Material) {
	stats.MaterialUploads++
	stats.MaterialUploadBytes += uploadedMaterialBytes
}

// Result is a rendered stereo frame and its resource accounting.
type Result struct {
	Mode  RenderMode
	Left  Image
	Right Image
	Stats StereoStats
}
