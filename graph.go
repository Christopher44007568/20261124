package headpose

import (
	"errors"
	"fmt"
	"math"
)

var (
	// ErrNoPath means the requested frames are not connected.
	ErrNoPath = errors.New("no transform path between frames")
)

// FrameGraph stores directed coordinate transforms. Adding an edge also adds
// its exact inverse.
type FrameGraph struct {
	edges map[Frame]map[Frame]Transform
}

// NewFrameGraph returns an empty frame graph.
func NewFrameGraph() *FrameGraph {
	return &FrameGraph{edges: make(map[Frame]map[Frame]Transform)}
}

// AddTransform connects from and to in both directions. A second edge between
// the same pair replaces the first one.
func (g *FrameGraph) AddTransform(from, to Frame, transform Transform) error {
	if !from.Valid() || !to.Valid() {
		return fmt.Errorf("unknown frame pair %q -> %q", from, to)
	}
	if from == to {
		return errors.New("from and to frames must differ")
	}
	if err := transform.Validate(); err != nil {
		return fmt.Errorf("transform %q -> %q: %w", from, to, err)
	}
	if g.edges == nil {
		g.edges = make(map[Frame]map[Frame]Transform)
	}
	if g.edges[from] == nil {
		g.edges[from] = make(map[Frame]Transform)
	}
	if g.edges[to] == nil {
		g.edges[to] = make(map[Frame]Transform)
	}
	g.edges[from][to] = transform
	g.edges[to][from] = transform.Inverse()
	return nil
}

// Path returns the shortest frame path from to, including both endpoints.
func (g *FrameGraph) Path(from, to Frame) ([]Frame, error) {
	if !from.Valid() || !to.Valid() {
		return nil, fmt.Errorf("unknown frame pair %q -> %q", from, to)
	}
	if from == to {
		return []Frame{from}, nil
	}

	queue := []Frame{from}
	previous := map[Frame]Frame{from: from}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range FrameOrder {
			if _, seen := previous[next]; seen {
				continue
			}
			if _, connected := g.edges[current][next]; !connected {
				continue
			}
			previous[next] = current
			if next == to {
				return buildPath(previous, from, to), nil
			}
			queue = append(queue, next)
		}
	}
	return nil, fmt.Errorf("%w: %s -> %s", ErrNoPath, from, to)
}

// Transform returns the composed transform from one frame to another.
func (g *FrameGraph) Transform(from, to Frame) (Transform, error) {
	path, err := g.Path(from, to)
	if err != nil {
		return Transform{}, err
	}
	result := IdentityTransform()
	for i := 0; i+1 < len(path); i++ {
		edge, ok := g.edges[path[i]][path[i+1]]
		if !ok {
			return Transform{}, fmt.Errorf("%w: %s -> %s", ErrNoPath, from, to)
		}
		result = result.Compose(edge)
	}
	return result, nil
}

// ConvertPoint maps point from one frame to another using the shortest path.
func (g *FrameGraph) ConvertPoint(from, to Frame, point Vec3) (Vec3, error) {
	transform, err := g.Transform(from, to)
	if err != nil {
		return Vec3{}, err
	}
	return transform.ApplyPoint(point), nil
}

// ConvertDirection rotates a direction from one frame to another. Translation
// and unit scale are not applied.
func (g *FrameGraph) ConvertDirection(from, to Frame, direction Vec3) (Vec3, error) {
	transform, err := g.Transform(from, to)
	if err != nil {
		return Vec3{}, err
	}
	return transform.ApplyDirection(direction), nil
}

// RoundTripResult records one source-to-target-to-source check.
type RoundTripResult struct {
	From        Frame
	To          Frame
	Forward     Vec3
	Backward    Vec3
	MaxError    float64
	Tolerance   float64
	Pass        bool
	ForwardPath []Frame
}

// RoundTripPoint converts point forward and back, then compares both results.
func (g *FrameGraph) RoundTripPoint(
	from, to Frame,
	point Vec3,
	tolerance float64,
) (RoundTripResult, error) {
	if tolerance < 0 || !isFinite(tolerance) {
		return RoundTripResult{}, errors.New("tolerance must be finite and non-negative")
	}
	forward, err := g.ConvertPoint(from, to, point)
	if err != nil {
		return RoundTripResult{}, err
	}
	backward, err := g.ConvertPoint(to, from, forward)
	if err != nil {
		return RoundTripResult{}, err
	}
	path, err := g.Path(from, to)
	if err != nil {
		return RoundTripResult{}, err
	}
	maxError := point.MaxComponentDiff(backward)
	return RoundTripResult{
		From:        from,
		To:          to,
		Forward:     forward,
		Backward:    backward,
		MaxError:    maxError,
		Tolerance:   tolerance,
		Pass:        maxError <= tolerance,
		ForwardPath: path,
	}, nil
}

// CheckAllPairs runs every sample through every frame and back.
func (g *FrameGraph) CheckAllPairs(
	samples map[Frame]Vec3,
	tolerance float64,
) ([]RoundTripResult, error) {
	results := make([]RoundTripResult, 0, len(samples)*len(FrameOrder))
	for _, from := range FrameOrder {
		point, ok := samples[from]
		if !ok {
			return nil, fmt.Errorf("missing sample point for frame %q", from)
		}
		if !point.IsFinite() {
			return nil, fmt.Errorf("sample point for frame %q must be finite", from)
		}
		for _, to := range FrameOrder {
			result, err := g.RoundTripPoint(from, to, point, tolerance)
			if err != nil {
				return nil, err
			}
			results = append(results, result)
		}
	}
	return results, nil
}

// MaxRoundTripError returns the largest error in a result set.
func MaxRoundTripError(results []RoundTripResult) float64 {
	var maximum float64
	for _, result := range results {
		maximum = math.Max(maximum, result.MaxError)
	}
	return maximum
}

func buildPath(previous map[Frame]Frame, from, to Frame) []Frame {
	reversed := []Frame{to}
	for current := to; current != from; {
		current = previous[current]
		reversed = append(reversed, current)
	}
	path := make([]Frame, len(reversed))
	for i := range reversed {
		path[len(reversed)-1-i] = reversed[i]
	}
	return path
}
