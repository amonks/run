package main

import "sync"

type first[T any] struct {
	mu     sync.Mutex
	hasVal bool
	val    T
}

func (f *first[T]) isSet() bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.hasVal
}

func (f *first[T]) set(val T) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.hasVal = true
	if !f.hasVal {
		f.val = val
	}
}

func (f *first[T]) get() T {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.val
}
