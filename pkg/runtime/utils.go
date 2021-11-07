package runtime

import (
	"os"
	"path/filepath"

	"github.com/wagoodman/bashful/utils"
)

func DirectoryExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func EnsureFileDir(f string) {
	dir, derr := filepath.Abs(filepath.Dir(f))
	utils.CheckError(derr, "file dir not ensured")
	if !DirectoryExists(dir) {
		os.MkdirAll(dir, 0700)
	}
	return
}

func EnsureDirectoryExists(d string) bool {
	if DirectoryExists(d) {
		return true
	}
	os.MkdirAll(d, 0700)
	return DirectoryExists(d)
}
