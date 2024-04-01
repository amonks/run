package color

import (
	"hash/fnv"
	"math"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

func RenderHash(s string) string {
	return globalColorer.render(s)
}

func Hash(s string) lipgloss.AdaptiveColor {
	return globalColorer.hash(s)
}

var globalColorer = &colorer{colorCache: map[string]lipgloss.AdaptiveColor{}}

type colorer struct {
	mu          sync.Mutex
	colorCache  map[string]lipgloss.AdaptiveColor
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

func (c *colorer) hash(s string) lipgloss.AdaptiveColor {
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
	c.colorCache[s] = lipgloss.AdaptiveColor{
		Dark:  light,
		Light: dark,
	}
	return c.colorCache[s]
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
