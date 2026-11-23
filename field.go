package mvquant

import "math"

// 块网格尺寸与场景参数。
const (
	GridRows = 16
	GridCols = 16

	TranslationVX = 0.62
	TranslationVY = 0.38

	RotationStepRad = 0.018

	ScaleStep = 0.008
)

// FieldKind 标识向量场类型。
type FieldKind string

const (
	KindTranslation FieldKind = "translation"
	KindRotation    FieldKind = "rotation"
	KindScale       FieldKind = "scale"
	KindStatic      FieldKind = "static"
)

func center() (float64, float64) {
	return (GridCols - 1) / 2.0, (GridRows - 1) / 2.0
}

// BuildField 生成第 frame 帧（从 0 开始）的真实浮点向量场。
func BuildField(kind FieldKind, frame int) [][]Vec2 {
	field := make([][]Vec2, GridRows)
	cx, cy := center()
	for row := range field {
		field[row] = make([]Vec2, GridCols)
		for col := range field[row] {
			dx := float64(col) - cx
			dy := float64(row) - cy
			switch kind {
			case KindTranslation:
				field[row][col] = Vec2{X: TranslationVX, Y: TranslationVY}
			case KindRotation:
				theta := RotationStepRad * float64(frame+1)
				sin, cos := math.Sincos(theta)
				field[row][col] = Vec2{
					X: cos*dx - sin*dy - dx,
					Y: sin*dx + cos*dy - dy,
				}
			case KindScale:
				s := ScaleStep * float64(frame+1)
				field[row][col] = Vec2{X: s * dx, Y: s * dy}
			case KindStatic:
				field[row][col] = Vec2{}
			}
		}
	}
	return field
}

// BuildSequence 生成连续 frames 帧的真实向量场。
func BuildSequence(kind FieldKind, frames int) [][][]Vec2 {
	sequence := make([][][]Vec2, frames)
	for frame := range sequence {
		sequence[frame] = BuildField(kind, frame)
	}
	return sequence
}
