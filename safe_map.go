package run

import "sync"

type safeMap[T any] struct {
	mu sync.Mutex
	m  map[string]T
}

func newSafeMap[T any]() *safeMap[T] {
	return &safeMap[T]{
		m: map[string]T{},
	}
}

func (m *safeMap[T]) has(k string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.m[k]
	return ok
}

func (m *safeMap[T]) get(k string) T {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.m[k]
}

func (m *safeMap[T]) del(k string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.m, k)
}

func (m *safeMap[T]) set(k string, v T) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.m[k] = v
}

func (m *safeMap[T]) keys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var ks []string
	for k := range m.m {
		ks = append(ks, k)
	}
	return ks
}
