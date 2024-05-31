package taskfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaskfileNestingWithDir(t *testing.T) {
	tf, err := Load("testdata/very-nested")
	assert.NoError(t, err)

	testNesting(t, "../..", tf)

	assert.Equal(t, "testdata/very-nested", tf.find("test").Dir)
	assert.Equal(t, "testdata/very-nested/child", tf.find("child/test").Dir)
	assert.Equal(t, "testdata/very-nested/child/grandchild", tf.find("child/grandchild/test").Dir)
}

func TestTaskfileNestingWithParentDir(t *testing.T) {
	os.Chdir("testdata/very-nested/child")
	defer os.Chdir("../../..")

	tf, err := Load("..")
	assert.NoError(t, err)

	testNesting(t, "..", tf)

	assert.Equal(t, "..", tf.find("test").Dir)
	assert.Equal(t, "../child", tf.find("child/test").Dir)
	assert.Equal(t, "../child/grandchild", tf.find("child/grandchild/test").Dir)
}

func TestTaskfileNestingWithDot(t *testing.T) {
	os.Chdir("testdata/very-nested")
	defer os.Chdir("../..")

	tf, err := Load(".")
	assert.NoError(t, err)

	testNesting(t, ".", tf)

	assert.Equal(t, ".", tf.find("test").Dir)
	assert.Equal(t, "child", tf.find("child/test").Dir)
	assert.Equal(t, "child/grandchild", tf.find("child/grandchild/test").Dir)
}

func testNesting(t *testing.T, parent string, tf Taskfile) {
	ts := map[string]Task{}
	for _, task := range tf.Tasks {
		task.Dir = ""
		ts[task.ID] = task
	}

	assert.Equal(t, "test", tf.Tasks[0].ID)
	assert.Equal(t, "child/test", tf.Tasks[1].ID)
	assert.Equal(t, "child/grandchild/test", tf.Tasks[2].ID)

	assert.Equal(t, filepath.Join(parent, "test"), filepath.Join(parent, tf.Tasks[0].ID))
	assert.Equal(t, filepath.Join(parent, "child/test"), filepath.Join(parent, tf.Tasks[1].ID))
	assert.Equal(t, filepath.Join(parent, "child/grandchild/test"), filepath.Join(parent, tf.Tasks[2].ID))

	assert.EqualValues(t, map[string]Task{
		"test": {
			ID:           "test",
			Type:         "short",
			Dependencies: []string{"child/test"},
			Watch:        []string{"file"},
			CMD:          "touch parent.stamp",
		},
		"child/test": {
			ID:           "child/test",
			Type:         "short",
			Dependencies: []string{"child/grandchild/test"},
			Watch:        []string{"child/file"},
			CMD:          "touch child.stamp",
		},
		"child/grandchild/test": {
			ID:    "child/grandchild/test",
			Type:  "short",
			Watch: []string{"child/grandchild/file"},
			CMD:   "touch grandchild.stamp",
		},
	}, ts)
}
