package main

import "fmt"

// PixelFormat describes one eye texture format.
type PixelFormat struct {
	Name string
	BPP  int
}

var (
	FmtR8      = PixelFormat{"R8", 1}
	FmtRGB8    = PixelFormat{"RGB8", 3}
	FmtRGBA8   = PixelFormat{"RGBA8", 4}
	FmtRGBA16F = PixelFormat{"RGBA16F", 8}
)

// EyeConfig is the desired size/format of one eye.
type EyeConfig struct {
	W, H int
	Fmt  PixelFormat
}

func (c EyeConfig) Bytes() int64 {
	return int64(c.W) * int64(c.H) * int64(c.Fmt.BPP)
}

func (c EyeConfig) String() string {
	return fmt.Sprintf("%dx%d/%s", c.W, c.H, c.Fmt.Name)
}

// Block is one tracked allocation.
type Block struct {
	Buf  []byte
	Size int64
}

// Allocator simulates a graphics memory budget. Allocations above the
// budget fail instead of crashing, so strategies must degrade gracefully.
type Allocator struct {
	Budget   int64 // 0 = unlimited
	Current  int64
	Peak     int64
	Failures int64
}

func (a *Allocator) Alloc(n int64) *Block {
	if a.Budget > 0 && a.Current+n > a.Budget {
		a.Failures++
		return nil
	}
	a.Current += n
	if a.Current > a.Peak {
		a.Peak = a.Current
	}
	return &Block{Buf: make([]byte, n), Size: n}
}

func (a *Allocator) Free(b *Block) {
	if b == nil {
		return
	}
	a.Current -= b.Size
	b.Buf = nil
	b.Size = 0
}

// framePattern derives a deterministic 16-byte pattern per (eye, seq) so the
// harness can verify that presented pixels are bit-identical to what the
// producer rendered ("画面保持不变").
func framePattern(eye, seq int) [16]byte {
	var p [16]byte
	v := uint64(seq+1)*2654435761 + uint64(eye+1)*97
	for i := 0; i < 16; i++ {
		p[i] = byte(v >> (uint(i%8) * 8))
		v = v*6364136223846793005 + 1442695040888963407
	}
	return p
}

// fillFrame writes the pattern tiled across the whole buffer (memset-style).
func fillFrame(buf []byte, eye, seq int) {
	p := framePattern(eye, seq)
	copy(buf, p[:])
	for n := 16; n < len(buf); n *= 2 {
		copy(buf[n:], buf[:n])
	}
}

// checkFrame verifies head and tail samples of the tiled pattern.
func checkFrame(buf []byte, eye, seq int) bool {
	if len(buf) < 16 {
		return false
	}
	p := framePattern(eye, seq)
	for i := 0; i < 16; i++ {
		if buf[i] != p[i] {
			return false
		}
	}
	return buf[len(buf)-1] == p[(len(buf)-1)%16]
}

// Metrics are the per-run numbers we compare across layouts.
type Metrics struct {
	PeakMem       int64
	CopiedBytes   int64
	ExpiredFrames int
	AllocFailures int64
	Stalls        int // healthy eye presented an older frame than its latest submit
	ContentErrors int // presented bytes did not match the expected pattern
}

// Sim is shared context handed to every strategy.
type Sim struct {
	Alloc   *Allocator
	Metrics *Metrics
	Cfg     [2]EyeConfig
}
