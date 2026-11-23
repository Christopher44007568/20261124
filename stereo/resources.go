package stereo

import (
	"fmt"
	"hash/fnv"
)

// Rect 是帧缓冲像素坐标矩形，Min 包含、Max 不包含。
type Rect struct {
	MinX int
	MinY int
	MaxX int
	MaxY int
}

// Empty 报告矩形是否没有任何像素。
func (r Rect) Empty() bool {
	return r.MaxX <= r.MinX || r.MaxY <= r.MinY
}

// Overlaps 报告两个矩形是否存在像素交集。
func (r Rect) Overlaps(o Rect) bool {
	return r.MinX < o.MaxX && o.MinX < r.MaxX && r.MinY < o.MaxY && o.MinY < r.MaxY
}

// Intersect 返回两个矩形的交集；无交集时返回空矩形。
func (r Rect) Intersect(o Rect) Rect {
	out := Rect{
		MinX: maxInt(r.MinX, o.MinX),
		MinY: maxInt(r.MinY, o.MinY),
		MaxX: minInt(r.MaxX, o.MaxX),
		MaxY: minInt(r.MaxY, o.MaxY),
	}
	if out.Empty() {
		return Rect{}
	}
	return out
}

// Geometry 是一个轴对齐矩形图元；软件管线以扫描线方式逐像素光栅化。
type Geometry struct {
	Name   string
	Bounds Rect
}

// Swatch 是 1x1 纯色纹理样本，名称唯一。
type Swatch struct {
	Name  string
	Color [4]uint8
}

// Registry 保存批次可以引用的全部输入资源。
type Registry struct {
	geometries map[string]Geometry
	swatches   map[string]Swatch
}

// NewRegistry 创建空资源注册表。
func NewRegistry() *Registry {
	return &Registry{
		geometries: make(map[string]Geometry),
		swatches:   make(map[string]Swatch),
	}
}

// AddGeometry 注册一个几何图元，重名会被拒绝。
func (r *Registry) AddGeometry(g Geometry) error {
	if g.Name == "" {
		return fmt.Errorf("stereo: geometry name must not be empty")
	}
	if g.Bounds.Empty() {
		return fmt.Errorf("stereo: geometry %q has empty bounds", g.Name)
	}
	if _, ok := r.geometries[g.Name]; ok {
		return fmt.Errorf("stereo: duplicate geometry %q", g.Name)
	}
	r.geometries[g.Name] = g
	return nil
}

// AddSwatch 注册一个纹理样本，重名会被拒绝。
func (r *Registry) AddSwatch(s Swatch) error {
	if s.Name == "" {
		return fmt.Errorf("stereo: swatch name must not be empty")
	}
	if _, ok := r.swatches[s.Name]; ok {
		return fmt.Errorf("stereo: duplicate swatch %q", s.Name)
	}
	r.swatches[s.Name] = s
	return nil
}

// Geometry 按名称查找几何图元。
func (r *Registry) Geometry(name string) (Geometry, error) {
	g, ok := r.geometries[name]
	if !ok {
		return Geometry{}, fmt.Errorf("stereo: unknown geometry %q", name)
	}
	return g, nil
}

// Swatch 按名称查找纹理样本；空名称返回内置白色纹理。
func (r *Registry) Swatch(name string) (Swatch, error) {
	if name == "" {
		return Swatch{Name: "", Color: [4]uint8{255, 255, 255, 255}}, nil
	}
	s, ok := r.swatches[name]
	if !ok {
		return Swatch{}, fmt.Errorf("stereo: unknown swatch %q", name)
	}
	return s, nil
}

// Digest 对一批输入资源名称做稳定哈希，作为批次的输入资源指纹。
func ResourceDigest(geometryName string, textureName string) uint64 {
	h := fnv.New64a()
	writeString(h, geometryName)
	writeString(h, textureName)
	return h.Sum64()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
