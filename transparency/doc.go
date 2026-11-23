// Package transparency implements a small, dependency-free CPU renderer for
// comparing temporal reprojection of transparent objects.
//
// The important invariant is that glass, thin filaments, and fog are sampled
// into separate layer images. Each layer is reprojected with its own motion,
// depth, support mask, and history weight before final far-to-near
// compositing. A naive whole-block path is included as a control, alongside a
// no-history path and a supersampled reference.
package transparency
