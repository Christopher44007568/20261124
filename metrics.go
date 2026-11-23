package mvquant

import "math"

// GhostStats 描述连续帧上量化轨迹相对真实轨迹的漂移。
type GhostStats struct {
	FinalMaxDrift float64
	FinalAvgDrift float64
	PeakDrift     float64
}

// Metrics 汇总一次编码实验的三项指标。
type Metrics struct {
	MaxError       float64
	ZeroVectorRate float64
	Ghost          GhostStats
}

func euclid(dx, dy float64) float64 {
	return math.Hypot(dx, dy)
}

// MaxDisplacementError 返回所有帧、所有块、所有分量误差的最大欧氏范数。
func MaxDisplacementError(truth, decoded [][][]Vec2) float64 {
	maxErr := 0.0
	for frame := range truth {
		for row := range truth[frame] {
			for col, v := range truth[frame][row] {
				d := decoded[frame][row][col]
				err := euclid(d.X-v.X, d.Y-v.Y)
				if err > maxErr {
					maxErr = err
				}
			}
		}
	}
	return maxErr
}

// ZeroVectorRate 返回代码为 (0,0) 的块在全部帧中的占比。
func ZeroVectorRate(codes [][][][2]int32) float64 {
	total, zeros := 0, 0
	for frame := range codes {
		for row := range codes[frame] {
			for _, code := range codes[frame][row] {
				total++
				if code[0] == 0 && code[1] == 0 {
					zeros++
				}
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(zeros) / float64(total)
}

// Ghosting 让每个块分别沿真实场和重建场逐帧平移，
// 比较两条轨迹的位置漂移；残影表现为随帧数累积的漂移。
func Ghosting(truth, decoded [][][]Vec2) GhostStats {
	rows, cols := GridRows, GridCols
	truePos := make([][]Vec2, rows)
	decPos := make([][]Vec2, rows)
	for row := 0; row < rows; row++ {
		truePos[row] = make([]Vec2, cols)
		decPos[row] = make([]Vec2, cols)
		for col := 0; col < cols; col++ {
			truePos[row][col] = Vec2{X: float64(col), Y: float64(row)}
			decPos[row][col] = Vec2{X: float64(col), Y: float64(row)}
		}
	}

	stats := GhostStats{}
	for frame := range truth {
		sumDrift, maxDrift := 0.0, 0.0
		for row := 0; row < rows; row++ {
			for col := 0; col < cols; col++ {
				tv := truth[frame][row][col]
				dv := decoded[frame][row][col]
				truePos[row][col] = Vec2{X: truePos[row][col].X + tv.X, Y: truePos[row][col].Y + tv.Y}
				decPos[row][col] = Vec2{X: decPos[row][col].X + dv.X, Y: decPos[row][col].Y + dv.Y}
				drift := euclid(decPos[row][col].X-truePos[row][col].X,
					decPos[row][col].Y-truePos[row][col].Y)
				sumDrift += drift
				if drift > maxDrift {
					maxDrift = drift
				}
			}
		}
		if maxDrift > stats.PeakDrift {
			stats.PeakDrift = maxDrift
		}
		if frame == len(truth)-1 {
			stats.FinalMaxDrift = maxDrift
			stats.FinalAvgDrift = sumDrift / float64(rows*cols)
		}
	}
	return stats
}

// Evaluate 对一组编码代码计算全部三项指标。
func Evaluate(truth, decoded [][][]Vec2, codes [][][][2]int32) Metrics {
	return Metrics{
		MaxError:       MaxDisplacementError(truth, decoded),
		ZeroVectorRate: ZeroVectorRate(codes),
		Ghost:          Ghosting(truth, decoded),
	}
}
