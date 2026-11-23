// Package lensdistortion implements standard-library-only lens distortion
// mapping for monoscopic and stereo displays.
//
// All mappings use the corrected-display-to-raw-source direction for
// rendering: Mapper.SourceFromDisplay returns the source coordinate that a
// corrected output pixel must sample. Mapper.DisplayFromSource is the inverse
// direction used for verification. Keeping this direction explicit avoids
// applying a reprojection in the wrong order.
//
// Three evaluation strategies are provided: direct analytic evaluation with a
// bracketed inverse solver, a radial polynomial transfer, and a piecewise
// lookup table. Mapping validity is bounded; none of these extrapolate beyond
// the calibrated domain. StereoRig retains a separate mapper and distortion
// center for each eye.
//
// AnalyzeCoverage checks non-rectangular display masks and reports model-domain
// failures, source-boundary failures, bilinear footprint holes, and clamped
// edge samples.
package lensdistortion
