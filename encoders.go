package mvquant

import "math"

// TruncEncoder 向零截断（fix），量化残差直接丢弃。
type TruncEncoder struct{}

func (TruncEncoder) Name() string { return "trunc" }

func (TruncEncoder) Encode(field [][]Vec2) ([][][2]int32, error) {
	codes := make([][][2]int32, len(field))
	for row := range field {
		codes[row] = make([][2]int32, len(field[row]))
		for col, v := range field[row] {
			if err := checkRange(v); err != nil {
				return nil, err
			}
			codes[row][col] = [2]int32{int32(math.Trunc(v.X)), int32(math.Trunc(v.Y))}
		}
	}
	return codes, nil
}

// RoundEncoder 四舍五入（半数远离零）。
type RoundEncoder struct{}

func (RoundEncoder) Name() string { return "round" }

func (RoundEncoder) Encode(field [][]Vec2) ([][][2]int32, error) {
	codes := make([][][2]int32, len(field))
	for row := range field {
		codes[row] = make([][2]int32, len(field[row]))
		for col, v := range field[row] {
			if err := checkRange(v); err != nil {
				return nil, err
			}
			codes[row][col] = [2]int32{int32(math.Round(v.X)), int32(math.Round(v.Y))}
		}
	}
	return codes, nil
}

// DiffusionEncoder 误差扩散（误差反馈）：每块保留未被表示的残差，
// 下一帧编码同一块时并入该残差，残差幅度恒小于 1 code。
type DiffusionEncoder struct {
	carry [][][2]float64
}

func NewDiffusionEncoder(rows, cols int) *DiffusionEncoder {
	carry := make([][][2]float64, rows)
	for row := range carry {
		carry[row] = make([][2]float64, cols)
	}
	return &DiffusionEncoder{carry: carry}
}

func (*DiffusionEncoder) Name() string { return "diffusion" }

func (e *DiffusionEncoder) Encode(field [][]Vec2) ([][][2]int32, error) {
	if len(e.carry) != len(field) {
		return nil, ErrOutOfRange
	}
	codes := make([][][2]int32, len(field))
	for row := range field {
		if len(e.carry[row]) != len(field[row]) {
			return nil, ErrOutOfRange
		}
		codes[row] = make([][2]int32, len(field[row]))
		for col, v := range field[row] {
			if err := checkRange(v); err != nil {
				return nil, err
			}
			adjX := v.X + e.carry[row][col][0]
			adjY := v.Y + e.carry[row][col][1]
			codeX := int32(math.Trunc(adjX))
			codeY := int32(math.Trunc(adjY))
			if codeX < CodeMin || codeX > CodeMax || codeY < CodeMin || codeY > CodeMax {
				return nil, ErrOutOfRange
			}
			e.carry[row][col] = [2]float64{adjX - float64(codeX), adjY - float64(codeY)}
			codes[row][col] = [2]int32{codeX, codeY}
		}
	}
	return codes, nil
}
