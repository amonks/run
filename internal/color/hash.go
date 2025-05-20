package color

import (
	"hash/fnv"
	"math"
	"sync"

	lgcompat "github.com/charmbracelet/lipgloss/v2/compat"
	"github.com/charmbracelet/lipgloss/v2"
)

func RenderHash(s string) string {
	return globalColorer.render(s)
}

func Hash(s string) lgcompat.AdaptiveColor {
	return globalColorer.hash(s)
}

var globalColorer = &colorer{
	colorCache:  map[string]lgcompat.AdaptiveColor{},
	renderCache: map[string]string{},
}

type colorer struct {
	mu          sync.Mutex
	colorCache  map[string]lgcompat.AdaptiveColor
	renderCache map[string]string
}

func (c *colorer) render(s string) string {
	color := c.hash(s)

	c.mu.Lock()
	defer c.mu.Unlock()

	if out, ok := c.renderCache[s]; ok {
		return out
	}
	c.renderCache[s] = lipgloss.NewStyle().Foreground(color).Render(s)
	return c.renderCache[s]
}

func (c *colorer) hash(s string) lgcompat.AdaptiveColor {
	c.mu.Lock()
	defer c.mu.Unlock()

	if color, ok := c.colorCache[s]; ok {
		return color
	}
	hue := float64(hash(s)) / float64(math.MaxUint32)
	var (
		light = hsl{hue, 1.0, 0.7}.rgb().hex()
		dark  = hsl{hue, 1.0, 0.3}.rgb().hex()
	)
	c.colorCache[s] = lgcompat.AdaptiveColor{
		Dark:  lipgloss.Color(light),
		Light: lipgloss.Color(dark),
	}
	return c.colorCache[s]
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
