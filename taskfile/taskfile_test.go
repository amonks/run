package taskfile

import (
	"os"
	"testing"

	"github.com/amonks/run/task"
	"github.com/stretchr/testify/assert"
)

type dirGetter interface{ Dir() string }

func TestTaskfileNestingWithDir(t *testing.T) {
	ts, err := Load("./testdata/very-nested")
	assert.NoError(t, err)

	testNesting(t, ts)

	assert.Equal(t, "testdata/very-nested",
		ts.Get("test").(dirGetter).Dir())
	assert.Equal(t, "testdata/very-nested/child",
		ts.Get("child/test").(dirGetter).Dir())
	assert.Equal(t, "testdata/very-nested/child/grandchild",
		ts.Get("child/grandchild/test").(dirGetter).Dir())
}

func TestTaskfileNestingWithParentDir(t *testing.T) {
	os.Chdir("testdata/very-nested/child")
	defer os.Chdir("../../..")

	ts, err := Load("..")
	assert.NoError(t, err)

	testNesting(t, ts)

	assert.Equal(t, "..",
		ts.Get("test").(dirGetter).Dir())
	assert.Equal(t, "../child",
		ts.Get("child/test").(dirGetter).Dir())
	assert.Equal(t, "../child/grandchild",
		ts.Get("child/grandchild/test").(dirGetter).Dir())
}

func TestTaskfileNestingWithDot(t *testing.T) {
	os.Chdir("testdata/very-nested")
	defer os.Chdir("../..")

	ts, err := Load(".")
	assert.NoError(t, err)

	testNesting(t, ts)

	assert.Equal(t, ".",
		ts.Get("test").(dirGetter).Dir())
	assert.Equal(t, "child",
		ts.Get("child/test").(dirGetter).Dir())
	assert.Equal(t, "child/grandchild",
		ts.Get("child/grandchild/test").(dirGetter).Dir())
}

func testNesting(t *testing.T, ts task.Library) {
	metas := map[string]task.TaskMetadata{}
	for _, id := range ts.IDs() {
		metas[id] = ts.Get(id).Metadata()
	}

	assert.Equal(t, map[string]task.TaskMetadata{
		"test": {
			ID:           "test",
			Description:  `"touch parent.stamp"`,
			Type:         "short",
			Dependencies: []string{"child/test"},
			Watch:        []string{"file"},
		},
		"child/test": {
			ID:           "child/test",
			Description:  `"touch child.stamp"`,
			Type:         "short",
			Dependencies: []string{"child/grandchild/test"},
			Watch:        []string{"child/file"},
		},
		"child/grandchild/test": {
			ID:          "child/grandchild/test",
			Description: `"touch grandchild.stamp"`,
			Type:        "short",
			Watch:       []string{"child/grandchild/file"},
		},
	}, metas)
}
