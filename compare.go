package binocular

// Comparison summarizes a shared-resource render against its independently
// reconstructed reference.
type Comparison struct {
	SharedMode                    RenderMode
	ReferenceMode                 RenderMode
	ExactMatch                    bool
	LeftDifference                Difference
	RightDifference               Difference
	BatchesMatch                  bool
	GeometryAccessesMatch         bool
	ClippingMatch                 bool
	RasterizationMatch            bool
	VertexReadsMatch              bool
	IndexReadsMatch               bool
	TransformUploadsMatch         bool
	TransformDigestsMatch         bool
	MaterialUploadsMatch          bool
	SharedGeometryAccesses        int
	SharedGeometryUploads         int
	ReferenceGeometryUploads      int
	GeometryUploadBytesSaved      int64
	SharedTransformUploadBytes    int64
	ReferenceTransformUploadBytes int64
}

// CompareResults compares a shared render with a reconstructed reference.
// ExactMatch requires identical left/right pixels, matching per-eye batch and
// access counts, and matching transform/material uploads. The shared path has
// an additional SharedGeometryAccesses value because the reference path owns a
// separate geometry copy for each eye.
func CompareResults(shared, reference Result) Comparison {
	comparison := Comparison{
		SharedMode:      shared.Mode,
		ReferenceMode:   reference.Mode,
		LeftDifference:  CompareImages(shared.Left, reference.Left),
		RightDifference: CompareImages(shared.Right, reference.Right),
		BatchesMatch: shared.Stats.Left.Batches == reference.Stats.Left.Batches &&
			shared.Stats.Right.Batches == reference.Stats.Right.Batches,
		GeometryAccessesMatch: shared.Stats.Left.GeometryAccesses ==
			reference.Stats.Left.GeometryAccesses &&
			shared.Stats.Right.GeometryAccesses ==
				reference.Stats.Right.GeometryAccesses,
		ClippingMatch: shared.Stats.Left.ClippedTriangles ==
			reference.Stats.Left.ClippedTriangles &&
			shared.Stats.Right.ClippedTriangles ==
				reference.Stats.Right.ClippedTriangles,
		RasterizationMatch: shared.Stats.Left.RasterizedTriangles ==
			reference.Stats.Left.RasterizedTriangles &&
			shared.Stats.Right.RasterizedTriangles ==
				reference.Stats.Right.RasterizedTriangles &&
			shared.Stats.Left.Fragments == reference.Stats.Left.Fragments &&
			shared.Stats.Right.Fragments == reference.Stats.Right.Fragments,
		VertexReadsMatch: shared.Stats.VertexReads == reference.Stats.VertexReads,
		IndexReadsMatch:  shared.Stats.IndexReads == reference.Stats.IndexReads,
		TransformUploadsMatch: shared.Stats.Left.TransformUploadBytes ==
			reference.Stats.Left.TransformUploadBytes &&
			shared.Stats.Right.TransformUploadBytes ==
				reference.Stats.Right.TransformUploadBytes,
		TransformDigestsMatch: shared.Stats.Left.TransformDigest ==
			reference.Stats.Left.TransformDigest &&
			shared.Stats.Right.TransformDigest ==
				reference.Stats.Right.TransformDigest,
		MaterialUploadsMatch: shared.Stats.Left.MaterialUploadBytes ==
			reference.Stats.Left.MaterialUploadBytes &&
			shared.Stats.Right.MaterialUploadBytes ==
				reference.Stats.Right.MaterialUploadBytes,
		SharedGeometryAccesses:   shared.Stats.SharedGeometryAccesses,
		SharedGeometryUploads:    shared.Stats.GeometryUploads,
		ReferenceGeometryUploads: reference.Stats.GeometryUploads,
		GeometryUploadBytesSaved: reference.Stats.GeometryUploadBytes -
			shared.Stats.GeometryUploadBytes,
		SharedTransformUploadBytes:    shared.Stats.TransformUploadBytes,
		ReferenceTransformUploadBytes: reference.Stats.TransformUploadBytes,
	}
	comparison.ExactMatch = comparison.LeftDifference.DifferentPixels == 0 &&
		comparison.RightDifference.DifferentPixels == 0 &&
		comparison.BatchesMatch &&
		comparison.GeometryAccessesMatch &&
		comparison.ClippingMatch &&
		comparison.RasterizationMatch &&
		comparison.VertexReadsMatch &&
		comparison.IndexReadsMatch &&
		comparison.TransformUploadsMatch &&
		comparison.TransformDigestsMatch &&
		comparison.MaterialUploadsMatch
	return comparison
}
