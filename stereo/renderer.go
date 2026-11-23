package stereo

const (
	bytesFloat     = 8
	bytesVertex    = 9 * bytesFloat
	bytesTri       = 3 * 4
	bytesCmdRecord = 96
	bytesUBOSlot   = 16*bytesFloat + 16
	bytesViewState = 64
)

type transItem struct {
	tri   [3]cv
	alpha float64
	depth float64
	tag   string
}

func transformNormal(m Mat4, n Vec3) Vec3 {
	return norm(Vec3{
		m[0]*n.X + m[4]*n.Y + m[8]*n.Z,
		m[1]*n.X + m[5]*n.Y + m[9]*n.Z,
		m[2]*n.X + m[6]*n.Y + m[10]*n.Z,
	})
}

// renderEye 是两条路径共用的确定性光栅化核心：
// 只要 VP 矩阵相同，输出必然逐位一致。
func renderEye(sc *Scene, ep EyeParams, c *Counters) *Frame {
	f := newFrame(sc.Width, sc.Height)
	var opaque [][3]cv
	var transparent []transItem

	for instIdx, inst := range sc.Instances {
		m := inst.Mesh
		for ti, t := range m.Tris {
			var clip [3]cv
			viewDepth := 0.0
			for k, vi := range []int{t.I0, t.I1, t.I2} {
				v := m.Verts[vi]
				world := inst.Model.transform(Vec4{v.Pos.X, v.Pos.Y, v.Pos.Z, 1})
				nd := transformNormal(inst.Model, v.Normal)
				col := shade(nd, v.Color, inst.Tint)
				cp := ep.ViewProj.transform(world)
				clip[k] = cv{P: cp, Color: col}
				vs := ep.View.transform(world)
				viewDepth += vs.Z / 3
			}
			tris := clipTri(clip[:])
			c.ClippedTris[ep.Eye] += len(tris)
			tag := m.Name + "#" + itoa(ti) + "@" + itoa(instIdx)
			for ci, ct := range tris {
				if m.Transparent {
					transparent = append(transparent, transItem{ct, m.Alpha, viewDepth, tag + "." + itoa(ci)})
				} else {
					opaque = append(opaque, ct)
				}
			}
		}
	}

	for _, t := range opaque {
		raster(f, t, false, 1, "")
	}
	// 画家算法：远 → 近，排序结果会被记录用于跨路径比对。
	sortTrans(transparent)
	c.TransparentSorts++
	for _, it := range transparent {
		raster(f, it.tri, true, it.alpha, it.tag)
	}
	return f
}

func sortTrans(items []transItem) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j-1].depth > items[j].depth; j-- {
			items[j-1], items[j] = items[j], items[j-1]
		}
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [12]byte
	n := len(b)
	for i > 0 {
		n--
		b[n] = byte('0' + i%10)
		i /= 10
	}
	return string(b[n:])
}

func geometryBytes(sc *Scene) int64 {
	seen := map[*Mesh]bool{}
	var n int64
	for _, in := range sc.Instances {
		if seen[in.Mesh] {
			continue
		}
		seen[in.Mesh] = true
		n += int64(len(in.Mesh.Verts))*bytesVertex + int64(len(in.Mesh.Tris))*bytesTri
	}
	return n
}

func framebufferBytes(sc *Scene) int64 {
	return int64(sc.Width*sc.Height) * 4 * bytesFloat * 2
}

func drawCount(sc *Scene) int { return len(sc.Instances) }

// effectiveVP 解析某只眼实际生效的 VP；故障时右眼沿用上一帧矩阵。
func effectiveVP(sc *Scene, prev [2]Mat4, e Eye, fault FaultMode) Mat4 {
	if fault == FaultSingleEyeMatrix && e == Right {
		return prev[Right]
	}
	return sc.Eyes[e].ViewProj
}
