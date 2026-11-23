package mvquant

import (
	"errors"
	"math"
)

// 10 bit 有符号补码表示的代码范围：[-512, 511]。
const (
	CodeMin = -512
	CodeMax = 511
)

// ErrOutOfRange 表示浮点向量分量超出 10 bit 代码范围，调用方必须拒绝而不是夹紧。
var ErrOutOfRange = errors.New("motion vector component out of 10-bit range")

// Vec2 是像素单位的二维浮点向量。
type Vec2 struct {
	X float64
	Y float64
}

// Encoder 把浮点向量场量化为 10 bit 整数代码场。
// Encode 返回的每个值都在 [CodeMin, CodeMax] 内；越界时返回 ErrOutOfRange。
type Encoder interface {
	Name() string
	Encode(field [][]Vec2) ([][][2]int32, error)
}

// Decode 把整数代码场还原为浮点向量场（1 code = 1 像素）。
func Decode(codes [][][2]int32) [][]Vec2 {
	decoded := make([][]Vec2, len(codes))
	for row := range codes {
		decoded[row] = make([]Vec2, len(codes[row]))
		for col, code := range codes[row] {
			decoded[row][col] = Vec2{X: float64(code[0]), Y: float64(code[1])}
		}
	}
	return decoded
}

func checkRange(v Vec2) error {
	if math.IsNaN(v.X) || math.IsNaN(v.Y) ||
		v.X < CodeMin || v.X > CodeMax ||
		v.Y < CodeMin || v.Y > CodeMax {
		return ErrOutOfRange
	}
	return nil
}
