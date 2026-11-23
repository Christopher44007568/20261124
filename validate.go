package eyebuffer

import (
	"fmt"
	"image"
	"image/color"
	"strings"
)

// IssueKind is stable enough for tests and scripted CI output.
type IssueKind string

const (
	IssueStride      IssueKind = "stride"
	IssueAlignment   IssueKind = "alignment"
	IssueOverlap     IssueKind = "overlap"
	IssueMipBounds   IssueKind = "mip_bounds"
	IssueMipPolicy   IssueKind = "mip_policy"
	IssueGutter      IssueKind = "gutter"
	IssueCrossRegion IssueKind = "cross_region"
	IssueColorBleed  IssueKind = "color_bleed"
	IssueMetadata    IssueKind = "metadata"
	IssueSampleError IssueKind = "sample_error"
)

// Issue names the eye, mip level, and exact location of a failure.
type Issue struct {
	Kind     IssueKind
	Eye      string
	Level    int
	Where    string
	Expected string
	Actual   string
	Detail   string
}

func (i Issue) String() string {
	scope := "atlas"
	if i.Eye != "" {
		scope = i.Eye
		if i.Level >= 0 {
			scope = fmt.Sprintf("%s level %d", i.Eye, i.Level)
		}
	}
	parts := []string{fmt.Sprintf("[%s] %s", i.Kind, scope)}
	if i.Where != "" {
		parts = append(parts, i.Where)
	}
	if i.Expected != "" || i.Actual != "" {
		parts = append(parts, fmt.Sprintf("expected %s, got %s", i.Expected, i.Actual))
	}
	if i.Detail != "" {
		parts = append(parts, i.Detail)
	}
	return strings.Join(parts, ": ")
}

// Report is the complete validation result.
type Report struct {
	Width      int
	Height     int
	Stride     int
	Texels     int
	Samples    int
	Issues     []Issue
	Suppressed int
}

// OK reports whether the atlas passed every check.
func (r Report) OK() bool {
	return len(r.Issues) == 0
}

const maxReportedIssues = 128

func (r *Report) add(issue Issue) {
	if len(r.Issues) >= maxReportedIssues {
		r.Suppressed++
		return
	}
	r.Issues = append(r.Issues, issue)
}

// ValidationOptions controls sampling density and filter behaviour.
type ValidationOptions struct {
	Filter          Filter
	EdgeSamples     int
	InteriorSamples int
	Address         AddressMode
	// Expected overrides the synthetic TexelColor fingerprint. It is useful
	// when CopyLevel has installed caller-provided source pixels.
	Expected TexelFunc
}

// DefaultValidationOptions exercises bilinear edge sampling, where bleed is
// most likely, plus an interior grid.
func DefaultValidationOptions() ValidationOptions {
	return ValidationOptions{
		Filter:          FilterBilinear,
		EdgeSamples:     7,
		InteriorSamples: 3,
		Address:         ClampToLevel,
	}
}

// Validate checks geometry, stride metadata, mip policy, gutter safety, and the
// sampled RGBA values of every eye level.
func Validate(atlas *Atlas, opts ValidationOptions) Report {
	if opts.EdgeSamples < 2 {
		opts.EdgeSamples = 2
	}
	if opts.InteriorSamples < 1 {
		opts.InteriorSamples = 1
	}
	if opts.Filter != FilterNearest && opts.Filter != FilterBilinear {
		opts.Filter = FilterBilinear
	}
	if opts.Address != ClampToLevel && opts.Address != ClampToAtlas {
		opts.Address = ClampToLevel
	}
	if opts.Expected == nil {
		opts.Expected = TexelColor
	}

	if atlas == nil {
		return Report{Issues: []Issue{{Kind: IssueMetadata, Level: -1, Detail: "atlas is nil"}}}
	}
	layout := atlas.Layout
	report := Report{Width: layout.Width, Height: layout.Height, Stride: layout.Stride}
	validateMetadata(atlas, &report)
	validateLayout(layout, opts, &report)
	validateSamples(atlas, opts, &report)
	return report
}

