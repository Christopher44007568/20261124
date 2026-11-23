package headpose

import (
	"errors"
	"fmt"
	"math"
)

// ScreenParams defines the reversible Camera-to-Screen pixel mapping.
//
// PrincipalX and PrincipalY place the camera optical center in screen pixels.
// PixelsPerMeter is the uniform scale at the declared working plane. The
// mapping is intentionally affine so that every point, including depth, can be
// converted back exactly. It is not a perspective projection.
type ScreenParams struct {
	PixelsPerMeter float64
	PrincipalX     float64
	PrincipalY     float64
}

// CameraToScreenTransform builds the Camera-to-Screen similarity.
func CameraToScreenTransform(params ScreenParams) (Transform, error) {
	if !isFinite(params.PixelsPerMeter) || params.PixelsPerMeter <= 0 {
		return Transform{}, errors.New("pixels per meter must be finite and greater than zero")
	}
	if !isFinite(params.PrincipalX) || !isFinite(params.PrincipalY) {
		return Transform{}, errors.New("principal point must be finite")
	}

	// Camera +Y is up and camera +Z points backward. Screen +Y is down and
	// screen +Z points into the screen, so the basis change is a 180 degree
	// right-handed rotation about +X. Its determinant is +1.
	rotation, err := QuatFromAxisAngle(Vec3{X: 1}, math.Pi)
	if err != nil {
		return Transform{}, err
	}
	translation := Vec3{X: params.PrincipalX, Y: params.PrincipalY}
	return NewSimilarity(rotation, translation, params.PixelsPerMeter)
}

// Scene fixes the device, camera, and screen contracts relative to World.
type Scene struct {
	graph         *FrameGraph
	deviceToWorld Transform
	cameraToWorld Transform
	screen        ScreenParams
}

// NewScene builds the four-frame graph. Device and camera poses are rigid
// transforms from their local frames into World and must use meters.
func NewScene(deviceToWorld, cameraToWorld Transform, screen ScreenParams) (*Scene, error) {
	if err := deviceToWorld.Validate(); err != nil {
		return nil, fmt.Errorf("device-to-world transform: %w", err)
	}
	if err := cameraToWorld.Validate(); err != nil {
		return nil, fmt.Errorf("camera-to-world transform: %w", err)
	}
	if !deviceToWorld.IsRigid(1e-9) {
		return nil, errors.New("device-to-world transform must have unit scale")
	}
	if !cameraToWorld.IsRigid(1e-9) {
		return nil, errors.New("camera-to-world transform must have unit scale")
	}
	cameraToScreen, err := CameraToScreenTransform(screen)
	if err != nil {
		return nil, fmt.Errorf("screen: %w", err)
	}

	graph := NewFrameGraph()
	if err := graph.AddTransform(Device, World, deviceToWorld); err != nil {
		return nil, err
	}
	if err := graph.AddTransform(Camera, World, cameraToWorld); err != nil {
		return nil, err
	}
	if err := graph.AddTransform(Camera, Screen, cameraToScreen); err != nil {
		return nil, err
	}
	return &Scene{
		graph:         graph,
		deviceToWorld: deviceToWorld,
		cameraToWorld: cameraToWorld,
		screen:        screen,
	}, nil
}

// Graph exposes the underlying graph for arbitrary frame conversion.
func (s *Scene) Graph() *FrameGraph {
	return s.graph
}

// Screen returns the screen parameters used by the scene.
func (s *Scene) Screen() ScreenParams {
	return s.screen
}

// DevicePoseInWorld returns the device pose supplied at construction.
func (s *Scene) DevicePoseInWorld() Transform {
	return s.deviceToWorld
}

// CameraPoseInWorld returns the camera pose supplied at construction.
func (s *Scene) CameraPoseInWorld() Transform {
	return s.cameraToWorld
}

// ConvertPoint maps a point between any two scene frames.
func (s *Scene) ConvertPoint(from, to Frame, point Vec3) (Vec3, error) {
	return s.graph.ConvertPoint(from, to, point)
}

// ConvertDirection rotates a direction between any two scene frames.
func (s *Scene) ConvertDirection(from, to Frame, direction Vec3) (Vec3, error) {
	return s.graph.ConvertDirection(from, to, direction)
}

// RoundTripPoint converts a point there and back and reports the error.
func (s *Scene) RoundTripPoint(
	from, to Frame,
	point Vec3,
	tolerance float64,
) (RoundTripResult, error) {
	return s.graph.RoundTripPoint(from, to, point, tolerance)
}

// CheckAllPairs validates every sample against all four frames.
func (s *Scene) CheckAllPairs(
	samples map[Frame]Vec3,
	tolerance float64,
) ([]RoundTripResult, error) {
	return s.graph.CheckAllPairs(samples, tolerance)
}
