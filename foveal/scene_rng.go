package foveal

import "math"

const pi32 = float32(math.Pi)

func sqrt32(v float32) float32 { return float32(math.Sqrt(float64(v))) }
func log32(v float32) float32  { return float32(math.Log(float64(v))) }
func sin32(v float32) float32  { return float32(math.Sin(float64(v))) }
func cos32(v float32) float32  { return float32(math.Cos(float64(v))) }

// nextUnit returns a deterministic uniform value in (0,1) using splitmix64.
func (s *Scene) nextUnit() float32 {
	s.rngState += 0x9e3779b97f4a7c15
	z := s.rngState
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	z ^= z >> 31
	// 53-bit mantissa keeps the value strictly inside (0,1).
	return float32((z>>11)%(1<<53))/float32(uint64(1)<<53) + 1e-12
}