func validateMetadata(atlas *Atlas, report *Report) {
	layout := atlas.Layout
	if layout.Width <= 0 || layout.Height <= 0 {
		report.add(Issue{
			Kind: IssueMetadata, Level: -1,
			Expected: "positive atlas dimensions",
			Actual:   fmt.Sprintf("%dx%d", layout.Width, layout.Height),
		})
		return
	}
	if layout.Alignment <= 0 {
		report.add(Issue{
			Kind: IssueAlignment, Level: -1,
			Where: "layout alignment", Expected: "> 0", Actual: fmt.Sprint(layout.Alignment),
		})
		return
	}
	expectedStride := RowStride(layout.Width, layout.Alignment)
	if layout.Stride != expectedStride {
		report.add(Issue{
			Kind: IssueStride, Level: -1,
			Where:    fmt.Sprintf("atlas width %d with %d-byte alignment", layout.Width, layout.Alignment),
			Expected: fmt.Sprintf("%d bytes/row", expectedStride),
			Actual:   fmt.Sprintf("%d bytes/row", layout.Stride),
			Detail:   "row span is not width*pixelSize; subsequent rows are skewed",
		})
	}
	if layout.Stride%layout.Alignment != 0 {
		report.add(Issue{
			Kind: IssueAlignment, Level: -1,
			Where:    "atlas row stride",
			Expected: fmt.Sprintf("multiple of %d", layout.Alignment),
			Actual:   fmt.Sprintf("%d bytes", layout.Stride),
		})
	}
	need := layout.Stride * layout.Height
	if layout.Stride < 0 || need < 0 || len(atlas.Pix) < need {
		report.add(Issue{
			Kind: IssueMipBounds, Level: -1,
			Where:    "atlas backing store",
			Expected: fmt.Sprintf("at least %d bytes", need),
			Actual:   fmt.Sprintf("%d bytes", len(atlas.Pix)),
		})
	}
}

func validateLayout(layout AtlasLayout, opts ValidationOptions, report *Report) {
	if len(layout.Eyes) == 0 {
		report.add(Issue{Kind: IssueMetadata, Level: -1, Detail: "no eye viewports"})
		return
	}
	radius := 0
	if opts.Filter == FilterBilinear {
		radius = 1
	}
	names := make(map[string]bool, len(layout.Eyes))
	var allLevels []MipLevel
	for eyeIndex := range layout.Eyes {
		eye := layout.Eyes[eyeIndex]
		if eye.Name == "" {
			report.add(Issue{Kind: IssueMetadata, Level: -1, Where: fmt.Sprintf("eye index %d", eyeIndex), Detail: "empty name"})
		}
		if names[eye.Name] {
			report.add(Issue{Kind: IssueMetadata, Eye: eye.Name, Level: -1, Detail: "duplicate eye name"})
		}
		names[eye.Name] = true
		if eye.LogicalWidth <= 0 || eye.LogicalHeight <= 0 || eye.PixelWidth <= 0 || eye.PixelHeight <= 0 {
			report.add(Issue{
				Kind: IssueMetadata, Eye: eye.Name, Level: -1,
				Where:    "viewport dimensions",
				Expected: "positive logical and physical sizes",
				Actual: fmt.Sprintf("logical %dx%d physical %dx%d",
					eye.LogicalWidth, eye.LogicalHeight, eye.PixelWidth, eye.PixelHeight),
			})
			continue
		}
		expectedBaseStride := RowStride(eye.PixelWidth, layout.Alignment)
		if eye.BaseRowStride != expectedBaseStride {
			report.add(Issue{
				Kind: IssueStride, Eye: eye.Name, Level: 0,
				Where:    fmt.Sprintf("base width %d", eye.PixelWidth),
				Expected: fmt.Sprintf("%d bytes/row", expectedBaseStride),
				Actual:   fmt.Sprintf("%d bytes/row", eye.BaseRowStride),
			})
		}
		expectedSizes := MipChain(eye.PixelWidth, eye.PixelHeight, eye.MipmapsEnabled)
		if len(expectedSizes) != len(eye.Levels) {
			report.add(Issue{
				Kind: IssueMipPolicy, Eye: eye.Name, Level: -1,
				Where:    "mip chain",
				Expected: fmt.Sprintf("%d levels (Mipmaps=%t)", len(expectedSizes), eye.MipmapsEnabled),
				Actual:   fmt.Sprintf("%d levels", len(eye.Levels)),
			})
		}
		if !eye.Slot.In(image.Rect(0, 0, layout.Width, layout.Height)) {
			report.add(Issue{
				Kind: IssueMipBounds, Eye: eye.Name, Level: -1,
				Where: "viewport slot", Expected: "inside atlas",
				Actual: eye.Slot.String(),
			})
		}
		for level := range eye.Levels {
			lv := eye.Levels[level]
			if level < len(expectedSizes) {
				want := expectedSizes[level]
				if lv.Width != want.X || lv.Height != want.Y {
					report.add(Issue{
						Kind: IssueMipPolicy, Eye: eye.Name, Level: level,
						Where:    "mip dimensions",
						Expected: fmt.Sprintf("%dx%d", want.X, want.Y),
						Actual:   fmt.Sprintf("%dx%d", lv.Width, lv.Height),
					})
				}
			}
			validateLevel(layout, eye, lv, radius, report)
			allLevels = append(allLevels, lv)
		}
	}
	validateOverlaps(layout, allLevels, radius, report)
}

