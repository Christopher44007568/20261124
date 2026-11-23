package render

import (
	"bufio"
	"math"
	"os"
)

// DumpPGM writes the frame as a binary PGM (P5) for offline visual checks.
// Holes are rendered as black. No external image library is involved.
func DumpPGM(f *Frame, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	w := bufio.NewWriter(file)
	if _, err := w.WriteString("P5\n"); err != nil {
		return err
	}
	if _, err := w.WriteString(itoa(f.W) + " " + itoa(f.H) + "\n255\n"); err != nil {
		return err
	}
	buf := make([]byte, f.W*f.H)
	for i, g := range f.Gray {
		switch {
		case math.IsNaN(g):
			buf[i] = 0
		case g < 0:
			buf[i] = 0
		case g > 255:
			buf[i] = 255
		default:
			buf[i] = byte(g)
		}
	}
	if _, err := w.Write(buf); err != nil {
		return err
	}
	return w.Flush()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
