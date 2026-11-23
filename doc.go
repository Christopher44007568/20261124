// Package posetimeline validates per-eye pose sample streams and selects
// poses for re-projection. It is pure Go with no graphics SDK dependency.
//
// Design rules:
//
//   - Recency comes from translated timestamps, never from array position.
//     A batch is translated onto the primary clock domain and processed in
//     timestamp order, so the last array element is never assumed newest.
//   - Every sample yields a Decision recording acceptance state, reference
//     time, translated (global) time and a bitmask of rejection reasons.
//   - Rejected classes: duplicate timestamp, backwards/non-monotonic clock,
//     expired pose, future pose, left/right time mismatch and more than one
//     pose per frame id.
//   - A backwards clock invalidates the eye's timeline. While broken, samples
//     are refused and motion accumulation stops; Reset re-anchors from the
//     next accepted sample without carrying stale motion.
//   - Samples from unregistered clock domains are rejected rather than
//     compared across domains. Register affine offsets on ClockRegistry to
//     translate secondary domains onto the primary one.
//   - SelectForReprojection returns the latest accepted pose at or before the
//     target time (optionally within a max-age window); it never returns
//     future or rejected poses and never extrapolates.
package posetimeline
