package strided_test

import (
	"fmt"

	"strided"
)

// A 2x2 bottom-up RGB texture with 2 padding bytes per row and a
// 4-byte header before the first row.
func ExampleDecode() {
	// Layout: [4-byte header][bottom row: 6px bytes + 2 pad][top row + 2 pad]
	buf := []byte{
		0xDE, 0xAD, 0xBE, 0xEF, // header
		0, 0, 255, 0, 255, 0, 0xEE, 0xEE, // bottom row: blue, green
		255, 0, 0, 255, 255, 255, 0xEE, 0xEE, // top row: red, white
	}
	img, err := strided.Decode(buf, strided.Params{
		Width: 2, Height: 2,
		Stride: -8, Offset: 4 + 8, // first (top) row starts after bottom row
		Format: strided.RGB,
	})
	if err != nil {
		panic(err)
	}
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			if x > 0 {
				fmt.Print(" ")
			}
			r, g, b, _ := img.At(x, y).RGBA()
			fmt.Printf("(%3d,%3d,%3d)", r>>8, g>>8, b>>8)
		}
		fmt.Println()
	}
	// Output:
	// (255,  0,  0) (255,255,255)
	// (  0,  0,255) (  0,255,  0)
}
