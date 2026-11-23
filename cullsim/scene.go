package main

import (
	"math"
	"math/rand"
)

// Object is a cullable scene entity (a small circular prop).
type Object struct {
	Pos    Vec2
	Radius float64
}

// Scene holds the static geometry plus the dynamic light.
type Scene struct {
	Walls   []Seg
	Objects []Object
	Bounds  float64 // half-size of the outer square room
}

// wallWithGap appends a horizontal or vertical wall from a to b with a
// doorway gap of width gapW centered at gapT (0..1 along the wall).
func wallWithGap(walls *[]Seg, a, b Vec2, gapT, gapW float64) {
	if gapW <= 0 {
		*walls = append(*walls, Seg{a, b})
		return
	}
	gc := a.Add(b.Sub(a).Mul(gapT))
	dir := b.Sub(a).Norm()
	g1 := gc.Sub(dir.Mul(gapW / 2))
	g2 := gc.Add(dir.Mul(gapW / 2))
	*walls = append(*walls, Seg{a, g1}, Seg{g2, b})
}

// ringWalls builds a square ring centered at c with half-size r, with one
// door gap per side at the given relative positions.
func ringWalls(walls *[]Seg, c Vec2, r float64, gaps [4]float64, gapW float64) {
	tl := Vec2{c.X - r, c.Y - r}
	tr := Vec2{c.X + r, c.Y - r}
	br := Vec2{c.X + r, c.Y + r}
	bl := Vec2{c.X - r, c.Y + r}
	wallWithGap(walls, tl, tr, gaps[0], gapW) // top
	wallWithGap(walls, tr, br, gaps[1], gapW) // right
	wallWithGap(walls, br, bl, gaps[2], gapW) // bottom
	wallWithGap(walls, bl, tl, gaps[3], gapW) // left
}

// BuildScene creates three nested square room layers with door gaps, a few
// near-wall partitions, and randomly scattered objects (deterministic seed).
func BuildScene() *Scene {
	s := &Scene{Bounds: 300}
	c := Vec2{0, 0}

	// Three room layers: outer boundary + two inner rings.
	ringWalls(&s.Walls, c, 300, [4]float64{0.5, 0.30, 0.62, 0.45}, 46)
	ringWalls(&s.Walls, c, 200, [4]float64{0.35, 0.55, 0.50, 0.70}, 34)
	ringWalls(&s.Walls, c, 110, [4]float64{0.60, 0.40, 0.45, 0.55}, 26)

	// Near-wall partitions: short fins close to where the viewer walks,
	// so small eye offsets produce strong monocular occlusion differences.
	fins := []Seg{
		{Vec2{40, -152}, Vec2{108, -152}},
		{Vec2{-160, 30}, Vec2{-160, 104}},
		{Vec2{148, 60}, Vec2{148, 132}},
		{Vec2{-60, 148}, Vec2{16, 148}},
		{Vec2{232, -40}, Vec2{232, 40}},
		{Vec2{-248, -60}, Vec2{-186, -60}},
	}
	s.Walls = append(s.Walls, fins...)

	// Scatter objects in the walkable annuli, keeping them off the walls.
	rng := rand.New(rand.NewSource(20261124))
	placed := 0
	for placed < 240 {
		ang := rng.Float64() * 2 * math.Pi
		rad := 18 + rng.Float64()*262
		p := Vec2{math.Cos(ang) * rad, math.Sin(ang) * rad}
		if nearWall(s.Walls, p, 7) {
			continue
		}
		s.Objects = append(s.Objects, Object{Pos: p, Radius: 2.2})
		placed++
	}
	return s
}

func nearWall(walls []Seg, p Vec2, d float64) bool {
	for _, w := range walls {
		if distToSeg(p, w) < d {
			return true
		}
	}
	return false
}

func distToSeg(p Vec2, s Seg) float64 {
	ab := s.B.Sub(s.A)
	t := 0.0
	if l2 := ab.Dot(ab); l2 > 1e-12 {
		t = p.Sub(s.A).Dot(ab) / l2
		if t < 0 {
			t = 0
		}
		if t > 1 {
			t = 1
		}
	}
	return p.Dist(s.A.Add(ab.Mul(t)))
}

// LightAt returns the moving light position at normalized time t in [0,1).
// The light hovers around the viewer's walk ring with an oscillating
// angular lead/lag and radial wobble, so it keeps revealing objects near
// the viewer's path as both move.
func LightAt(t float64) Vec2 {
	ang := 2*math.Pi*t + 0.8*math.Sin(2*math.Pi*2*t)
	r := 225 + 30*math.Sin(2*math.Pi*3*t+1.0)
	return Vec2{
		X: r * math.Cos(ang),
		Y: r * math.Sin(ang),
	}
}

// ViewerPose is the head state for one frame.
type ViewerPose struct {
	Pos Vec2
	Yaw float64
}

// Eyes returns the left/right eye positions for this pose.
func (v ViewerPose) Eyes(eyeSep float64) (Vec2, Vec2) {
	perp := FromAngle(v.Yaw).Perp()
	return v.Pos.Sub(perp.Mul(eyeSep / 2)), v.Pos.Add(perp.Mul(eyeSep / 2))
}

// ViewerAt returns the head pose at normalized time t. The head walks a
// rounded loop through the door gaps while yaw oscillates; during two
// "quick turn" windows the yaw rate is deliberately high so that the
// previous frame's binocular union misses newly revealed objects.
func ViewerAt(t float64) ViewerPose {
	ang := 2 * math.Pi * t
	pos := Vec2{
		X: 255 * math.Cos(ang),
		Y: 255 * math.Sin(ang),
	}
	// Base yaw follows the tangent of the walk.
	base := math.Atan2(math.Cos(ang), -math.Sin(ang))
	// Slow scan plus fast head-turn bursts (~25 deg/frame at the peak).
	slow := 0.55 * math.Sin(2*math.Pi*3*t)
	burst := 0.0
	for _, w := range [][2]float64{{0.18, 0.30}, {0.62, 0.74}} {
		if t >= w[0] && t <= w[1] {
			u := (t - w[0]) / (w[1] - w[0])
			burst += 0.5 * math.Sin(2*math.Pi*1.5*u)
		}
	}
	return ViewerPose{Pos: pos, Yaw: base + slow + burst}
}
