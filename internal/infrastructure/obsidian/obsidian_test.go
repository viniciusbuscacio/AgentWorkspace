package obsidian

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newVault(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, dir, "Home.md", "# Home\nWelcome to the vault. flamengo")
	mustWrite(t, dir, "Projects/aw.md", "# aw\nAgent Workspace project notes")
	mustWrite(t, dir, ".obsidian/app.json", "{}")
	return dir
}

func mustWrite(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveRejectsEscapes(t *testing.T) {
	dir := newVault(t)
	for _, rel := range []string{"../outside.md", "a/../../outside.md", "../../etc/passwd"} {
		if _, err := Resolve(dir, rel); err == nil {
			t.Fatalf("Resolve(%q) should fail", rel)
		}
	}
	if _, err := Resolve(dir, "Projects/aw.md"); err != nil {
		t.Fatalf("valid path rejected: %v", err)
	}
	if _, err := Resolve("", "x.md"); err == nil {
		t.Fatal("empty vault dir must error")
	}
}

func TestListHidesDotfolders(t *testing.T) {
	dir := newVault(t)
	entries, err := List(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Path, ".obsidian") {
			t.Fatalf(".obsidian leaked into list: %+v", entries)
		}
	}
	if len(entries) != 2 { // Projects dir + Home.md
		t.Fatalf("entries = %+v, want 2", entries)
	}
	if !entries[0].IsDir {
		t.Fatalf("folders must sort first: %+v", entries)
	}
}

func TestSearchByNameAndContent(t *testing.T) {
	dir := newVault(t)
	byName, err := Search(dir, "home", 10)
	if err != nil || len(byName) != 1 || byName[0].Path != "Home.md" {
		t.Fatalf("byName = %+v, err=%v", byName, err)
	}
	byContent, err := Search(dir, "workspace project", 10)
	if err != nil || len(byContent) != 1 || byContent[0].Path != "Projects/aw.md" {
		t.Fatalf("byContent = %+v, err=%v", byContent, err)
	}
	if byContent[0].Snippet == "" {
		t.Fatal("content match must carry a snippet")
	}
}

func TestReadCapsLongNotes(t *testing.T) {
	dir := newVault(t)
	mustWrite(t, dir, "big.md", strings.Repeat("a", ReadCap+500))
	text, err := Read(dir, "big.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(text, "[truncated]") {
		t.Fatal("long note must be truncated")
	}
}

func TestWriteAndAppend(t *testing.T) {
	dir := newVault(t)
	if err := Write(dir, "New/idea.md", "first"); err != nil {
		t.Fatal(err)
	}
	if err := Append(dir, "New/idea.md", "second"); err != nil {
		t.Fatal(err)
	}
	text, err := Read(dir, "New/idea.md")
	if err != nil || text != "first\nsecond" {
		t.Fatalf("text = %q, err=%v", text, err)
	}
	if err := Write(dir, ".obsidian/app.json", "{}"); err == nil {
		t.Fatal("writes into .obsidian must be refused")
	}
	if err := Write(dir, "../escape.md", "x"); err == nil {
		t.Fatal("write outside the vault must be refused")
	}
}

func TestAlwaysReadBlock(t *testing.T) {
	dir := newVault(t)
	block := AlwaysReadBlock(dir, []string{"Home.md", "missing.md"})
	if !strings.Contains(block, "### Obsidian note: Home.md") || !strings.Contains(block, "Welcome to the vault") {
		t.Fatalf("block missing content: %s", block)
	}
	if !strings.Contains(block, "[unreadable") {
		t.Fatal("missing files must be annotated, not fatal")
	}
	if AlwaysReadBlock(dir, nil) != "" || AlwaysReadBlock("", []string{"Home.md"}) != "" {
		t.Fatal("empty config must produce no block")
	}
}

func TestDeleteJail(t *testing.T) {
	dir := newVault(t)
	if err := Delete(dir, "Projects/aw.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Projects", "aw.md")); !os.IsNotExist(err) {
		t.Fatal("note should be gone")
	}
	for _, rel := range []string{"", ".", "/", ".obsidian", ".obsidian/app.json", "../outside"} {
		if err := Delete(dir, rel); err == nil {
			t.Fatalf("Delete(%q) must be refused", rel)
		}
	}
}

func TestAlwaysReadBlockInjectsNotesInFull(t *testing.T) {
	dir := newVault(t)
	big := strings.Repeat("a", 30000)
	mustWrite(t, dir, "big.md", big)
	mustWrite(t, dir, "small.md", "conteudo pequeno importante")
	block := AlwaysReadBlock(dir, []string{"big.md", "small.md"})
	if !strings.Contains(block, big) {
		t.Fatal("notes must be injected in full — no cap (user decision)")
	}
	if !strings.Contains(block, "conteudo pequeno importante") {
		t.Fatal("second note must be present in full")
	}
	if got := AlwaysReadSize(dir, []string{"big.md", "small.md"}); got < 30000 {
		t.Fatalf("AlwaysReadSize = %d, want >= 30000", got)
	}
}
