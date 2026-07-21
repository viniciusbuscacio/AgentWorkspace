package sandbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aw/internal/domain"
)

// Symlink/traversal defense, ported from AW2's sandbox-fs
// assertSandboxResolvedPath: validate the user-facing path, then realpath the
// nearest existing ancestor and validate the resolved path too, so a symlink
// inside an allowed folder cannot escape the sandbox.

// DeniedError reports a path blocked by the permissions sandbox. Callers
// surface it as a structured blocked result, not a hidden failure.
type DeniedError struct {
	Path string
	Mode domain.SandboxMode
}

func (e *DeniedError) Error() string {
	return fmt.Sprintf("access to %q is blocked by Permissions (mode %s). Change Permissions in the app.", e.Path, e.Mode)
}

// IsDenied reports whether err is a sandbox denial.
func IsDenied(err error) bool {
	var denied *DeniedError
	return errors.As(err, &denied)
}

// ResolveAndCheck expands and validates a path, then re-validates the
// realpath of its nearest existing ancestor (symlink escape defense).
// Returns the normalized (non-realpathed) absolute path.
func ResolveAndCheck(path string, policy Policy) (string, error) {
	normalized := ExpandPath(path)
	if !IsPathAllowed(normalized, policy) {
		return "", &DeniedError{Path: path, Mode: policy.Config.Mode}
	}
	existing, err := findExistingAncestor(normalized)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", err
	}
	if !realPathAllowed(resolved, policy) {
		return "", &DeniedError{Path: path, Mode: policy.Config.Mode}
	}
	return normalized, nil
}

// findExistingAncestor walks up from path to the nearest component that
// exists on disk (the path itself when it exists).
func findExistingAncestor(path string) (string, error) {
	current := path
	for {
		if _, err := os.Lstat(current); err == nil {
			return current, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", os.ErrNotExist
		}
		current = parent
	}
}

// realPathAllowed accepts a resolved path that either passes IsPathAllowed
// directly or lands inside the realpath of an allowed root. The latter covers
// roots that are themselves behind symlinks (macOS /tmp → /private/tmp).
func realPathAllowed(resolved string, policy Policy) bool {
	if IsPathAllowed(resolved, policy) {
		return true
	}
	roots := []string{policy.WorkspaceRoot}
	if policy.Config.Mode == domain.SandboxPermitList {
		roots = append(roots, builtinAllowedPermitList...)
		roots = append(roots, policy.Config.AllowedFolders...)
	}
	for _, root := range roots {
		if strings.TrimSpace(root) == "" || !IsPathAllowed(root, policy) {
			continue
		}
		realRoot, err := filepath.EvalSymlinks(ExpandPath(root))
		if err != nil {
			continue
		}
		if isInsideResolvedPath(resolved, realRoot) {
			return true
		}
	}
	return false
}

// isInsideResolvedPath reports whether filePath is folder or inside it,
// comparing already-resolved absolute paths.
func isInsideResolvedPath(filePath string, folder string) bool {
	relative, err := filepath.Rel(folder, filePath)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative))
}
