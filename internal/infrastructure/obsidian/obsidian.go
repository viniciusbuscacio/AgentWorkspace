// Package obsidian implements the Obsidian module's vault-jailed file
// operations. The user-chosen vault folder is the jail: every path is
// resolved relative to it and requests that escape it (.. traversal,
// absolute paths outside, symlinked parents) are rejected. The .obsidian
// configuration folder is invisible to list/search and refused for writes.
package obsidian

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"aw/internal/domain"
)

const (
	// ReadCap bounds obsidian.read output (runes).
	ReadCap = 24000
	// AlwaysReadWarnChars is the total size (runes) above which the module
	// page warns that the always-read notes are getting heavy. The notes are
	// injected IN FULL regardless — user decision 2026-07-10: no cap, warn.
	AlwaysReadWarnChars = 15000
	// searchMaxDefault caps search results when the caller does not.
	searchMaxDefault = 20
	// snippetRunes is the context window around a content match.
	snippetRunes = 160
	// walkFileLimit is a runaway guard for pathological vaults.
	walkFileLimit = 50000
)

// Resolve turns a vault-relative path into an absolute one, enforcing the
// jail. Empty rel resolves to the vault root.
func Resolve(vaultDir, rel string) (string, error) {
	root := strings.TrimSpace(vaultDir)
	if root == "" {
		return "", fmt.Errorf("no Obsidian vault folder is configured — set it in the Obsidian module")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	// Resolve symlinks in the ROOT so the prefix check below compares real
	// paths (e.g. /tmp vs /private/tmp on macOS).
	if realRoot, err := filepath.EvalSymlinks(rootAbs); err == nil {
		rootAbs = realRoot
	}
	target := filepath.Join(rootAbs, filepath.FromSlash(strings.TrimSpace(rel)))
	target = filepath.Clean(target)
	if target != rootAbs && !strings.HasPrefix(target, rootAbs+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes the Obsidian vault: %s", rel)
	}
	return target, nil
}

// relPath returns the vault-relative slash path for display.
func relPath(rootAbs, target string) string {
	rel, err := filepath.Rel(rootAbs, target)
	if err != nil {
		return target
	}
	return filepath.ToSlash(rel)
}

func isConfigDir(rel string) bool {
	return rel == ".obsidian" || strings.HasPrefix(rel, ".obsidian/")
}

// List returns the entries directly under folder (vault root when empty),
// hiding dotfiles (including .obsidian). Folders first, then notes, sorted.
func List(vaultDir, folder string) ([]domain.ObsidianEntry, error) {
	dir, err := Resolve(vaultDir, folder)
	if err != nil {
		return nil, err
	}
	rootAbs, err := Resolve(vaultDir, "")
	if err != nil {
		return nil, err
	}
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	entries := make([]domain.ObsidianEntry, 0, len(items))
	for _, item := range items {
		name := item.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		full := filepath.Join(dir, name)
		e := domain.ObsidianEntry{Path: relPath(rootAbs, full), IsDir: item.IsDir()}
		if info, err := item.Info(); err == nil && !item.IsDir() {
			e.Size = info.Size()
		}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Path < entries[j].Path
	})
	return entries, nil
}

// Search walks the vault's Markdown notes matching query against file names
// and contents (case-insensitive). Dotfolders are skipped.
func Search(vaultDir, query string, limit int) ([]domain.ObsidianMatch, error) {
	rootAbs, err := Resolve(vaultDir, "")
	if err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if limit <= 0 || limit > 100 {
		limit = searchMaxDefault
	}
	needle := strings.ToLower(query)
	matches := []domain.ObsidianMatch{}
	seen := 0
	err = filepath.WalkDir(rootAbs, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // unreadable entries are skipped, not fatal
		}
		if len(matches) >= limit || seen >= walkFileLimit {
			return filepath.SkipAll
		}
		name := d.Name()
		if d.IsDir() {
			if path != rootAbs && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		seen++
		if !strings.EqualFold(filepath.Ext(name), ".md") {
			return nil
		}
		rel := relPath(rootAbs, path)
		if strings.Contains(strings.ToLower(name), needle) {
			matches = append(matches, domain.ObsidianMatch{Path: rel})
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil //nolint:nilerr
		}
		content := string(data)
		idx := strings.Index(strings.ToLower(content), needle)
		if idx < 0 {
			return nil
		}
		runes := []rune(content[:idx])
		start := len(runes) - snippetRunes/2
		if start < 0 {
			start = 0
		}
		all := []rune(content)
		end := len(runes) + snippetRunes
		if end > len(all) {
			end = len(all)
		}
		snippet := strings.Join(strings.Fields(string(all[start:end])), " ")
		matches = append(matches, domain.ObsidianMatch{Path: rel, Snippet: snippet})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matches, nil
}

// Read returns a note's content, capped at ReadCap runes.
func Read(vaultDir, rel string) (string, error) {
	target, err := Resolve(vaultDir, rel)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return "", err
	}
	text := string(data)
	if r := []rune(text); len(r) > ReadCap {
		text = string(r[:ReadCap]) + "\n[truncated]"
	}
	return text, nil
}