func validateLevel(layout AtlasLayout, eye EyeLayout, lv MipLevel, radius int, report *Report) {
	if lv.Width <= 0 || lv.Height <= 0 {
		report.add(Issue{
			Kind: IssueMipBounds, Eye: eye.Name, Level: lv.Level,
			Where: "mip dimensions", Expected: "positive size",
			Actual: fmt.Sprintf("%dx%d", lv.Width, lv.Height),
		})
		return
	}
	if lv.Rect.Dx() != lv.Width || lv.Rect.Dy() != lv.Height {
		report.add(Issue{
			Kind: IssueMipBounds, Eye: eye.Name, Level: lv.Level,
			Where: "mip rectangle", Expected: fmt.Sprintf("%dx%d", lv.Width, lv.Height),
			Actual: lv.Rect.String(),
		})
	}
	if !lv.Rect.In(eye.Slot) {
		report.add(Issue{
			Kind: IssueMipBounds, Eye: eye.Name, Level: lv.Level,
			Where: "mip rectangle", Expected: "inside its eye slot " + eye.Slot.String(),
			Actual: lv.Rect.String(),
		})
	}
	if !lv.Rect.In(image.Rect(0, 0, layout.Width, layout.Height)) {
		report.add(Issue{
			Kind: IssueMipBounds, Eye: eye.Name, Level: lv.Level,
			Where: "mip rectangle", Expected: "inside atlas",
			Actual: lv.Rect.String(),
		})
	}
	expectedPacked := RowStride(lv.Width, layout.Alignment)
	if lv.PackedStride != expectedPacked {
		report.add(Issue{
			Kind: IssueStride, Eye: eye.Name, Level: lv.Level,
			Where:    "packed row span",
			Expected: fmt.Sprintf("%d bytes", expectedPacked),
			Actual:   fmt.Sprintf("%d bytes", lv.PackedStride),
		})
	}
	if lv.PackedStride%4 != 0 {
		report.add(Issue{
			Kind: IssueAlignment, Eye: eye.Name, Level: lv.Level,
			Where: "packed row span", Expected: "4-byte aligned",
			Actual: fmt.Sprintf("%d bytes", lv.PackedStride),
		})
	}
	expectedOffset := lv.Rect.Min.Y*layout.Stride + lv.Rect.Min.X*BytesPerPixel
	if lv.ByteOffset != expectedOffset {
		report.add(Issue{
			Kind: IssueStride, Eye: eye.Name, Level: lv.Level,
			Where: "byte offset", Expected: fmt.Sprint(expectedOffset),
			Actual: fmt.Sprint(lv.ByteOffset),
		})
	}
	if lv.ByteOffset%4 != 0 {
		report.add(Issue{
			Kind: IssueAlignment, Eye: eye.Name, Level: lv.Level,
			Where: "byte offset", Expected: "4-byte aligned",
			Actual: fmt.Sprint(lv.ByteOffset),
		})
	}
	if radius > 0 {
		footprint := lv.Rect.Inset(-radius)
		if !footprint.In(eye.Slot) {
			report.add(Issue{
				Kind: IssueGutter, Eye: eye.Name, Level: lv.Level,
				Where:    "bilinear edge footprint " + footprint.String(),
				Expected: "within eye slot " + eye.Slot.String(),
				Actual:   fmt.Sprintf("slot gutter is %d px", layout.Gutter),
				Detail:   "edge filtering can reach gutter/row padding; increase gutter",
			})
		}
	}
}

func validateOverlaps(layout AtlasLayout, levels []MipLevel, radius int, report *Report) {
	for i := 0; i < len(levels); i++ {
		for j := i + 1; j < len(levels); j++ {
			a, b := levels[i], levels[j]
			if a.Rect.Intersect(b.Rect).Empty() {
				continue
			}
			report.add(Issue{
				Kind: IssueOverlap, Eye: a.Eye, Level: a.Level,
				Where:    fmt.Sprintf("%s overlaps %s level %d", a.Rect.String(), b.Eye, b.Level),
				Expected: "disjoint mip rectangles", Actual: b.Rect.String(),
			})
		}
	}
	if radius == 0 {
		return
	}
	for i := 0; i < len(levels); i++ {
		footprint := levels[i].Rect.Inset(-radius)
		for j := 0; j < len(levels); j++ {
			if i == j {
				continue
			}
			if !footprint.Intersect(levels[j].Rect).Empty() {
				report.add(Issue{
					Kind: IssueGutter, Eye: levels[i].Eye, Level: levels[i].Level,
					Where:    fmt.Sprintf("filter footprint %s reaches %s level %d", footprint.String(), levels[j].Eye, levels[j].Level),
					Expected: "at least 1 px gutter", Actual: fmt.Sprintf("%d px", layout.Gutter),
					Detail: "adjacent viewport/mip is close enough for bilinear bleed",
				})
			}
		}
	}
}

