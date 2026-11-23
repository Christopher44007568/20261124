# 20261124

Pure-Go (stdlib only) simulation of panel/lens/render center
miscalibration: crosshair, boundary and known-depth markers are
used to apply translation, rotation and per-eye biases, and to
measure reprojection error and binocular disparity.

- `go test ./...` — runs the calibration fault test suite
- `go run ./cmd/report` — prints the per-scenario error table