// Write creates or overwrites a note. The .obsidian folder is refused and
// missing parent folders inside the vault are created.
func Write(vaultDir, rel, content string) error {
	return writeNote(vaultDir, rel, content, false)
}

// Append adds content to the end of a note, creating it when absent.
func Append(vaultDir, rel, content string) error {
	return writeNote(vaultDir, rel, content, true)
}

func writeNote(vaultDir, rel, content string, appendMode bool) error {
	if isConfigDir(filepath.ToSlash(filepath.Clean(strings.TrimSpace(rel)))) {
		return fmt.Errorf("the .obsidian configuration folder is read-only for the agent")
	}
	target, err := Resolve(vaultDir, rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if appendMode {
		existing, err := os.ReadFile(target)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		text := string(existing)
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		content = text + content
	}
	return os.WriteFile(target, []byte(content), 0o644)
}

// Delete removes a note or folder (recursively) inside the vault. The vault
// root and the .obsidian configuration folder are refused.
func Delete(vaultDir, rel string) error {
	cleaned := filepath.ToSlash(filepath.Clean(strings.TrimSpace(rel)))
	if cleaned == "" || cleaned == "." || cleaned == "/" {
		return fmt.Errorf("refusing to delete the vault root")
	}
	if isConfigDir(cleaned) {
		return fmt.Errorf("the .obsidian configuration folder is read-only for the agent")
	}
	target, err := Resolve(vaultDir, rel)
	if err != nil {
		return err
	}
	if _, err := os.Stat(target); err != nil {
		return err
	}
	return os.RemoveAll(target)
}

// AlwaysReadBlock renders the configured always-read notes as a prompt block,
// capped at AlwaysReadCap runes total. Unreadable files are annotated, never
// fatal — the block is best-effort by design.
func AlwaysReadBlock(vaultDir string, files []string) string {
	if strings.TrimSpace(vaultDir) == "" || len(files) == 0 {
		return ""
	}
	parts := make([]string, 0, len(files))
	for _, f := range files {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		text, err := readFull(vaultDir, f)
		if err != nil {
			parts = append(parts, fmt.Sprintf("### Obsidian note: %s\n[unreadable: %v]", f, err))
			continue
		}
		parts = append(parts, fmt.Sprintf("### Obsidian note: %s\n%s", f, strings.TrimSpace(text)))
	}
	if len(parts) == 0 {
		return ""
	}
	return "## Information loaded from the Obsidian module\n" +
		"Notes the user marked \"always read\":\n\n" + strings.Join(parts, "\n\n")
}

// AlwaysReadSize returns the total rune count of the always-read notes as
// they would enter the prompt (unreadable files count zero) — the module
// page uses it to warn when the block is getting heavy.
func AlwaysReadSize(vaultDir string, files []string) int {
	total := 0
	for _, f := range files {
		if f = strings.TrimSpace(f); f == "" {
			continue
		}
		if text, err := readFull(vaultDir, f); err == nil {
			total += len([]rune(text))
		}
	}
	return total
}

// readFull reads a note without the obsidian.read display cap.
func readFull(vaultDir, rel string) (string, error) {
	target, err := Resolve(vaultDir, rel)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(target)
	return string(data), err
}

// Vault is the ports.ObsidianVault adapter over this package's operations.
type Vault struct{}

func New() Vault { return Vault{} }

func (Vault) List(vaultDir, folder string) ([]domain.ObsidianEntry, error) {
	return List(vaultDir, folder)
}
func (Vault) Search(vaultDir, query string, limit int) ([]domain.ObsidianMatch, error) {
	return Search(vaultDir, query, limit)
}
func (Vault) Read(vaultDir, rel string) (string, error)  { return Read(vaultDir, rel) }
func (Vault) Write(vaultDir, rel, content string) error  { return Write(vaultDir, rel, content) }
func (Vault) Append(vaultDir, rel, content string) error { return Append(vaultDir, rel, content) }
func (Vault) Delete(vaultDir, rel string) error          { return Delete(vaultDir, rel) }
func (Vault) AlwaysReadSize(vaultDir string, files []string) int {
	return AlwaysReadSize(vaultDir, files)
}
func (Vault) AlwaysReadBlock(vaultDir string, files []string) string {
	return AlwaysReadBlock(vaultDir, files)
}
