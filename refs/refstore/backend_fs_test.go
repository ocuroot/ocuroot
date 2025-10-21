package refstore

import (
	"os/exec"
	"testing"
)

func TestFSBackend(t *testing.T) {
	doTestBackendSetGet(t, func() DocumentBackend {
		be := NewFsBackend(emptyTestDir("fs/set-get"))
		return be
	})
	doTestBackendMatch(t, func() DocumentBackend {
		be := NewFsBackend(emptyTestDir("fs/match"))
		return be
	})
	doTestBackendInfo(t, func() DocumentBackend {
		be := NewFsBackend(emptyTestDir("fs/info"))
		return be
	})
}

func emptyTestDir(dir string) string {
	dirPath := "testdata/" + dir

	// Remove if it exists from previous run
	exec.Command("rm", "-rf", dirPath).Run()

	return dirPath
}
