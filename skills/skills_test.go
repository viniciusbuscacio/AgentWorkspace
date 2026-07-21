package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aw/internal/domain"
)

func findSkill(list []domain.Skill, id string) (domain.Skill, bool) {
	for _, s := range list {
		if s.ID == id {
			return s, true
		}
	}
	return domain.Skill{}, false
}

func TestBundledSkillsIncludeGmailWebWithAuxFiles(t *testing.T) {
	list, err := BundledSkills()
	if err != nil {
		t.Fatalf("BundledSkills() error = %v", err)
	}
	gmail, ok := findSkill(list, "gmail-web")
	if !ok {
		t.Fatalf("gmail-web not in bundled seed: %+v", list)
	}
	if gmail.Origin != domain.SkillOriginBuiltin || !gmail.Enabled {
		t.Fatalf("bundled skill should be builtin+enabled: %+v", gmail)
	}
	body := gmail.SkillBody()
	for _, want := range []string{"Gmail", "Do not try `gws.gmail.inbox`", "Snippet: List Visible Rows", "TARGET_ITEM_JSON"} {
		if !strings.Contains(gmail.Description+body, want) {
			t.Fatalf("gmail-web seed missing %q", want)
		}
	}
	// The walker must capture auxiliary files, not just SKILL.md — the bug the
	// old parseSkill had. gmail-web ships agents/openai.yaml.
	var hasAux bool
	for _, f := range gmail.Files {
		if f.Path == "agents/openai.yaml" {
			hasAux = true
		}
		if f.ContentHash == "" || f.SeedHash != f.ContentHash {
			t.Fatalf("seed file %q must have ContentHash==SeedHash precomputed: %+v", f.Path, f)
		}
	}
	if !hasAux {
		t.Fatalf("gmail-web seed dropped aux file agents/openai.yaml: %+v", gmail.Files)
	}
}

func TestBundledAgentsDocPresent(t *testing.T) {
	doc, ok, err := BundledAgentsDoc()
	if err != nil || !ok {
		t.Fatalf("BundledAgentsDoc() ok=%v err=%v", ok, err)
	}
	if doc.ID != domain.AgentsDocumentID || strings.TrimSpace(doc.Content) == "" {
		t.Fatalf("unexpected AGENTS.md seed: %+v", doc)
	}
	if doc.ContentHash != doc.SeedHash || doc.ContentHash == "" {
		t.Fatalf("AGENTS.md seed hashes not set: %+v", doc)
	}
}

func TestInstructionForSkillsRenders(t *testing.T) {
	list, err := BundledSkills()
	if err != nil {
		t.Fatal(err)
	}
	got := InstructionForSkills(list)
	for _, want := range []string{"## Skills", "`gmail-web`", "Use when:", "skill.read"} {
		if !strings.Contains(got, want) {
			t.Fatalf("index missing %q:\n%s", want, got)
		}
	}
	// Progressive disclosure: the index must carry only the trigger, never the body.
	if strings.Contains(got, "Do not try `gws.gmail.inbox`") {
		t.Fatalf("index must not contain the skill body:\n%s", got)
	}
	if InstructionForSkills(nil) != "" {
		t.Fatal("empty set should render empty string")
	}
}

func TestNormalizeStableAcrossLineEndingsAndBOM(t *testing.T) {
	unix := "name: x\nbody line\n"
	dos := "\ufeffname: x\r\nbody line  \r\n\r\n"
	if Normalize(unix) != Normalize(dos) {
		t.Fatalf("normalize not stable: %q vs %q", Normalize(unix), Normalize(dos))
	}
	if Hash(unix) != Hash(dos) {
		t.Fatal("hash not stable across CRLF/BOM — cross-OS would flag false customization")
	}
}

func TestDevCatalogReadsOverrideDir(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "my-skill")
	if err := os.MkdirAll(filepath.Join(skillDir, "refs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: my-skill\ndescription: Dev only.\n---\n\n# Body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "refs", "a.md"), []byte("ref"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, domain.AgentsDocumentID), []byte("# Dev AGENTS"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv(DevSkillsDirEnv, root)
	list, agents, hasAgents, gotRoot, active := DevCatalog()
	if !active || gotRoot != root {
		t.Fatalf("override should be active at %q; got active=%v root=%q", root, active, gotRoot)
	}
	skill, ok := findSkill(list, "my-skill")
	if !ok {
		t.Fatalf("dev skill not loaded: %+v", list)
	}
	if skill.Origin != domain.SkillOriginUser {
		t.Fatalf("dev skills must be origin=user: %+v", skill)
	}
	if len(skill.Files) != 2 {
		t.Fatalf("dev walker must capture all files, got %+v", skill.Files)
	}
	if !hasAgents || !strings.Contains(agents.Content, "Dev AGENTS") {
		t.Fatalf("dev AGENTS.md not loaded: ok=%v %+v", hasAgents, agents)
	}
}

func TestDevCatalogInactiveWhenUnset(t *testing.T) {
	t.Setenv(DevSkillsDirEnv, "")
	if _, _, _, _, active := DevCatalog(); active {
		t.Fatal("override must be inactive when AW_SKILLS_DIR is empty")
	}
}

func writeSkillFolder(t *testing.T, dir, name, description string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "refs"), 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: " + name + "\ndescription: " + description + "\n---\n\n# Body\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "refs", "a.md"), []byte("ref"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadSkillFromMarkdown(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "my-skill")
	writeSkillFolder(t, skillDir, "My Skill", "Do something useful.")

	skill, err := LoadSkillFromMarkdown(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("LoadSkillFromMarkdown() error = %v", err)
	}
	if skill.ID != "my-skill" {
		t.Fatalf("id should be the folder name, got %q", skill.ID)
	}
	if skill.Name != "My Skill" || skill.Description != "Do something useful." {
		t.Fatalf("name/description should come from frontmatter: %+v", skill)
	}
	if len(skill.Files) != 2 {
		t.Fatalf("auxiliary files should be imported, got %+v", skill.Files)
	}

	// Rejects a non-SKILL.md path (case-sensitive).
	if _, err := LoadSkillFromMarkdown(filepath.Join(skillDir, "refs", "a.md")); err == nil {
		t.Fatal("non-SKILL.md path should be rejected")
	}
	if _, err := LoadSkillFromMarkdown(filepath.Join(skillDir, "skill.md")); err == nil {
		t.Fatal("lowercase skill.md should be rejected")
	}
}

