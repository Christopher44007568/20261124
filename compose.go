package reprojection

// FrameBuffer is the final scanout raster.
type FrameBuffer struct {
	Width  int
	Height int
	Pixels []Color
}

func NewFrameBuffer(width, height int) *FrameBuffer {
	return &FrameBuffer{Width: width, Height: height, Pixels: make([]Color, width*height)}
}

func (f *FrameBuffer) At(x, y int) *Color {
	if x < 0 || y < 0 || x >= f.Width || y >= f.Height {
		return nil
	}
	return &f.Pixels[y*f.Width+x]
}

// CompositeOverlay copies the world raster and then source-over blends the UI.
// When testDepth is true, the UI's declared depth is compared with the world
// depth buffer. That mode is intentionally not the system-UI path.
func CompositeOverlay(world *WorldImage, ui *UIImage, testDepth bool) (*FrameBuffer, []bool, []bool) {
	out := NewFrameBuffer(world.Width, world.Height)
	for i := range world.Pixels {
		out.Pixels[i] = world.Pixels[i].Color
	}

	mask := make([]bool, len(out.Pixels))
	rejected := make([]bool, len(out.Pixels))
	for i := range ui.Pixels {
		u := &ui.Pixels[i]
		if !u.Filled || u.Color.A == 0 {
			continue
		}
		w := &world.Pixels[i]
		if testDepth && w.Panel >= 0 && w.Depth < u.Depth {
			rejected[i] = true
			continue
		}
		out.Pixels[i] = u.Color.Over(w.Color)
		mask[i] = true
	}
	return out, mask, rejected
}

func countFilled(ui *UIImage) int {
	n := 0
	for i := range ui.Pixels {
		if ui.Pixels[i].Filled && ui.Pixels[i].Color.A > 0 {
			n++
		}
	}
	return n
}

func countTrue(v []bool) int {
	n := 0
	for _, b := range v {
		if b {
			n++
		}
	}
	return n
}

func countFrameDiff(a, b *FrameBuffer) int {
	n := 0
	for i := range a.Pixels {
		if a.Pixels[i] != b.Pixels[i] {
			n++
		}
	}
	return n
}
