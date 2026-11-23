# 20261124

Reversed-Z depth mip reduction (Go standard library only).

- `Buffer.Reduce` / `Buffer.Chain`: min (farthest) and max (nearest) 2x2
  reduction paths; odd widths/heights reduce partial edge blocks without
  dropping texels.
- Strict validation: NaN/Inf depths, empty layers, unknown reduction
  directions and invalid viewport scales are errors, never silent fallbacks.
- `Viewport`: offset+scale rect restricting verification to visible cells.
- `Scene.RenderReference`: analytic ray/sphere and ray/plane intersection
  producing the ground-truth reversed-Z base buffer.
- `VerifyChain`: recomputes each level's bound from the base footprint and
  returns the first bad cell as `*CellError` with level and coordinates.

Run tests with `go test ./...` and the demo with `go run ./cmd/depthmip`.
