package watcher

import (
	"fmt"
	"sync"
)

var OriginalWatch = Watch

var (
	mocks   map[string]chan []EventInfo
	mocksmu sync.Mutex
)

func Mock() {
	mocksmu.Lock()
	defer mocksmu.Unlock()

	mocks = map[string]chan []EventInfo{}
	Watch = func(inputPath string) (<-chan []EventInfo, func(), error) {
		mocksmu.Lock()
		defer mocksmu.Unlock()

		mock, hasMock := mocks[inputPath]
		if !hasMock {
			mock = make(chan []EventInfo)
			mocks[inputPath] = mock
		}
		stop := func() { close(mock) }
		return mock, stop, nil
	}
}

func Dispatch(path string) {
	mocksmu.Lock()
	defer mocksmu.Unlock()

	mock, hasMock := mocks[path]
	if !hasMock {
		panic(fmt.Errorf("can't dispatch on unwatched path '%s'", path))
	}
	mock <- []EventInfo{{Path: path}}
}

func Unmock() {
	mocks = nil
	Watch = OriginalWatch
}
