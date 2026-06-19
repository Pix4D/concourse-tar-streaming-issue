// Standalone reproducer for https://github.com/golang/go/issues/80073
//
// os.Root.Symlink writes Unix-style forward slashes into NTFS reparse points on
// Windows. os.Symlink normalizes them to backslashes. Broken symlinks cause
// robocopy to fail on Windows Server 2019 (ERROR 123 on directory symlinks) but
// succeed on Windows Server 2025 — yet the symlinks remain unreadable on both.
//
// Build and copy to a worker:
//
//	GOOS=windows GOARCH=amd64 go build -o symlink-robocopy-demo.exe symlink-robocopy.go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var robocopyErrorPath = regexp.MustCompile(`(?i)ERROR \d+ \(0x[0-9A-Fa-f]+\) (?:Copying (?:Directory|File)|Accessing (?:Source |Destination )?Directory|Creating Destination Directory) (.+)$`)

func main() {
	baseDir := "symlink-demo"
	dir1 := filepath.Join(baseDir, "dir-1")
	symlinkDirName := "symlink-dir"
	symlinkDir := filepath.Join(baseDir, symlinkDirName)
	targetFile := filepath.Join(dir1, "file-with-content")

	os.RemoveAll(baseDir)
	os.RemoveAll(baseDir + "-robocopy-dest")

	if err := os.MkdirAll(dir1, 0755); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(symlinkDir, 0755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(targetFile, []byte("hello world"), 0644); err != nil {
		panic(err)
	}

	// Intra-repo relative targets with Unix-style separators, as produced by
	// git on Linux and passed verbatim to os.Root.Symlink during tar extract.
	unixStyleFileTarget := "../dir-1/file-with-content"
	unixStyleDirTarget := "../dir-1"

	// --- os.Symlink (normalizes separators on Windows) ---
	if err := os.Symlink(unixStyleFileTarget, filepath.Join(symlinkDir, "os-unix-file")); err != nil {
		fmt.Printf("os.Symlink (file) err: %v\n", err)
	}
	if err := os.Symlink(unixStyleDirTarget, filepath.Join(symlinkDir, "os-unix-dir")); err != nil {
		fmt.Printf("os.Symlink (dir) err: %v\n", err)
	}

	// --- os.Root.Symlink (bug: does NOT normalize separators) ---
	root, err := os.OpenRoot(baseDir)
	if err != nil {
		panic(fmt.Errorf("OpenRoot failed: %v", err))
	}
	defer root.Close()

	if err := root.Symlink(unixStyleFileTarget, filepath.Join(symlinkDirName, "root-unix-file")); err != nil {
		fmt.Printf("root.Symlink (file) err: %v\n", err)
	}
	if err := root.Symlink(unixStyleDirTarget, filepath.Join(symlinkDirName, "root-unix-dir")); err != nil {
		fmt.Printf("root.Symlink (dir) err: %v\n", err)
	}

	type testCase struct {
		description string
		path        string
		isDir       bool
	}

	cases := []testCase{
		{"FILE: os.Symlink   w/ Unix target", filepath.Join(symlinkDir, "os-unix-file"), false},
		{"FILE: root.Symlink w/ Unix target", filepath.Join(symlinkDir, "root-unix-file"), false},
		{"DIR:  os.Symlink   w/ Unix target", filepath.Join(symlinkDir, "os-unix-dir"), true},
		{"DIR:  root.Symlink w/ Unix target", filepath.Join(symlinkDir, "root-unix-dir"), true},
	}

	fmt.Println("--- Symlink resolution (Go issue #80073) ---")
	for _, c := range cases {
		fmt.Printf("\n[%s]\n", c.description)

		target, err := os.Readlink(c.path)
		if err != nil {
			fmt.Printf("  Target Written : ERROR (%v)\n", err)
		} else {
			fmt.Printf("  Target Written : %s\n", target)
		}

		if c.isDir {
			entries, err := os.ReadDir(c.path)
			if err != nil {
				fmt.Printf("  Resolution     : FAILED -> %v\n", err)
			} else {
				fmt.Printf("  Resolution     : SUCCESS (found %d entries)\n", len(entries))
			}
		} else {
			content, err := os.ReadFile(c.path)
			if err != nil {
				fmt.Printf("  Resolution     : FAILED -> %v\n", err)
			} else {
				fmt.Printf("  Resolution     : SUCCESS (content: %q)\n", content)
			}
		}
	}

	fmt.Println("\n--- Robocopy test (baggageclaim /sl flags) ---")
	if runtime.GOOS != "windows" {
		fmt.Println("Skipped: robocopy is only available on Windows")
		fmt.Println("\nExpected on Windows Server 2019: robocopy exit code >= 2 (ERROR 123 on root-unix-dir)")
		fmt.Println("Expected on Windows Server 2025: robocopy exit code <= 1 (copies despite broken symlinks)")
		return
	}

	printRobocopyVersion()

	src, err := filepath.Abs(baseDir)
	if err != nil {
		panic(err)
	}
	dest, err := filepath.Abs(baseDir + "-robocopy-dest")
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll(dest, 0755); err != nil {
		panic(err)
	}

	exitCode := runRobocopyTest(src, dest)

	fmt.Println("\n--- Expected behaviour ---")
	fmt.Println("Windows Server 2019: robocopy fails (exit >= 2) copying broken directory symlinks")
	fmt.Println("Windows Server 2025: robocopy succeeds (exit <= 1) but symlinks remain unreadable")
	fmt.Printf("This run: exit code %d\n", exitCode)
}

func printRobocopyVersion() {
	path, err := exec.LookPath("robocopy")
	if err != nil {
		fmt.Printf("Robocopy version: unknown (not found: %v)\n", err)
		return
	}

	cmd := exec.Command(
		"powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-Command",
		fmt.Sprintf(
			"(Get-Item -LiteralPath '%s').VersionInfo.FileVersion",
			escapePSSingleQuoted(path),
		),
	)

	output, err := cmd.CombinedOutput()
	out := strings.TrimSpace(string(output))
	if err != nil {
		fmt.Printf("Robocopy path: %s\n", path)
		fmt.Printf("Robocopy version: unknown (VersionInfo failed: %v)\n", err)
		return
	}

	fmt.Printf("Robocopy path: %s\n", path)
	fmt.Printf("Robocopy version: %s\n\n", out)
}

func runRobocopyTest(src, dest string) int {
	// Same flags as concourse/worker/baggageclaim/volume/copy/copy_windows.go
	args := []string{
		"/e",
		"/mt",
		"/r:5",
		"/w:5",
		"/sl",
		src,
		dest,
	}

	cmd := exec.Command("robocopy", args...)
	output, err := cmd.CombinedOutput()
	out := strings.TrimSpace(string(output))

	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("Command:")
	fmt.Printf("robocopy.exe %s\n", strings.Join(args, " "))

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	if exitCode <= 1 {
		fmt.Printf("Result: success (exit code %d)\n", exitCode)
	} else {
		fmt.Printf("Result: failed with exit code %d\n", exitCode)
	}

	fmt.Println(strings.Repeat("-", 80))
	fmt.Println("Robocopy output:")
	if out != "" {
		fmt.Println(out)
	} else {
		fmt.Println("(no output)")
	}

	errorPaths := robocopyErrorPaths(out)
	if len(errorPaths) > 0 {
		fmt.Println(strings.Repeat("-", 80))
		fmt.Println("Get-Item diagnostics:")
		fmt.Print(diagnoseRobocopyPaths(errorPaths))
		fmt.Println()
	}

	fmt.Println(strings.Repeat("=", 80))
	return exitCode
}

func robocopyErrorPaths(output string) []string {
	seen := make(map[string]struct{})
	var paths []string

	for _, line := range strings.Split(output, "\n") {
		matches := robocopyErrorPath.FindStringSubmatch(strings.TrimSpace(line))
		if len(matches) < 2 {
			continue
		}

		path := strings.TrimSpace(matches[1])
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}

	return paths
}

func diagnoseRobocopyPaths(paths []string) string {
	var b strings.Builder

	for _, path := range paths {
		b.WriteString("\n\n--- ")
		b.WriteString(path)
		b.WriteString(" ---\n")

		cmd := exec.Command(
			"powershell.exe",
			"-NoProfile",
			"-NonInteractive",
			"-Command",
			fmt.Sprintf("Get-Item -LiteralPath '%s' | Format-List", escapePSSingleQuoted(path)),
		)

		output, err := cmd.CombinedOutput()
		out := strings.TrimSpace(string(output))
		if err != nil {
			b.WriteString(fmt.Sprintf("Get-Item failed: %v\n", err))
		}
		if out != "" {
			b.WriteString(out)
		}
		if err == nil && out == "" {
			b.WriteString("(no output)")
		}
	}

	return b.String()
}

func escapePSSingleQuoted(value string) string {
	return strings.NewReplacer("'", "''").Replace(value)
}
