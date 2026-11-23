// Package headpose converts head/device poses among four explicitly defined
// coordinate frames without third-party dependencies.
//
// Conventions:
//   - All frames are right-handed.
//   - Quaternions are scalar-first: (w, x, y, z).
//   - Rotations are active and follow the right-hand rule.
//   - q and -q represent the same rotation; q.Mul(r) applies r first.
//   - World, Device, and Camera use metres. Screen uses pixels.
//
// A Transform maps points as p_target = Rotation * (Scale * p_source) +
// Translation. Scale is uniform and positive, which supports pixel/metre
// handoff while excluding reflections.
//
// Scene connects Device->World, Camera->World, and Camera->Screen. FrameGraph
// composes the shortest path for any pair and supplies exact inverse edges.
// Screen is a reversible affine pixel frame, including depth in pixels, rather
// than a lossy perspective projection.
package headpose
