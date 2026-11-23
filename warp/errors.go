package warp

import "errors"

var (
	// ErrNilResource is returned when the single-eye color or depth buffer is missing.
	ErrNilResource = errors.New("warp: single-eye resource is missing (nil color or depth buffer)")
	// ErrBufferSizeMismatch is returned when resource and target resolutions disagree.
	ErrBufferSizeMismatch = errors.New("warp: color/depth buffer size does not match source camera")
	// ErrDepthCorrupt is returned when a depth cell is non-positive or beyond the far plane.
	ErrDepthCorrupt = errors.New("warp: invalid depth sample, single-eye depth resource is corrupt")
	// ErrStaleCamera is returned when the source resource was captured by another camera epoch.
	ErrStaleCamera = errors.New("warp: source camera epoch is stale, refusing to reuse expired eye resource")
	// ErrStaleDepth is returned when the depth buffer version does not match its color buffer.
	ErrStaleDepth = errors.New("warp: depth buffer version is stale, refusing to mix expired depth into reuse")
)
