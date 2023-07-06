package runner

import (
	"fmt"
	"hash/fnv"
	"math"
)

func colorHash(s string) string {
	hue := float64(hash(s)) / float64(math.MaxUint32)
	c := hsl{hue, 1.0, 0.7}.rgb()
	return c.hex()
}

type rgb struct {
	// [0-255]
	r, g, b int
}

func (c rgb) hex() string {
	return fmt.Sprintf("#%02X%02X%02X", c.r, c.g, c.b)
}

type hsl struct {
	// [0-1]
	h, s, l float64
}

func (c hsl) rgb() rgb {
	if c.s == 0 {
		v := int(c.l * 255)
		return rgb{v, v, v}
	}

	var r, g, b float64

	var q, p float64
	if c.l < 0.5 {
		q = c.l * (1.0 + c.s)
	} else {
		q = c.l + c.s - c.l*c.s
	}
	p = 2.0*c.l - q
	r = hueToRGB(p, q, c.h+(1.0/3.0))
	g = hueToRGB(p, q, c.h)
	b = hueToRGB(p, q, c.h-(1.0/3.0))

	return rgb{int(r * 255), int(g * 255), int(b * 255)}
}

func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t += 1.0
	} else if t > 1.0 {
		t -= 1.0
	}

	switch true {
	case t < 1.0/6.0:
		return p + (q-p)*6*t
	case t < 1.0/2.0:
		return q
	case t < 2.0/3.0:
		return p + (q-p)*(2.0/3.0-t)*6
	default:
		return p
	}
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