func validateSamples(atlas *Atlas, opts ValidationOptions, report *Report) {
	layout := atlas.Layout
	type sampleKey struct {
		eye   int
		level int
		kind  IssueKind
	}
	counts := make(map[sampleKey]int)
	const perLocationLimit = 2
	addSampleIssue := func(eyeIndex, level int, issue Issue) {
		key := sampleKey{eye: eyeIndex, level: level, kind: issue.Kind}
		if counts[key] >= perLocationLimit {
			report.Suppressed++
			return
		}
		counts[key]++
		report.add(issue)
	}
	for eyeIndex, eye := range layout.Eyes {
		for level, lv := range eye.Levels {
			for y := 0; y < lv.Height; y++ {
				for x := 0; x < lv.Width; x++ {
					report.Texels++
					atlasPoint := image.Pt(lv.Rect.Min.X+x, lv.Rect.Min.Y+y)
					got, ok := atlas.Pixel(atlasPoint.X, atlasPoint.Y)
					expected := opts.Expected(eyeIndex, level, x, y, lv.Width, lv.Height)
					if ok && sameRGBA(got, expected) {
						continue
					}
					kind := IssueColorBleed
					detail := "texel fingerprint mismatch; possible stride, copy, or bleed error"
					if !ok {
						kind = IssueMipBounds
						detail = "texel is outside the backing store"
					} else if got == GutterColor {
						detail = "gutter sentinel colour found inside a viewport"
					} else if texelMatchesOtherEye(atlas, lv, x, y, got, opts.Expected) {
						kind = IssueCrossRegion
						detail = "texel colour belongs to a different eye viewport or mip"
					}
					addSampleIssue(eyeIndex, level, Issue{
						Kind: kind, Eye: eye.Name, Level: level,
						Where:    fmt.Sprintf("local texel (%d,%d) at atlas %s", x, y, atlasPoint),
						Expected: formatRGBA(expected), Actual: formatRGBA(got),
						Detail: detail,
					})
				}
			}
			for _, uv := range validationUVs(opts.EdgeSamples, opts.InteriorSamples) {
				sample, err := atlas.SampleWithAddress(eyeIndex, level, uv.x, uv.y, opts.Filter, opts.Address)
				report.Samples++
				if err != nil {
					report.add(Issue{
						Kind: IssueSampleError, Eye: eye.Name, Level: level,
						Where:  fmt.Sprintf("UV (%.6f,%.6f)", uv.x, uv.y),
						Detail: err.Error(),
					})
					continue
				}
				outside := sampleOutsideLevel(atlas, lv, sample)
				if len(outside) > 0 {
					chosen := outside[0]
					for _, tap := range outside {
						owner, found := levelAt(layout, tap.Atlas)
						if found && owner.EyeIndex != lv.EyeIndex {
							chosen = tap
							break
						}
					}
					addSampleIssue(eyeIndex, level, sampleTapIssue(atlas, lv, sample, chosen, uv))
					continue
				}
				expected := expectedSample(opts.Expected, eyeIndex, level,
					lv.Width, lv.Height, uv.x, uv.y, opts.Filter)
				if sameRGBA(sample.Color, expected) {
					continue
				}
				kind := IssueColorBleed
				detail := fmt.Sprintf("edge/interior texel mismatch at atlas %s", tapSummary(sample))
				if sample.Color == GutterColor {
					detail = "gutter sentinel colour leaked into the sample"
				} else if matchesAnotherEye(atlas, lv, sample, sample.Color, opts.Expected) {
					kind = IssueCrossRegion
					detail = "sampled colour belongs to a different eye viewport"
				}
				addSampleIssue(eyeIndex, level, Issue{
					Kind: kind, Eye: eye.Name, Level: level,
					Where:    fmt.Sprintf("UV (%.6f,%.6f) filter=%s", uv.x, uv.y, opts.Filter),
					Expected: formatRGBA(expected), Actual: formatRGBA(sample.Color),
					Detail: detail,
				})
			}
		}
	}
}

