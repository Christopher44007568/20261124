package binocular

import "fmt"

// Vertex is one shared geometry vertex. Color is a per-vertex shade that is
// modulated by the material of the eye currently being rendered.
type Vertex struct {
	Position Vec3
	Color    Vec3
}

// Geometry owns shared vertex and index arrays. Configure it before rendering
// and treat it as read-only while a stereo frame is in flight.
type Geometry struct {
	Name     string
	Vertices []Vertex
	Indices  []uint32
}

const (
	uploadedVertexBytes = 6 * 8 // position and color, three float64 each
	uploadedIndexBytes  = 4
)

// TriangleCount returns the number of indexed triangles.
func (g *Geometry) TriangleCount() int {
	if g == nil {
		return 0
	}
	return len(g.Indices) / 3
}

// UploadBytes is the vertex/index payload size used by statistics.
func (g *Geometry) UploadBytes() int64 {
	if g == nil {
		return 0
	}
	return int64(len(g.Vertices)*uploadedVertexBytes + len(g.Indices)*uploadedIndexBytes)
}

// Validate checks index bounds and triangle alignment.
func (g *Geometry) Validate() error {
	if g == nil {
		return fmt.Errorf("binocular: nil geometry")
	}
	if len(g.Indices)%3 != 0 {
		return fmt.Errorf("binocular: geometry %q has %d indices, want a multiple of 3",
			g.Name, len(g.Indices))
	}
	for i, index := range g.Indices {
		if int(index) >= len(g.Vertices) {
			return fmt.Errorf("binocular: geometry %q index %d references vertex %d of %d",
				g.Name, i, index, len(g.Vertices))
		}
	}
	return nil
}

// clone makes a fresh vertex/index payload. It is intentionally used only by
// the reconstructed reference renderer.
func (g *Geometry) clone() *Geometry {
	copied := &Geometry{
		Name:     g.Name,
		Vertices: append([]Vertex(nil), g.Vertices...),
		Indices:  append([]uint32(nil), g.Indices...),
	}
	return copied
}
