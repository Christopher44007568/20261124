package stereocull

import "testing"

func testPoses() []HeadPose {
	return []HeadPose{
		{Rotation: QuatEuler(0, 0, 0)},
		{Rotation: QuatEuler(0, 0.61, 0)},
		{Rotation: QuatEuler(-0.44, 0, 0)},
		{Rotation: QuatEuler(0, 0.35, 0.26)},
		{Position: Vec3{Z: -2}, Rotation: QuatEuler(0, 0, 0)},
		{Position: Vec3{X: 1, Z: 3}, Rotation: QuatEuler(0, -0.45, 0)},
	}
}

func TestPipelinesAgreeForEveryPose(t *testing.T) {
	for _, scene := range DefaultScenes() {
		tree := BuildBVH(scene.Instances, 8)
		for pi, pose := range testPoses() {
			exact := ExactPerEye(tree, scene.Config, pose)
			fast := UnionScreenFast(tree, scene.Config, pose)

			if !exact.Left.Accepted.Equal(fast.Left.Accepted) {
				t.Fatalf("%s pose %d: left sets differ", scene.Kind, pi)
			}
			if !exact.Right.Accepted.Equal(fast.Right.Accepted) {
				t.Fatalf("%s pose %d: right sets differ", scene.Kind, pi)
			}
			if !exact.LeftOnly.Equal(fast.LeftOnly) ||
				!exact.RightOnly.Equal(fast.RightOnly) {
				t.Fatalf("%s pose %d: single-eye sets differ", scene.Kind, pi)
			}
			if !exact.FinalUnion.Equal(fast.FinalUnion) {
				t.Fatalf("%s pose %d: final union differs", scene.Kind, pi)
			}
			if exact.Left.Stats.Accepted != fast.Left.Stats.Accepted ||
				exact.Right.Stats.Accepted != fast.Right.Stats.Accepted {
				t.Fatalf("%s pose %d: accepted counts differ", scene.Kind, pi)
			}
			if fast.Left.Stats.Candidates > fast.UnionScreen.Accepted.Len() ||
				fast.Right.Stats.Candidates > fast.UnionScreen.Accepted.Len() ||
				fast.Left.Stats.Accepted+fast.Left.Stats.Rejected != fast.Left.Stats.Candidates ||
				fast.Right.Stats.Accepted+fast.Right.Stats.Rejected != fast.Right.Stats.Candidates {
				t.Fatalf("%s pose %d: fast per-eye candidate bookkeeping broken", scene.Kind, pi)
			}
			for _, id := range fast.FinalUnion.IDs(nil) {
				if !fast.UnionScreen.Accepted.Has(id) {
					t.Fatalf("%s pose %d: visible id %d missing from screen", scene.Kind, pi, id)
				}
			}
			for _, id := range fast.FalsePositives.IDs(nil) {
				if fast.Left.Accepted.Has(id) || fast.Right.Accepted.Has(id) {
					t.Fatalf("%s pose %d: false positive %d is eye visible", scene.Kind, pi, id)
				}
			}
		}
	}
}

func TestSpecialInstancesBehaveAsDesigned(t *testing.T) {
	for _, scene := range DefaultScenes() {
		tree := BuildBVH(scene.Instances, 8)
		fast := UnionScreenFast(tree, scene.Config, HeadPose{Rotation: QuatEuler(0, 0, 0)})

		byName := map[string]int{}
		for _, ins := range scene.Instances {
			byName[ins.Name] = ins.ID
		}

		leftOnly := byName[NameLeftOnly]
		if !fast.Left.Accepted.Has(leftOnly) || fast.Right.Accepted.Has(leftOnly) {
			t.Fatalf("%s: left-only instance misclassified", scene.Kind)
		}
		rightOnly := byName[NameRightOnly]
		if !fast.Right.Accepted.Has(rightOnly) || fast.Left.Accepted.Has(rightOnly) {
			t.Fatalf("%s: right-only instance misclassified", scene.Kind)
		}
		if ghostID, ok := byName[NameGapGhost]; ok {
			if fast.Left.Accepted.Has(ghostID) || fast.Right.Accepted.Has(ghostID) ||
				!fast.UnionScreen.Accepted.Has(ghostID) {
				t.Fatalf("%s: gap ghost misclassified", scene.Kind)
			}
			if !fast.FalsePositives.Has(ghostID) {
				t.Fatalf("%s: gap ghost not flagged as union false positive", scene.Kind)
			}
		}
	}
}
