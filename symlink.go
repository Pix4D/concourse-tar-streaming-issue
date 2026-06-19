// Minimal reproducer for https://github.com/golang/go/issues/80073
//
// Build:
//
//	GOOS=windows GOARCH=amd64 go build -o symlink-demo.exe symlink.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	baseDir := "symlink-demo"
	dir1 := filepath.Join(baseDir, "dir-1")
	symlinkDirName := "symlink-dir"
	symlinkDir := filepath.Join(baseDir, symlinkDirName)
	targetFile := filepath.Join(dir1, "file-with-content")

	os.RemoveAll(baseDir)
	os.MkdirAll(dir1, 0755)
	os.MkdirAll(symlinkDir, 0755)
	os.WriteFile(targetFile, []byte("hello world"), 0644)

	unixStyleFileTarget := "../dir-1/file-with-content"
	winStyleFileTarget := "..\\dir-1\\file-with-content"
	unixStyleDirTarget := "../dir-1"
	winStyleDirTarget := "..\\dir-1"

	os.Symlink(unixStyleFileTarget, filepath.Join(symlinkDir, "os-unix-file"))
	os.Symlink(winStyleFileTarget, filepath.Join(symlinkDir, "os-win-file"))
	os.Symlink(unixStyleDirTarget, filepath.Join(symlinkDir, "os-unix-dir"))
	os.Symlink(winStyleDirTarget, filepath.Join(symlinkDir, "os-win-dir"))

	root, _ := os.OpenRoot(baseDir)
	defer root.Close()

	root.Symlink(unixStyleFileTarget, filepath.Join(symlinkDirName, "root-unix-file"))
	root.Symlink(winStyleFileTarget, filepath.Join(symlinkDirName, "root-win-file"))
	root.Symlink(unixStyleDirTarget, filepath.Join(symlinkDirName, "root-unix-dir"))
	root.Symlink(winStyleDirTarget, filepath.Join(symlinkDirName, "root-win-dir"))

	type testCase struct {
		name, path, inputTarget string
		isDir                   bool
	}
	cases := []testCase{
		{"FILE: os.Symlink   (Win Target) ", filepath.Join(symlinkDir, "os-win-file"), winStyleFileTarget, false},
		{"FILE: root.Symlink (Win Target) ", filepath.Join(symlinkDir, "root-win-file"), winStyleFileTarget, false},
		{"DIR:  os.Symlink   (Win Target) ", filepath.Join(symlinkDir, "os-win-dir"), winStyleDirTarget, true},
		{"DIR:  root.Symlink (Win Target) ", filepath.Join(symlinkDir, "root-win-dir"), winStyleDirTarget, true},

		{"FILE: os.Symlink   (Unix Target)", filepath.Join(symlinkDir, "os-unix-file"), unixStyleFileTarget, false},
		{"FILE: root.Symlink (Unix Target)", filepath.Join(symlinkDir, "root-unix-file"), unixStyleFileTarget, false},
		{"DIR:  os.Symlink   (Unix Target)", filepath.Join(symlinkDir, "os-unix-dir"), unixStyleDirTarget, true},
		{"DIR:  root.Symlink (Unix Target)", filepath.Join(symlinkDir, "root-unix-dir"), unixStyleDirTarget, true},
	}

	for _, c := range cases {
		fmt.Printf("\n[%s]\n", c.name)
		fmt.Printf("  Input Target   : %s\n", c.inputTarget)

		writtenTarget, _ := os.Readlink(c.path)
		fmt.Printf("  Target Written : %s\n", writtenTarget)

		if c.isDir {
			_, err := os.ReadDir(c.path)
			if err != nil {
				fmt.Printf("  Resolution     : FAILED -> %v\n", err)
			} else {
				fmt.Printf("  Resolution     : SUCCESS\n")
			}
		} else {
			_, err := os.ReadFile(c.path)
			if err != nil {
				fmt.Printf("  Resolution     : FAILED -> %v\n", err)
			} else {
				fmt.Printf("  Resolution     : SUCCESS\n")
			}
		}
	}
}
