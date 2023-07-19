package run

import "testing"

func TestColorHash(t *testing.T) {
	for _, tc := range []struct {
		s        string
		expected string
	}{
		{"dev", "#FF65FE"},
	} {
		got := colorHash(tc.s)
		if got != tc.expected {
			t.Errorf(`colorHash("%s") = %s, got %s`, tc.s, tc.expected, got)
		}
	}
}

func TestRGB(t *testing.T) {
	for _, tc := range []struct {
		hsl hsl
		rgb rgb
	}{
		{hsl{0, 0, 0}, rgb{0, 0, 0}},
		{hsl{0, 1.0, 1.0}, rgb{255, 255, 255}},
	} {
		got := tc.hsl.rgb()
		if got != tc.rgb {
			t.Errorf(`rgb(%+v) = %+v, got %+v`, tc.hsl, tc.rgb, got)
		}
	}
}

func TestHex(t *testing.T) {
	for _, tc := range []struct {
		rgb rgb
		hex string
	}{
		{rgb{255, 255, 255}, "#FFFFFF"},
		{rgb{255, 0, 0}, "#FF0000"},
		{rgb{0, 255, 255}, "#00FFFF"},
		{rgb{0, 0, 0}, "#000000"},
	} {
		got := tc.rgb.hex()
		if got != tc.hex {
			t.Errorf(`hex(%+v) = %s, got %s`, tc.rgb, tc.hex, got)
		}
	}
}
