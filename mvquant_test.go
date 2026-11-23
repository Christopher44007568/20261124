package mvquant

import (
	"errors"
	"math"
	"testing"
)

func TestTruncSemantics(t *testing.T) {
	field := [][]Vec2{{{X: 0.9, Y: -0.9}, {X: 1.7, Y: -1.7}}}
	codes, err := TruncEncoder{}.Encode(field)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := [][2]int32{{0, 0}, {1, -1}}
	for i := range want {
		if codes[0][i] != want[i] {
			t.Fatalf("cell %d: got %v want %v", i, codes[0][i], want[i])
		}
	}
}

func TestRoundSemantics(t *testing.T) {
	field := [][]Vec2{{{X: 0.5, Y: -0.5}, {X: 0.4, Y: -0.6}}}
	codes, err := RoundEncoder{}.Encode(field)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := [][2]int32{{1, -1}, {0, -1}}
	for i := range want {
		if codes[0][i] != want[i] {
			t.Fatalf("cell %d: got %v want %v", i, codes[0][i], want[i])
		}
	}
}

func TestDiffusionAccumulatesResidual(t *testing.T) {
	encoder := NewDiffusionEncoder(1, 1)
	frame := [][]Vec2{{{X: 0.7, Y: -0.7}}}

	var sumX int32
	for frameIndex := 0; frameIndex < 10; frameIndex++ {
		codes, err := encoder.Encode(frame)
		if err != nil {
			t.Fatalf("frame %d: unexpected error: %v", frameIndex, err)
		}
		sumX += codes[0][0][0]
	}
	// 理想累计位移为 7.0，误差反馈把累计偏差限制在 1 code 以内（浮点抖动下取 6 或 7 都合法），
	// 而截断编码在同样输入上的偏差会随帧数线性增长。
	drift := math.Abs(float64(sumX) - 10*0.7)
	if drift > 1+1e-9 {
		t.Fatalf("10 frames of 0.7: cumulative drift %f must be below 1 code", drift)
	}

	carryX := encoder.carry[0][0][0]
	if math.Abs(carryX) >= 1 {
		t.Fatalf("carry magnitude must stay below 1 code, got %f", carryX)
	}
}

func TestStaticFieldIsExact(t *testing.T) {
	truth := BuildSequence(KindStatic, 8)
	for _, encoder := range []Encoder{
		TruncEncoder{},
		RoundEncoder{},
		NewDiffusionEncoder(GridRows, GridCols),
	} {
		codes := make([][][][2]int32, len(truth))
		for frame := range truth {
			encoded, err := encoder.Encode(truth[frame])
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", encoder.Name(), err)
			}
			codes[frame] = encoded
		}
		decoded := make([][][]Vec2, len(truth))
		for frame := range codes {
			decoded[frame] = Decode(codes[frame])
		}
		metrics := Evaluate(truth, decoded, codes)
		if metrics.MaxError != 0 || metrics.ZeroVectorRate != 1 ||
			metrics.Ghost.FinalMaxDrift != 0 {
			t.Fatalf("%s on static field: metrics not perfect: %+v", encoder.Name(), metrics)
		}
	}
}

func TestOutOfRangeRejected(t *testing.T) {
	illegalFields := [][][]Vec2{
		{{{X: CodeMax + 0.1, Y: 0}}},
		{{{X: 0, Y: CodeMin - 0.1}}},
		{{{X: math.NaN(), Y: 0}}},
	}
	encoders := []Encoder{
		TruncEncoder{},
		RoundEncoder{},
		NewDiffusionEncoder(1, 1),
	}
	for _, field := range illegalFields {
		for _, encoder := range encoders {
			codes, err := encoder.Encode(field)
			if !errors.Is(err, ErrOutOfRange) {
				t.Fatalf("%s accepted illegal vector %+v (codes=%v)", encoder.Name(), field, codes)
			}
		}
	}
}
