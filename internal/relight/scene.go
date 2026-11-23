package relight

func BuildScene() Scene {
	receiver := Surface{
		Name:       "receiver",
		Center:     Vec3{0, 0, 4},
		Normal:     Vec3{0, 1, 0},
		Right:      Vec3{1, 0, 0},
		Up:         Vec3{0, 0, 1},
		HalfWidth:  3,
		HalfHeight: 3,
		Albedo:     RGB{0.55, 0.55, 0.55},
	}
	wall := Surface{
		Name:       "near-wall",
		Center:     Vec3{-1.0, 0.35, 2},
		Normal:     Vec3{1, 0, 0},
		Right:      Vec3{0, 0, 1},
		Up:         Vec3{0, 1, 0},
		HalfWidth:  0.35,
		HalfHeight: 0.45,
		Albedo:     RGB{0.35, 0.32, 0.3},
	}
	mirror := Surface{
		Name:       "mirror-patch",
		Center:     Vec3{0.75, 0.06, 4.2},
		Normal:     Vec3{0, 1, 0},
		Right:      Vec3{1, 0, 0},
		Up:         Vec3{0, 0, 1},
		HalfWidth:  0.55,
		HalfHeight: 0.55,
		Albedo:     RGB{0.08, 0.08, 0.09},
		Mirror:     true,
	}
	card := Surface{
		Name:       "reflection-card",
		Center:     Vec3{0.75, 1.25, 4.2},
		Normal:     Vec3{0, -1, 0},
		Right:      Vec3{1, 0, 0},
		Up:         Vec3{0, 0, 1},
		HalfWidth:  1.1,
		HalfHeight: 0.8,
		Albedo:     RGB{0.9, 0.75, 0.45},
	}
	surfaces := []Surface{receiver, wall, mirror, card}
	for i := range surfaces {
		surfaces[i].ID = i
	}
	return Scene{Surfaces: surfaces}
}

type Light struct {
	Position  Vec3
	Intensity RGB
}

func OldLight() Light {
	return Light{Position: Vec3{1.4, 1.8, 1.8}, Intensity: RGB{1.25, 1.25, 1.25}}
}

func CurrentLight() Light {
	return Light{Position: Vec3{2.2, 2.2, 2.8}, Intensity: RGB{0.82, 0.82, 0.82}}
}

func LightCamera(light Light) Camera {
	pose := LookAt(light.Position, Vec3{0, 0, 4}, Vec3{0, 1, 0})
	return NewCamera(pose, 64, 64, 1.1, 0.05, 12)
}

func OldCamera() Camera {
	pose := LookAt(Vec3{-1.08, 0.53, 0.13}, Vec3{-0.08, 0, 4}, Vec3{0, 1, 0})
	return NewCamera(pose, 48, 36, 0.9, 0.05, 12)
}

func CurrentCamera() Camera {
	pose := LookAt(Vec3{-1.0, 0.53, 0.13}, Vec3{-0.1, 0, 4}, Vec3{0, 1, 0})
	return NewCamera(pose, 48, 36, 0.9, 0.05, 12)
}
