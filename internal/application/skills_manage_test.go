package application

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"aw/internal/domain"
)

func writeDiskSkill(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: " + name + "\ndescription: desc\n---\n\n# Body\n"
	if err := os.WriteFile(filepath.Join(dir, domain.SkillMarkdownPath), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestImportSkillFromFile(t *testing.T) {
	store := newFakeSkillStore()
	root := t.TempDir()
	skillDir := filepath.Join(root, "my-skill")
	writeDiskSkill(t, skillDir, "My Skill")
	mdPath := filepath.Join(skillDir, domain.SkillMarkdownPath)

	if err := ImportSkillFromFile(store, mdPath); err != nil {
		t.Fatalf("ImportSkillFromFile() error = %v", err)
	}
	if s, ok := store.skills["my-skill"]; !ok || s.Origin != domain.SkillOriginUser || !s.Enabled {
		t.Fatalf("imported skill should be user+enabled: %+v", s)
	}
	// Duplicate id is rejected.
	if err := ImportSkillFromFile(store, mdPath); err == nil {
		t.Fatal("re-importing same id should error")
	}
	// Non-SKILL.md path rejected.
	if err := ImportSkillFromFile(store, filepath.Join(skillDir, "Body.md")); err == nil {
		t.Fatal("non-SKILL.md path should error")
	}
	// Locked vault.
	store.unlocked = false
	if err := ImportSkillFromFile(store, mdPath); err == nil {
		t.Fatal("locked vault should error")
	}
}

func TestImportSkillsFromParentPartial(t *testing.T) {
	store := newFakeSkillStore()
	// A pre-existing skill to force a duplicate skip.
	store.skills["beta"] = domain.Skill{ID: "beta", Origin: domain.SkillOriginUser, Files: mdFiles("x")}

	root := t.TempDir()
	writeDiskSkill(t, filepath.Join(root, "alpha"), "Alpha")
	writeDiskSkill(t, filepath.Join(root, "beta"), "Beta") // duplicate
	if err := os.MkdirAll(filepath.Join(root, "garbage"), 0o755); err != nil {
		t.Fatal(err)
	}

	summary, err := ImportSkillsFromParent(store, root)
	if err != nil {
		t.Fatalf("ImportSkillsFromParent() error = %v", err)
	}
	if len(summary.Imported) != 1 || summary.Imported[0] != "alpha" {
		t.Fatalf("only alpha should import, got %+v", summary.Imported)
	}
	reasons := map[string]string{}
	for _, s := range summary.Skipped {
		reasons[s.ID] = s.Reason
	}
	if reasons["beta"] != "already exists" {
		t.Fatalf("beta should be skipped as duplicate, got %+v", summary.Skipped)
	}
	if reasons["garbage"] != "no SKILL.md found" {
		t.Fatalf("garbage should be skipped (no SKILL.md), got %+v", summary.Skipped)
	}
}

func TestImportSkillsFromParentStrictWhenRootIsSkill(t *testing.T) {
	store := newFakeSkillStore()
	root := t.TempDir()
	writeDiskSkill(t, root, "Single") // SKILL.md at the root itself

	if _, err := ImportSkillsFromParent(store, root); err == nil {
		t.Fatal("a root that is itself a skill should error (use Import Skill)")
	}
	if len(store.skills) != 0 {
		t.Fatal("strict mode must not import anything")
	}
}

func TestImportSkillsFromParentFatalOnMissingRoot(t *testing.T) {
	store := newFakeSkillStore()
	if _, err := ImportSkillsFromParent(store, filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("missing root should be a fatal error")
	}
}

func createMd(name, desc string) []domain.SkillFile {
	return []domain.SkillFile{{Path: domain.SkillMarkdownPath, Content: "---\nname: " + name + "\ndescription: " + desc + "\n---\n\n# Body\n"}}
}

func TestCreateSkillFromFiles(t *testing.T) {
	store := newFakeSkillStore()
	if err := CreateSkill(store, "my-skill", "", "", true, createMd("my-skill", "Use when X")); err != nil {
		t.Fatalf("CreateSkill() error = %v", err)
	}
	s := store.skills["my-skill"]
	if s.Origin != domain.SkillOriginUser || !s.Enabled {
		t.Fatalf("created skill should be user+enabled: %+v", s)
	}
	if s.Name != "my-skill" || s.Description != "Use when X" {
		t.Fatalf("name/description should come from frontmatter: %+v", s)
	}
	// Duplicate id rejected.
	if err := CreateSkill(store, "my-skill", "", "", true, createMd("my-skill", "x")); err == nil {
		t.Fatal("duplicate id should error")
	}
}

func TestCreateSkillDisabled(t *testing.T) {
	store := newFakeSkillStore()
	if err := CreateSkill(store, "disabled-skill", "", "", false, createMd("disabled-skill", "x")); err != nil {
		t.Fatal(err)
	}
	if store.skills["disabled-skill"].Enabled {
		t.Fatal("enabled=false must be preserved, not silently enabled")
	}
}

func TestCreateSkillRejectsDivergentMeta(t *testing.T) {
	store := newFakeSkillStore()
	err := CreateSkill(store, "x", "Different Name", "", true, createMd("x", "desc"))
	if err == nil {
		t.Fatal("explicit name diverging from frontmatter should error")
	}
	err = CreateSkill(store, "y", "", "Different desc", true, createMd("y", "real desc"))
	if err == nil {
		t.Fatal("explicit description diverging from frontmatter should error")
	}
	// Matching explicit values are accepted.
	if err := CreateSkill(store, "z", "z", "real", true, createMd("z", "real")); err != nil {
		t.Fatalf("matching explicit meta should be accepted: %v", err)
	}
}

func TestCreateSkillRejectsInvalidID(t *testing.T) {
	store := newFakeSkillStore()
	for _, bad := range []string{"My Skill", "my_skill", "my.skill", "-lead", "UPPER"} {
		if err := CreateSkill(store, bad, "", "", true, createMd(bad, "x")); err == nil {
			t.Fatalf("id %q should be rejected by the v1 pattern", bad)
		}
	}
	// Valid ids pass the pattern check.
	for _, ok := range []string{"my-skill", "gmail2", "aw3-dev"} {
		if err := CreateSkill(store, ok, "", "", true, createMd(ok, "x")); err != nil {
			t.Fatalf("id %q should be valid: %v", ok, err)
		}
	}
}

func TestCreateSkillRequiresMarkdown(t *testing.T) {
	store := newFakeSkillStore()
	if err := CreateSkill(store, "x", "", "", true, []domain.SkillFile{{Path: "refs/a.md", Content: "x"}}); err == nil {
		t.Fatal("create without SKILL.md should error")
	}
}

func TestSkillFilePathValidation(t *testing.T) {
	store := newFakeSkillStore()
	bad := [][]domain.SkillFile{
		{{Path: domain.SkillMarkdownPath, Content: "a"}, {Path: "../escape.md", Content: "x"}},
		{{Path: domain.SkillMarkdownPath, Content: "a"}, {Path: "/etc/passwd", Content: "x"}},
		{{Path: domain.SkillMarkdownPath, Content: "a"}, {Path: "C:\\abs.md", Content: "x"}},
		{{Path: domain.SkillMarkdownPath, Content: "a"}, {Path: "", Content: "x"}},
		{{Path: domain.SkillMarkdownPath, Content: "a"}, {Path: domain.SkillMarkdownPath, Content: "dup"}},
	}
	for i, files := range bad {
		if err := CreateSkill(store, fmt.Sprintf("s%d", i), "", "", true, files); err == nil {
			t.Fatalf("case %d: invalid file path set should be rejected", i)
		}
	}
}

func mdFiles(content string) []domain.SkillFile {
	h := hashFor(content)
	return []domain.SkillFile{{Path: domain.SkillMarkdownPath, Content: content, ContentHash: h, SeedHash: h}}
}

func TestSaveSkillFilesMarksBuiltinCustomized(t *testing.T) {
	store := newFakeSkillStore()
	store.skills["gmail-web"] = domain.Skill{
		ID: "gmail-web", Name: "gmail-web", Enabled: true, Origin: domain.SkillOriginBuiltin,
		Files: mdFiles("---\nname: gmail-web\ndescription: orig\n---\n\nORIG"),
	}

	err := SaveSkillFiles(store, "gmail-web", []domain.SkillFile{
		{Path: domain.SkillMarkdownPath, Content: "---\nname: gmail-web\ndescription: edited\n---\n\nEDITED"},
	})
	if err != nil {
		t.Fatalf("SaveSkillFiles() error = %v", err)
	}
	s := store.skills["gmail-web"]
	if s.Origin != domain.SkillOriginBuiltin {
		t.Fatalf("edited builtin must stay builtin: %+v", s)
	}
	md, _ := skillMdFile(s)
	if md.ContentHash == md.SeedHash {
		t.Fatal("edited builtin file should read as customized (content_hash != seed_hash)")
	}
	if s.Description != "edited" {
		t.Fatalf("name/description should refresh from edited SKILL.md, got %q", s.Description)
	}
}

func TestSaveSkillFilesRequiresMarkdown(t *testing.T) {
	store := newFakeSkillStore()
	store.skills["x"] = domain.Skill{ID: "x", Origin: domain.SkillOriginUser, Files: mdFiles("a")}
	if err := SaveSkillFiles(store, "x", []domain.SkillFile{{Path: "refs/a.md", Content: "no md"}}); err == nil {
		t.Fatal("saving without SKILL.md should error")
	}
}

func TestDeleteSkillSoftForBuiltinHardForUser(t *testing.T) {
	store := newFakeSkillStore()
	store.skills["builtin"] = domain.Skill{ID: "builtin", Origin: domain.SkillOriginBuiltin, Files: mdFiles("a")}
	store.skills["user"] = domain.Skill{ID: "user", Origin: domain.SkillOriginUser, Files: mdFiles("b")}

	if err := DeleteSkill(store, "builtin"); err != nil {
		t.Fatal(err)
	}
	if s := store.skills["builtin"]; !s.Deleted {
		t.Fatal("builtin should be soft-deleted, not removed")
	}
	if err := DeleteSkill(store, "user"); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.skills["user"]; ok {
		t.Fatal("user skill should be hard-deleted")
	}
}

func TestResetSkillToSeed(t *testing.T) {
	store := newFakeSkillStore()
	seeds := []domain.Skill{{
		ID: "gmail-web", Name: "gmail-web", Origin: domain.SkillOriginBuiltin, SeedVersion: "1",
		Files: mdFiles("SEED"),
	}}
	// Customized + soft-deleted current state.
	store.skills["gmail-web"] = domain.Skill{
		ID: "gmail-web", Origin: domain.SkillOriginBuiltin, Deleted: true,
		Files: []domain.SkillFile{{Path: domain.SkillMarkdownPath, Content: "EDIT", ContentHash: hashFor("EDIT"), SeedHash: hashFor("SEED")}},
	}

	if err := ResetSkillToSeed(store, seeds, "gmail-web"); err != nil {
		t.Fatalf("ResetSkillToSeed() error = %v", err)
	}
	s := store.skills["gmail-web"]
	if s.Deleted || !s.Enabled {
		t.Fatalf("reset should restore + enable: %+v", s)
	}
	md, _ := skillMdFile(s)
	if md.Content != "SEED" || md.ContentHash != md.SeedHash {
		t.Fatalf("reset should restore seed content and clear customization: %+v", md)
	}

	if err := ResetSkillToSeed(store, seeds, "unknown"); err == nil {
		t.Fatal("reset with no bundled seed should error")
	}
}

func TestListSkillViewsFlags(t *testing.T) {
	store := newFakeSkillStore()
	// Customized builtin whose seed advanced -> updateAvailable.
	store.skills["gmail-web"] = domain.Skill{
		ID: "gmail-web", Name: "gmail-web", Origin: domain.SkillOriginBuiltin, Enabled: true,
		Files: []domain.SkillFile{{Path: domain.SkillMarkdownPath, Content: "EDIT", ContentHash: hashFor("EDIT"), SeedHash: hashFor("V1")}},
	}
	store.skills["web-read"] = domain.Skill{
		ID: "web-read", Origin: domain.SkillOriginBuiltin, Enabled: true,
		Files: mdFiles("CLEAN"), // content_hash == seed_hash -> not customized
	}
	seeds := []domain.Skill{
		{ID: "gmail-web", Files: mdFiles("V2")}, // seed hash != stored seed_hash(V1) -> update available
		{ID: "web-read", Files: mdFiles("CLEAN")},
	}

	views, err := ListSkillViews(store, seeds)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]SkillView{}
	for _, v := range views {
		byID[v.ID] = v
	}
	if !byID["gmail-web"].Customized || !byID["gmail-web"].UpdateAvailable {
		t.Fatalf("gmail-web should be customized + updateAvailable: %+v", byID["gmail-web"])
	}
	if byID["web-read"].Customized {
		t.Fatalf("web-read should not be customized: %+v", byID["web-read"])
	}
}
