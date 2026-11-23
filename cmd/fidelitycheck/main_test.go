package main

import "testing"

func TestRunIsRepeatableAndQuiet(t *testing.T) {
	args := []string{
		"-width", "32",
		"-height", "24",
		"-frames", "3",
		"-radii", "20",
		"-algorithms", "mean",
		"-quiet",
	}
	for i := 0; i < 2; i++ {
		if code := run(args); code != 0 {
			t.Fatalf("run %d returned %d, want 0", i, code)
		}
	}
}

func TestRunRejectsSingleFrame(t *testing.T) {
	code := run([]string{"-frames", "1", "-quiet"})
	if code != 2 {
		t.Fatalf("run returned %d, want 2", code)
	}
}

func TestParseInputs(t *testing.T) {
	values, err := parseFloats("20, 35,50")
	if err != nil || len(values) != 3 || values[1] != 35 {
		t.Fatalf("parseFloats = %v, %v", values, err)
	}
	if _, err := parseFloats("30,nope"); err == nil {
		t.Fatal("parseFloats accepted a non-number")
	}
	algorithms, err := parseAlgorithms("mean,diffusion")
	if err != nil || len(algorithms) != 2 {
		t.Fatalf("parseAlgorithms = %v, %v", algorithms, err)
	}
	if _, err := parseAlgorithms("does-not-exist"); err == nil {
		t.Fatal("parseAlgorithms accepted an unknown reconstructor")
	}
}
