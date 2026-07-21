// Package selfdev contains filesystem helpers for aw's self-development mode,
// where the agent edits its own source tree. These are infrastructure concerns
// (filesystem probing) kept out of the interface layer.
package selfdev

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveRepoRoot resolves the aw source repository root for self-dev mode.
// If configured is set it is validated; otherwise the root is discovered by
// walking up from the current working directory and the executable location
// until a go.mod is found.
func ResolveRepoRoot(configured string) (string, error) {
	if strings.TrimSpace(configured) != "" {
		return validateRepoRoot(configured)
	}
	var starts []string
	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		starts = append(starts, cwd)
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		starts = append(starts, filepath.Dir(exe))
	}
	for _, start := range starts {
		if root, err := findRepoRoot(start); err == nil {
			return root, nil
		}
	}
	return "", fmt.Errorf("self-dev repo root not found; set selfDev.repoRoot to the aw source directory")
}

func validateRepoRoot(root string) (string, error) {
	abs, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return "", fmt.Errorf("resolve self-dev repo root: %w", err)
	}
	info, err := os.Stat(filepath.Join(abs, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("self-dev repo root %q must contain go.mod: %w", abs, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("self-dev repo root %q has go.mod as a directory", abs)
	}
	return abs, nil
}

func findRepoRoot(start string) (string, error) {
	dir, err := filepath.Abs(strings.TrimSpace(start))
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %q", start)
		}
		dir = parent
	}
}
