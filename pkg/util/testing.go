package util

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kruntime "k8s.io/apimachinery/pkg/runtime"
)

// CompareJSONObjs compares two objects via their JSON representations. This
// is much easier to debug that comparing the objects directly.
func CompareJSONObjs(t *testing.T, exp kruntime.Object, actual kruntime.Object) {
	expBytes, err := json.Marshal(exp)
	if err != nil {
		assert.FailNow(t, "Error marshalling expected object to JSON", err)
	}

	actualBytes, err := json.Marshal(actual)
	if err != nil {
		assert.FailNow(t, "Error marshalling actual object to JSON", err)
	}

	assert.JSONEq(t, string(expBytes), string(actualBytes))
}

// WriteFiles takes a map of paths to file contents and uses this to write out files to
// the file system.
func WriteFiles(t *testing.T, baseDir string, files map[string]string) {
	for path, contents := range files {
		fullPath := filepath.Join(baseDir, path)
		fullPathDir := filepath.Dir(fullPath)

		ok, err := DirExists(fullPathDir)
		if err != nil {
			assert.FailNow(t, "Error checking dir: %+v", err)
		}

		if !ok {
			err = os.MkdirAll(fullPathDir, 0755)
			if err != nil {
				assert.FailNow(t, "Error creating dir: %+v", err)
			}
		}

		err = ioutil.WriteFile(fullPath, []byte(contents), 0644)
		if err != nil {
			assert.FailNow(t, "Error creating file: %+v", err)
		}
	}
}

// GetContents returns the contents of a directory as a map from
// file name to string content lines.
func GetContents(t *testing.T, root string) map[string][]string {
	contentsMap := map[string][]string{}

	// Process as a directory
	err := filepath.Walk(
		root,
		func(subPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			contents, err := ioutil.ReadFile(subPath)
			if err != nil {
				return err
			}

			relPath, err := filepath.Rel(root, subPath)
			if err != nil {
				return err
			}

			lines := []string{}
			for _, line := range bytes.Split(contents, []byte("\n")) {
				lines = append(lines, string(line))
			}

			contentsMap[relPath] = lines
			return nil
		},
	)
	require.Nil(t, err)

	return contentsMap
}

// GetFileContents gets the string contents of a single file.
func GetFileContents(t *testing.T, path string) string {
	contents, err := ioutil.ReadFile(path)
	require.NoError(t, err)
	return string(contents)
}
