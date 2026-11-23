# Compositor occlusion comparison

Scene `hand-panel-notification`: 12 x 8, evaluated independently for left and right eyes.

The safe policy uses nearest valid depth for the current eye. If any covering layer has missing/invalid depth, it falls back to the explicit back-to-front order and marks the pixel ambiguous. A system flag never changes ordering.

## Scene layers

| layer | kind | system | fallback order | coverage | left depth | right depth |
|---|---|:---:|---:|---:|---:|---:|
| notice | notification | true | 0 | 96 | 7.0000-7.0290 m | 6.9900-7.0201 m |
| panel | panel | false | 1 | 48 | 1.2070-1.2330 m | 1.2172-1.2439 m |
| hand | hand | false | 2 | 36 | 0.3240-0.3390 m | 0.3226-0.3381 m |

## Submission summary

| submission | mode | pixels | correct | wrong occlusions | order fallbacks | incomplete-depth pixels | invalid samples | quantization ambiguities |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| submission-no-depth | none | 192 | 192 | 0 | 192 | 0 | 0 | 0 |
| submission-sparse-depth | sparse | 192 | 192 | 0 | 174 | 174 | 272 | 0 |
| submission-full-depth | full | 192 | 192 | 0 | 0 | 0 | 0 | 0 |

`order fallbacks` includes no-depth submissions and incomplete-depth pixels. `quantization ambiguities` counts pixels where candidate depth intervals overlap after quantization.

## Coverage decisions

Uppercase means a decisive result; lowercase means the result used an ambiguous fallback. `H` hand, `P` panel, `N` system notification, `.` background.

### submission-no-depth (none)

left eye:

```text
NNNNNNNNNNNN
NNPPPPPPPPNN
NNPPPPHHHHHH
NNPPPPHHHHHH
NNPPPPHHHHHH
NNPPPPHHHHHH
NNPPPPHHHHHH
NNNNNNHHHHHH
```

right eye:

```text
NNNNNNNNNNNN
NNPPPPPPPPNN
NNPPPPHHHHHH
NNPPPPHHHHHH
NNPPPPHHHHHH
NNPPPPHHHHHH
NNPPPPHHHHHH
NNNNNNHHHHHH
```

### submission-sparse-depth (sparse)

left eye:

```text
NnnnnNnnnnNn
nnpppppPppnn
nnpppphhhhhh
nNpppphhhhhh
nnpppphhhhhh
NnpppPhhhhhh
nnpppphhhhhh
nnnnNnhhhHhh
```

right eye:

```text
NnnnnNnnnnNn
nnpppppPppnn
nnpppphhhhhh
nNpppphhhhhh
nnpppphhhhhh
NnpppPhhhhhh
nnpppphhhhhh
nnnnNnhhhHhh
```

### submission-full-depth (full)

left eye:

```text
NNNNNNNNNNNN
NNPPPPPPPPNN
NNPPPPHHHHHH
NNPPPPHHHHHH
NNPPPPHHHHHH
NNPPPPHHHHHH
NNPPPPHHHHHH
NNNNNNHHHHHH
```

right eye:

```text
NNNNNNNNNNNN
NNPPPPPPPPNN
NNPPPPHHHHHH
NNPPPPHHHHHH
NNPPPPHHHHHH
NNPPPPHHHHHH
NNPPPPHHHHHH
NNNNNNHHHHHH
```

## Occlusion errors

No occlusion errors were recorded for the compared submissions.

## Edge cases and negative controls

| case | safe errors | negative control | control errors | detail |
|---|---:|---|---:|---|
| near-ui-quantization | 0 | negative-system-always-front | 2 | hand and system notification quantize to the same bin; stable explicit order keeps the hand in front |
| invalid-depth | 0 | negative-ignore-incomplete-depth | 2 | invalid hand depth is not treated as infinity; safe fallback uses explicit order, while ignoring the hole exposes the panel |
| stereo-isolation | 0 | negative-left-depth-for-right-eye | 1 | left eye sees the hand in front; right eye depth places the panel in front; reusing left depth corrupts the right eye |

The negative controls are not production policies. They demonstrate the errors caused by always promoting system layers, treating invalid samples as usable depth, or reusing left-eye depth for the right eye.
