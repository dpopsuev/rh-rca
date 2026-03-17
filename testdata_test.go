package rca_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dpopsuev/rh-rca"
)

func init() {
	data, err := os.ReadFile(filepath.Join(testdataDir(), "vocabulary.yaml"))
	if err == nil {
		rca.InitVocab(data)
	}
}

func testdataDir() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "testdata")
}

func readTestdata(t *testing.T, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testdataDir(), rel))
	if err != nil {
		t.Fatalf("read testdata %s: %v", rel, err)
	}
	return data
}

func testdataPromptFSExternal() fs.FS {
	sub, _ := fs.Sub(os.DirFS(testdataDir()), "prompts")
	return sub
}
