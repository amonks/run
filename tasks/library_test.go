package tasks_test

import (
	"testing"

	"github.com/amonks/run/internal/fixtures"
	"github.com/amonks/run/tasks"
	"github.com/stretchr/testify/assert"
)

func TestNewLibrary(t *testing.T) {
	t.Run("ignores duplicates", func(t *testing.T) {
		ts := tasks.NewLibrary(
			fixtures.NewTask("root"),
			fixtures.NewTask("root"),
		)
		assert.Equal(t, []string{"root"}, ts.IDs())
	})
	t.Run("preserves order", func(t *testing.T) {
		ts := tasks.NewLibrary(
			fixtures.NewTask("2"),
			fixtures.NewTask("3"),
			fixtures.NewTask("1"),
		)
		assert.Equal(t, []string{"2", "3", "1"}, ts.IDs())
	})
	t.Run("preserves order with duplicates", func(t *testing.T) {
		ts := tasks.NewLibrary(
			fixtures.NewTask("2"),
			fixtures.NewTask("3"),
			fixtures.NewTask("1"),
			fixtures.NewTask("1"),
			fixtures.NewTask("2"),
			fixtures.NewTask("3"),
		)
		assert.Equal(t, []string{"2", "3", "1"}, ts.IDs())
	})
}

// TODO
func TestValidate(t *testing.T) {}

func TestWatches(t *testing.T) {
	t.Run("removes duplicates", func(t *testing.T) {
		ts := tasks.NewLibrary(
			fixtures.NewTask("a").WithWatch("f1", "f2", "f3"),
			fixtures.NewTask("b").WithWatch("f3", "f4", "f5"),
		)
		assert.Equal(t, []string{"f1", "f2", "f3", "f4", "f5"}, ts.Watches())
	})
}

func TestSubtree(t *testing.T) {
	t.Run("deduplicates and preserves order", func(t *testing.T) {
		ts := tasks.NewLibrary(
			fixtures.NewTask("useful"),
			fixtures.NewTask("a").WithDependencies("aa", "ab", "useful"),
			fixtures.NewTask("spam"),
			fixtures.NewTask("aa").WithDependencies("useful", "ab"),
			fixtures.NewTask("ab").WithDependencies("useful"),
		)
		assert.Equal(t, []string{"useful", "a", "aa", "ab"}, ts.Subtree("a").IDs())
	})

	t.Run("deduplicates and preserves order with multiple roots", func(t *testing.T) {
		ts := tasks.NewLibrary(
			fixtures.NewTask("common"),
			fixtures.NewTask("root-left").WithDependencies("common", "left"),
			fixtures.NewTask("right"),
			fixtures.NewTask("spam"),
			fixtures.NewTask("root-right").WithDependencies("common", "right"),
			fixtures.NewTask("left"),
		)
		assert.Equal(t, []string{"common", "root-left", "right", "root-right", "left"}, ts.Subtree("root-right", "root-left").IDs())
	})

	t.Run("returns the nil slice if nothing matches", func(t *testing.T) {
		ts := tasks.NewLibrary(
			fixtures.NewTask("a"),
			fixtures.NewTask("b"),
			fixtures.NewTask("c"),
		)
		assert.Equal(t, []string(nil), ts.Subtree("d").IDs())
	})
}

func TestWithDependency(t *testing.T) {
	t.Run("preserves order", func(t *testing.T) {
		ts := tasks.NewLibrary(
			fixtures.NewTask("a").WithDependencies("dep"),
			fixtures.NewTask("b").WithDependencies("not dep"),
			fixtures.NewTask("c").WithDependencies("dep"),
		)
		assert.Equal(t, []string{"a", "c"}, ts.WithDependency("dep"))
	})
}

func TestWithTrigger(t *testing.T) {
	t.Run("preserves order", func(t *testing.T) {
		ts := tasks.NewLibrary(
			fixtures.NewTask("a").WithTriggers("trig"),
			fixtures.NewTask("b").WithTriggers("not trig"),
			fixtures.NewTask("c").WithTriggers("trig"),
		)
		assert.Equal(t, []string{"a", "c"}, ts.WithTrigger("trig"))
	})
}

func TestWithWatch(t *testing.T) {
	t.Run("preserves order", func(t *testing.T) {
		ts := tasks.NewLibrary(
			fixtures.NewTask("a").WithWatch("path"),
			fixtures.NewTask("b").WithWatch("not path"),
			fixtures.NewTask("c").WithWatch("path"),
		)
		assert.Equal(t, []string{"a", "c"}, ts.WithWatch("path"))
	})
}

