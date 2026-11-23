package scene

import (
	"stereowarp/geom"
	"stereowarp/raster"
)

const (
	RoomHalfWidth = 4.0
	RoomHeight    = 3.0
	RoomBackZ     = 8.0
)

func mat(r, g, b, a uint8) raster.Material {
	return raster.Material{Color: raster.RGBA{R: r, G: g, B: b, A: a}}
}

// Build constructs the reusable test scene: a closed room, a close pole that
// produces strong stereo parallax, a second mid-depth pole and a glass sheet.
func Build() []*raster.Mesh {
	meshes := make([]*raster.Mesh, 0, 16)

	meshes = append(meshes,
		raster.Quad("floor",
			geom.V(-RoomHalfWidth, -RoomHeight/2, 0), geom.V(RoomHalfWidth, -RoomHeight/2, 0),
			geom.V(RoomHalfWidth, -RoomHeight/2, RoomBackZ), geom.V(-RoomHalfWidth, -RoomHeight/2, RoomBackZ),
			mat(74, 70, 66, 255)),
		raster.Quad("ceiling",
			geom.V(-RoomHalfWidth, RoomHeight/2, 0), geom.V(-RoomHalfWidth, RoomHeight/2, RoomBackZ),
			geom.V(RoomHalfWidth, RoomHeight/2, RoomBackZ), geom.V(RoomHalfWidth, RoomHeight/2, 0),
			mat(150, 146, 140, 255)),
		raster.Quad("left-wall",
			geom.V(-RoomHalfWidth, -RoomHeight/2, 0), geom.V(-RoomHalfWidth, -RoomHeight/2, RoomBackZ),
			geom.V(-RoomHalfWidth, RoomHeight/2, RoomBackZ), geom.V(-RoomHalfWidth, RoomHeight/2, 0),
			mat(96, 110, 130, 255)),
		raster.Quad("right-wall",
			geom.V(RoomHalfWidth, -RoomHeight/2, 0), geom.V(RoomHalfWidth, RoomHeight/2, 0),
			geom.V(RoomHalfWidth, RoomHeight/2, RoomBackZ), geom.V(RoomHalfWidth, -RoomHeight/2, RoomBackZ),
			mat(120, 104, 96, 255)),
		raster.Quad("back-wall",
			geom.V(-RoomHalfWidth, -RoomHeight/2, RoomBackZ), geom.V(RoomHalfWidth, -RoomHeight/2, RoomBackZ),
			geom.V(RoomHalfWidth, RoomHeight/2, RoomBackZ), geom.V(-RoomHalfWidth, RoomHeight/2, RoomBackZ),
			mat(58, 64, 72, 255)),
	)

	// Colored back-wall panels give the comparison images clear landmarks.
	meshes = append(meshes,
		raster.Quad("panel-red",
			geom.V(-3.2, -0.9, RoomBackZ-0.02), geom.V(-1.6, -0.9, RoomBackZ-0.02),
			geom.V(-1.6, 0.9, RoomBackZ-0.02), geom.V(-3.2, 0.9, RoomBackZ-0.02),
			mat(196, 74, 64, 255)),
		raster.Quad("panel-green",
			geom.V(-0.8, -0.9, RoomBackZ-0.02), geom.V(0.8, -0.9, RoomBackZ-0.02),
			geom.V(0.8, 0.9, RoomBackZ-0.02), geom.V(-0.8, 0.9, RoomBackZ-0.02),
			mat(84, 168, 96, 255)),
		raster.Quad("panel-blue",
			geom.V(1.6, -0.9, RoomBackZ-0.02), geom.V(3.2, -0.9, RoomBackZ-0.02),
			geom.V(3.2, 0.9, RoomBackZ-0.02), geom.V(1.6, 0.9, RoomBackZ-0.02),
			mat(72, 110, 200, 255)),
	)

	// The near pole is only ~1.8 m away, so its two-eye parallax is large.
	meshes = append(meshes,
		raster.Box("near-pole", geom.V(-0.78, -RoomHeight/2, 1.7), geom.V(-0.58, RoomHeight/2, 1.9),
			mat(214, 196, 96, 255)),
		raster.Box("mid-pole", geom.V(0.95, -RoomHeight/2, 3.6), geom.V(1.12, RoomHeight/2, 3.78),
			mat(168, 96, 188, 255)),
		raster.Box("far-block", geom.V(0.2, -RoomHeight/2, 5.6), geom.V(1.0, -0.4, 6.2),
			mat(96, 188, 180, 255)),
	)

	// Transparent sheet: it must be repainted rather than warped, because its
	// color depends on whatever is behind it from the new viewpoint.
	meshes = append(meshes,
		raster.Quad("glass",
			geom.V(-1.45, -0.85, 2.55), geom.V(0.15, -0.85, 2.55),
			geom.V(0.15, 0.85, 2.55), geom.V(-1.45, 0.85, 2.55),
			mat(120, 200, 230, 110)),
	)

	return meshes
}
