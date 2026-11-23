# strided

Decodes pixel buffers whose row stride is not `width * bytesPerPixel`
— rows may carry padding, the stride may be negative (bottom-up
layout), and the first row may start at a non-zero offset. Pure Go
standard library (`image`, `image/color`).

## Usage

```go
img, err := strided.Decode(buf, strided.Params{
    Width:  w, Height: h,
    Stride: stride, // bytes between row starts; may be negative
    Offset: off,    // byte offset of the first row's first pixel
    Format: strided.RGB, // or strided.RGBA, strided.Gray
})
```

Returns `*image.Gray` for `Gray`, `*image.NRGBA` for `RGB` (alpha
forced to 255) and `RGBA`. Padding bytes are never read as pixels.

## Errors

All failures are `*strided.Error` carrying the failing `Row` and byte
`Offset` plus an `ErrKind`:

- `ErrEmptyImage` — width or height ≤ 0
- `ErrInvalidStride` — `|stride|` smaller than one packed row
- `ErrOutOfBounds` — a row escapes the buffer (truncated, bad offset,
  negative stride running below zero)
- `ErrOverflow` — dimension arithmetic would overflow int64

## Tests

`go test ./...` verifies checkerboard, solid-border and cross-row
gradient patterns byte-by-byte across all three formats and four
geometries (tight, padded, offset, negative stride), plus every error
path.
