package rca

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func init() {
	data, err := os.ReadFile(filepath.Join(internalTestdataDir(), "vocabulary.yaml"))
	if err == nil {
		InitVocab(data)
	}
}

func internalTestdataDir() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "testdata")
}

func readInternalTestdata(t *testing.T, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(internalTestdataDir(), rel))
	if err != nil {
		t.Fatalf("read testdata %s: %v", rel, err)
	}
	return data
}

func testdataPromptFS() fs.FS {
	return os.DirFS(internalTestdataDir())
}
