# Internal Color

## Overview

The `internal/color` package provides deterministic hash-based color generation and a Solarized color palette for terminal output.

## Hash-Based Colors

- `Hash(s string) lipgloss.AdaptiveColor`: generates a deterministic color from a string.
  - Uses FNV-32a hash to derive a hue value in [0, 1].
  - Creates HSL colors with full saturation: lightness 0.7 for dark backgrounds, 0.3 for light backgrounds.
  - Results are cached per string for performance.
- `RenderHash(s string) string`: renders a string in its own hash-derived color. Also cached.

## Solarized Palette

Exports named colors from the [Solarized](https://ethanschoonover.com/solarized/) palette:

- Accent colors: `Yellow`, `Orange`, `Red`, `Magenta`, `Violet`, `Blue`, `Cyan`, `Green`.
- Base colors as adaptive (dark/light aware) pairs: `XXXLight`/`XXXDark` through `Light`/`Dark`.

## Color Conversion

Internal HSL-to-RGB conversion:
- `hsl` type with `h`, `s`, `l` fields in [0, 1].
- `rgb` type with `r`, `g`, `b` fields in [0, 255].
- `hex()` method produces `#RRGGBB` strings for lipgloss.

## Thread Safety

The global `colorer` uses a `sync.Mutex` to protect both the color and render caches.
