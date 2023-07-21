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
		ts["test"].(*scriptTask).dir)
	assert.Equal(t, "testdata/very-nested/child",
		ts["child/test"].(*scriptTask).dir)
	assert.Equal(t, "testdata/very-nested/child/grandchild",
		ts["child/grandchild/test"].(*scriptTask).dir)
}

func TestTaskfileNestingWithParentDir(t *testing.T) {
	os.Chdir("testdata/very-nested/child")
	defer os.Chdir("../../..")

	ts, err := Load("..")
	assert.NoError(t, err)

	testNesting(t, ts)

	assert.Equal(t, "..",
		ts["test"].(*scriptTask).dir)
	assert.Equal(t, "../child",
		ts["child/test"].(*scriptTask).dir)
	assert.Equal(t, "../child/grandchild",
		ts["child/grandchild/test"].(*scriptTask).dir)
}

func TestTaskfileNestingWithDot(t *testing.T) {
	os.Chdir("testdata/very-nested")
	defer os.Chdir("../..")

	ts, err := Load(".")
	assert.NoError(t, err)

	testNesting(t, ts)

	assert.Equal(t, ".",
		ts["test"].(*scriptTask).dir)
	assert.Equal(t, "child",
		ts["child/test"].(*scriptTask).dir)
	assert.Equal(t, "child/grandchild",
		ts["child/grandchild/test"].(*scriptTask).dir)
}

func testNesting(t *testing.T, ts Tasks) {
	metas := map[string]TaskMetadata{}
	for id, t := range ts {
		metas[id] = t.Metadata()
	}

	assert.Equal(t, map[string]TaskMetadata{
		"test": {
			ID:           "test",
			Type:         "short",
			Dependencies: []string{"child/test"},
			Watch:        []string{"file"},
		},
		"child/test": {
			ID:           "child/test",
			Type:         "short",
			Dependencies: []string{"child/grandchild/test"},
			Watch:        []string{"child/file"},
		},
		"child/grandchild/test": {
			ID:    "child/grandchild/test",
			Type:  "short",
			Watch: []string{"child/grandchild/file"},
		},
	}, metas)
}
