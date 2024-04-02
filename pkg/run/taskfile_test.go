package run

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaskfileNestingWithDir(t *testing.T) {
	ts, err := Load("./testdata/very-nested")
	assert.NoError(t, err)

	testNesting(t, ts)

	assert.Equal(t, "testdata/very-nested",
		ts.Get("test").(*scriptTask).dir)
	assert.Equal(t, "testdata/very-nested/child",
		ts.Get("child/test").(*scriptTask).dir)
	assert.Equal(t, "testdata/very-nested/child/grandchild",
		ts.Get("child/grandchild/test").(*scriptTask).dir)
}

func TestTaskfileNestingWithParentDir(t *testing.T) {
	os.Chdir("testdata/very-nested/child")
	defer os.Chdir("../../..")

	ts, err := Load("..")
	assert.NoError(t, err)

	testNesting(t, ts)

	assert.Equal(t, "..",
		ts.Get("test").(*scriptTask).dir)
	assert.Equal(t, "../child",
		ts.Get("child/test").(*scriptTask).dir)
	assert.Equal(t, "../child/grandchild",
		ts.Get("child/grandchild/test").(*scriptTask).dir)
}

func TestTaskfileNestingWithDot(t *testing.T) {
	os.Chdir("testdata/very-nested")
	defer os.Chdir("../..")

	ts, err := Load(".")
	assert.NoError(t, err)

	testNesting(t, ts)

	assert.Equal(t, ".",
		ts.Get("test").(*scriptTask).dir)
	assert.Equal(t, "child",
		ts.Get("child/test").(*scriptTask).dir)
	assert.Equal(t, "child/grandchild",
		ts.Get("child/grandchild/test").(*scriptTask).dir)
}

func testNesting(t *testing.T, ts Tasks) {
	metas := map[string]TaskMetadata{}
	for _, id := range ts.IDs() {
		metas[id] = ts.Get(id).Metadata()
	}

	assert.Equal(t, map[string]TaskMetadata{
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
