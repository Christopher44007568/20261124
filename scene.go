package main

// OBB is an oriented bounding box in an instance's local space.
type OBB struct {
	Center Vec3
	Half   Vec3
}

// Instance is a renderable object carrying a local-space OBB.
type Instance struct {
	ID   int
	Name string
	Box  OBB
}

// Node is a scene-graph node; transforms compose down the hierarchy.
type Node struct {
	Name     string
	Local    Mat4
	Instance *Instance
	Children []*Node
}

// BoundInstance pairs an instance with its accumulated world transform.
type BoundInstance struct {
	Inst  *Instance
	World Mat4
}

// Flatten walks the scene graph and computes world transforms.
func Flatten(roots []*Node) []BoundInstance {
	var out []BoundInstance
	var walk func(n *Node, parent Mat4)
	walk = func(n *Node, parent Mat4) {
		world := Mul(parent, n.Local)
		if n.Instance != nil {
			out = append(out, BoundInstance{Inst: n.Instance, World: world})
		}
		for _, c := range n.Children {
			walk(c, world)
		}
	}
	for _, r := range roots {
		walk(r, Identity())
	}
	return out
}

// WorldOBB returns the world-space center and axes (scale included) of b.
func (b BoundInstance) WorldOBB() (center Vec3, axes [3]Vec3, half Vec3) {
	center = b.World.MulPoint(b.Inst.Box.Center)
	axes = [3]Vec3{b.World.Basis(0), b.World.Basis(1), b.World.Basis(2)}
	half = b.Inst.Box.Half
	return center, axes, half
}

// Corners returns the 8 world-space corners of the instance's OBB.
func (b BoundInstance) Corners() [8]Vec3 {
	c := b.Inst.Box.Center
	h := b.Inst.Box.Half
	var pts [8]Vec3
	i := 0
	for _, sx := range [2]float64{-1, 1} {
		for _, sy := range [2]float64{-1, 1} {
			for _, sz := range [2]float64{-1, 1} {
				pts[i] = b.World.MulPoint(Vec3{c.X + sx*h.X, c.Y + sy*h.Y, c.Z + sz*h.Z})
				i++
			}
		}
	}
	return pts
}
