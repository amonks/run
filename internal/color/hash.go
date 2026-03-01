package color

import (
	"hash/fnv"
	"image/color"
	"math"
	"sync"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
)

func RenderHash(s string) string {
	return globalColorer.render(s)
}

func Hash(s string) compat.AdaptiveColor {
	return globalColorer.hash(s)
}

var globalColorer = &colorer{
	colorCache:  map[string]compat.AdaptiveColor{},
	renderCache: map[string]string{},
}

type colorer struct {
	mu          sync.Mutex
	colorCache  map[string]compat.AdaptiveColor
	renderCache map[string]string
}

func (c *colorer) render(s string) string {
	clr := c.hash(s)

	c.mu.Lock()
	defer c.mu.Unlock()

	if out, ok := c.renderCache[s]; ok {
		return out
	}
	c.renderCache[s] = lipgloss.NewStyle().Foreground(clr).Render(s)
	return c.renderCache[s]
}

func (c *colorer) hash(s string) compat.AdaptiveColor {
	c.mu.Lock()
	defer c.mu.Unlock()

	if clr, ok := c.colorCache[s]; ok {
		return clr
	}
	hue := float64(hashStr(s)) / float64(math.MaxUint32)
	var (
		light = hsl{hue, 1.0, 0.7}.rgb().hex()
		dark  = hsl{hue, 1.0, 0.3}.rgb().hex()
	)
	c.colorCache[s] = compat.AdaptiveColor{
		Dark:  lipgloss.Color(light),
		Light: lipgloss.Color(dark),
	}
	return c.colorCache[s]
}

func hashStr(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// Ensure compat.AdaptiveColor satisfies color.Color for callers
// that pass Hash() results to Foreground().
var _ color.Color = compat.AdaptiveColor{}