type uvPoint struct {
	x float64
	y float64
}

func validationUVs(edgeSamples, interiorSamples int) []uvPoint {
	points := make([]uvPoint, 0, edgeSamples*4+interiorSamples*interiorSamples)
	add := func(x, y float64) {
		for _, point := range points {
			if point.x == x && point.y == y {
				return
			}
		}
		points = append(points, uvPoint{x: x, y: y})
	}
	denom := float64(max(1, edgeSamples-1))
	for i := 0; i < edgeSamples; i++ {
		t := float64(i) / denom
		add(t, 0)
		add(t, 1)
		add(0, t)
		add(1, t)
	}
	for y := 0; y < interiorSamples; y++ {
		for x := 0; x < interiorSamples; x++ {
			add((float64(x)+0.5)/float64(interiorSamples), (float64(y)+0.5)/float64(interiorSamples))
		}
	}
	return points
}

func sampleOutsideLevel(atlas *Atlas, lv MipLevel, sample Sample) []Tap {
	var outside []Tap
	for _, tap := range sample.Taps {
		if tap.Local.X < 0 || tap.Local.Y < 0 || tap.Local.X >= lv.Width || tap.Local.Y >= lv.Height ||
			!tap.Atlas.In(lv.Rect) {
			outside = append(outside, tap)
		}
	}
	return outside
}

func sampleTapIssue(atlas *Atlas, current MipLevel, sample Sample, tap Tap, uv uvPoint) Issue {
	owner, found := levelAt(atlas.Layout, tap.Atlas)
	kind := IssueGutter
	detail := "tap is outside the selected mip level"
	if found && (owner.EyeIndex != current.EyeIndex || owner.Level != current.Level) {
		kind = IssueCrossRegion
		detail = fmt.Sprintf("tap crossed into %s level %d", owner.Eye, owner.Level)
	} else if !tap.Atlas.In(image.Rect(0, 0, atlas.Layout.Width, atlas.Layout.Height)) {
		kind = IssueMipBounds
		detail = "tap is outside the atlas"
	}
	return Issue{
		Kind: kind, Eye: current.Eye, Level: current.Level,
		Where: fmt.Sprintf("UV (%.6f,%.6f) %s filtered tap raw=(%d,%d) atlas=(%d,%d)",
			uv.x, uv.y, sample.Filter, tap.Raw.X, tap.Raw.Y, tap.Atlas.X, tap.Atlas.Y),
		Expected: fmt.Sprintf("tap inside %s", current.Rect.String()),
		Actual:   fmt.Sprintf("tap outside at %s", image.Pt(tap.Atlas.X, tap.Atlas.Y)),
		Detail:   detail,
	}
}

func levelAt(layout AtlasLayout, p image.Point) (MipLevel, bool) {
	for _, eye := range layout.Eyes {
		for _, lv := range eye.Levels {
			if p.In(lv.Rect) {
				return lv, true
			}
		}
	}
	return MipLevel{}, false
}

func matchesAnotherEye(atlas *Atlas, current MipLevel, sample Sample, got color.RGBA, texel TexelFunc) bool {
	for _, tap := range sample.Taps {
		if texelMatchesOtherEye(atlas, current, tap.Local.X, tap.Local.Y, got, texel) {
			return true
		}
	}
	return false
}

func texelMatchesOtherEye(atlas *Atlas, current MipLevel, x, y int, got color.RGBA, texel TexelFunc) bool {
	for _, eye := range atlas.Layout.Eyes {
		if eye.Index == current.EyeIndex {
			continue
		}
		for level, lv := range eye.Levels {
			if x < 0 || y < 0 || x >= lv.Width || y >= lv.Height {
				continue
			}
			if texel(eye.Index, level, x, y, lv.Width, lv.Height) == got {
				return true
			}
		}
	}
	return false
}

func tapSummary(sample Sample) string {
	if len(sample.Taps) == 0 {
		return "no taps"
	}
	parts := make([]string, 0, min(4, len(sample.Taps)))
	for _, tap := range sample.Taps {
		parts = append(parts, fmt.Sprintf("(%d,%d)", tap.Atlas.X, tap.Atlas.Y))
	}
	return strings.Join(parts, ",")
}

func sameRGBA(a, b color.RGBA) bool {
	return a.R == b.R && a.G == b.G && a.B == b.B && a.A == b.A
}

func formatRGBA(c color.RGBA) string {
	return fmt.Sprintf("#%02X%02X%02X%02X", c.R, c.G, c.B, c.A)
}