func TestLoadSkillsFromParent(t *testing.T) {
	root := t.TempDir()
	writeSkillFolder(t, filepath.Join(root, "alpha"), "Alpha", "a")
	writeSkillFolder(t, filepath.Join(root, "beta"), "Beta", "b")
	// A direct subfolder without SKILL.md -> skipped.
	if err := os.MkdirAll(filepath.Join(root, "garbage"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A loose file in root -> ignored.
	if err := os.WriteFile(filepath.Join(root, "README.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A nested SKILL.md under a valid skill must NOT be picked up (non-recursive).
	if err := os.WriteFile(filepath.Join(root, "alpha", "refs", "SKILL.md"), []byte("---\nname: nested\ndescription: n\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, skipped, err := LoadSkillsFromParent(root)
	if err != nil {
		t.Fatalf("LoadSkillsFromParent() error = %v", err)
	}
	if len(loaded) != 2 || loaded[0].ID != "alpha" || loaded[1].ID != "beta" {
		t.Fatalf("should load the 2 valid skills, got %+v", loaded)
	}
	if len(skipped) != 1 || skipped[0] != "garbage" {
		t.Fatalf("garbage should be skipped, got %+v", skipped)
	}

	if _, _, err := LoadSkillsFromParent(filepath.Join(root, "does-not-exist")); err == nil {
		t.Fatal("missing root should be a fatal error")
	}
}

func TestParseSkillMetaBlockScalars(t *testing.T) {
	cases := []struct {
		name    string
		content string
		wantN   string
		wantD   string
	}{
		{
			name:    "single-line quoted with colon",
			content: "---\nname: gmail-web\ndescription: \"Manage Gmail: inbox, search\"\n---\n\nbody",
			wantN:   "gmail-web",
			wantD:   "Manage Gmail: inbox, search",
		},
		{
			name:    "folded >",
			content: "---\nname: a\ndescription: >\n  Executa um goal\n  complexo agora.\n---\n",
			wantN:   "a",
			wantD:   "Executa um goal complexo agora.",
		},
		{
			name:    "strip-folded >-",
			content: "---\nname: b\ndescription: >-\n  Builda, roda\n  e usa o AW3 → fim.\n---\n",
			wantN:   "b",
			wantD:   "Builda, roda e usa o AW3 → fim.",
		},
		{
			name:    "literal |",
			content: "---\nname: c\ndescription: |\n  Remove signs\n  of AI writing.\n---\n",
			wantN:   "c",
			wantD:   "Remove signs of AI writing.",
		},
		{
			name:    "missing description",
			content: "---\nname: d\n---\n",
			wantN:   "d",
			wantD:   "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n, d := ParseSkillMeta(tc.content)
			if n != tc.wantN || d != tc.wantD {
				t.Fatalf("ParseSkillMeta() = (%q, %q), want (%q, %q)", n, d, tc.wantN, tc.wantD)
			}
		})
	}
}

func TestParseSkillMetaInvalidFrontmatter(t *testing.T) {
	if n, d := ParseSkillMeta("# no frontmatter\n\nbody"); n != "" || d != "" {
		t.Fatalf("no frontmatter should yield empty, got (%q,%q)", n, d)
	}
	// Unterminated frontmatter.
	if n, _ := ParseSkillMeta("---\nname: x\nno closing fence"); n != "" {
		t.Fatalf("unterminated frontmatter should yield empty name, got %q", n)
	}
}

func TestSplitFrontmatterIgnoresBodyDashes(t *testing.T) {
	content := "---\nname: x\ndescription: y\n---\n\n# Title\n\nsome --- text\n\n---\n\nmore"
	_, body, ok := splitFrontmatter(content)
	if !ok {
		t.Fatal("should detect frontmatter")
	}
	if !strings.Contains(body, "some --- text") || !strings.Contains(body, "more") {
		t.Fatalf("body should keep the --- inside it: %q", body)
	}
	n, _ := ParseSkillMeta(content)
	if n != "x" {
		t.Fatalf("name should parse despite --- in body, got %q", n)
	}
}

func TestParseSkillMetaUTF8RoundTrips(t *testing.T) {
	content := "---\nname: café\ndescription: \"Acentuação, símbolos → — e emoji 🎉\"\n---\n"
	n, d := ParseSkillMeta(content)
	if n != "café" || d != "Acentuação, símbolos → — e emoji 🎉" {
		t.Fatalf("UTF-8 not preserved: (%q, %q)", n, d)
	}
}

func TestDirHasSkillMarkdown(t *testing.T) {
	root := t.TempDir()
	writeSkillFolder(t, filepath.Join(root, "single"), "Single", "s")
	if !DirHasSkillMarkdown(filepath.Join(root, "single")) {
		t.Fatal("folder with SKILL.md should report true")
	}
	if DirHasSkillMarkdown(root) {
		t.Fatal("parent without SKILL.md should report false")
	}
}
