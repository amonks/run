package main

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestFirst(t *testing.T) {
	f := first[string]{}
	assert.Equal(t, f.get(), "")

	f.set("one")
	assert.Equal(t, f.get(), "one")

	f.set("two")
	assert.Equal(t, f.get(), "one")
}
