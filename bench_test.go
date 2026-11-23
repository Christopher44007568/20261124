package stereocull

import "testing"

func BenchmarkExactVsFast(b *testing.B) {
	scenes := DefaultScenes()
	pose := HeadPose{Rotation: QuatEuler(0, 0.3, 0.1)}
	for _, leaf := range []int{4, 8, 16} {
		for _, sc := range scenes {
			tree := BuildBVH(sc.Instances, leaf)
			b.Run(fmtLabel("exact", leaf, sc.Kind), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					ExactPerEye(tree, sc.Config, pose)
				}
			})
			b.Run(fmtLabel("fast", leaf, sc.Kind), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					UnionScreenFast(tree, sc.Config, pose)
				}
			})
		}
	}
}

func fmtLabel(pipe string, leaf int, kind SceneKind) string {
	return pipe + "/leaf" + itoa(leaf) + "/" + string(kind)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
