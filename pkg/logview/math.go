package logview

import "cmp"

func modulo(i, n int) int {
	if n == 0 {
		return 0
	}
	return ((i % n) + n) % n
}

func clamp[T cmp.Ordered](lower, upper, val T) T {
	return max(lower, min(upper, val))
}
