package application

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"aw/internal/domain"
)

func sha256hex(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func docSeed(id, content string) domain.AppDocument {
	h := hashFor(content)
	return domain.AppDocument{ID: id, Content: content, ContentHash: h, SeedHash: h, Origin: domain.SkillOriginBuiltin, SeedVersion: "1"}
}

// fakeSkillStore is an in-memory ports.SkillStore for the update-safe tests.
type fakeSkillStore struct {
	unlocked bool
	skills   map[string]domain.Skill
	docs     map[string]domain.AppDocument
}

func newFakeSkillStore() *fakeSkillStore {
	return &fakeSkillStore{unlocked: true, skills: map[string]domain.Skill{}, docs: map[string]domain.AppDocument{}}
}

func (f *fakeSkillStore) IsUnlocked() bool { return f.unlocked }

func (f *fakeSkillStore) ListSkills(includeDeleted bool) ([]domain.Skill, error) {
	var out []domain.Skill
	for _, s := range f.skills {
		if !includeDeleted && s.Deleted {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

func (f *fakeSkillStore) GetSkill(id string) (domain.Skill, bool, error) {
	s, ok := f.skills[id]
	return s, ok, nil
}

func (f *fakeSkillStore) UpsertSkill(skill domain.Skill) error {
	f.skills[skill.ID] = skill
	return nil
}

func (f *fakeSkillStore) SetSkillEnabled(id string, enabled bool) error {
	s := f.skills[id]
	s.Enabled = enabled
	f.skills[id] = s
	return nil
}

func (f *fakeSkillStore) SoftDeleteSkill(id string) error {
	s := f.skills[id]
	s.Deleted = true
	s.Enabled = false
	f.skills[id] = s
	return nil
}

func (f *fakeSkillStore) DeleteSkill(id string) error {
	delete(f.skills, id)
	return nil
}

func (f *fakeSkillStore) GetAppDocument(id string) (domain.AppDocument, bool, error) {
	d, ok := f.docs[id]
	return d, ok, nil
}

func (f *fakeSkillStore) ListAppDocuments() ([]domain.AppDocument, error) {
	out := make([]domain.AppDocument, 0, len(f.docs))
	for _, d := range f.docs {
		out = append(out, d)
	}
	return out, nil
}

func (f *fakeSkillStore) UpsertAppDocument(doc domain.AppDocument) error {
	f.docs[doc.ID] = doc
	return nil
}

// seedSkill builds a one-file builtin seed (SKILL.md) with hashes set.
func seedSkill(id, content string) domain.Skill {
	h := hashFor(content)
	return domain.Skill{
		ID: id, Name: id, Description: "d", Enabled: true,
		Origin: domain.SkillOriginBuiltin, SeedVersion: "1",
		Files: []domain.SkillFile{{Path: domain.SkillMarkdownPath, Content: content, ContentHash: h, SeedHash: h}},
	}
}

// hashFor mirrors skills.Hash for the SKILL.md content in tests without
// importing the skills package (its hashing normalizes; these inputs are
// already normalized).
func hashFor(content string) string {
	return sha256hex(content)
}

func TestBootstrapSeedsEmptyVault(t *testing.T) {
	store := newFakeSkillStore()
	seeds := []domain.Skill{seedSkill("gmail-web", "a"), seedSkill("web-read", "b")}
	doc := docSeed("AGENTS.md", "runtime guide")

	if err := BootstrapSkills(store, seeds, doc, true); err != nil {
		t.Fatalf("BootstrapSkills() error = %v", err)
	}
	if len(store.skills) != 2 {
		t.Fatalf("expected 2 seeded skills, got %d", len(store.skills))
	}
	if _, ok := store.docs["AGENTS.md"]; !ok {
		t.Fatal("AGENTS.md not seeded")
	}
	for _, id := range []string{"gmail-web", "web-read"} {
		s := store.skills[id]
		if s.Origin != domain.SkillOriginBuiltin || !s.Enabled {
			t.Fatalf("seeded skill %s should be builtin+enabled: %+v", id, s)
		}
	}
}

func TestBootstrapUpdatesUnmodifiedBuiltin(t *testing.T) {
	store := newFakeSkillStore()
	_ = BootstrapSkills(store, []domain.Skill{seedSkill("gmail-web", "v1")}, domain.AppDocument{}, false)

	// New seed content, user never touched it -> should be replaced.
	_ = BootstrapSkills(store, []domain.Skill{seedSkill("gmail-web", "v2")}, domain.AppDocument{}, false)

	got := store.skills["gmail-web"].SkillBody()
	if got != "v2" {
		t.Fatalf("unmodified builtin should update to v2, got %q", got)
	}
}

func TestBootstrapPreservesUserModifiedBuiltin(t *testing.T) {
	store := newFakeSkillStore()
	_ = BootstrapSkills(store, []domain.Skill{seedSkill("gmail-web", "v1")}, domain.AppDocument{}, false)

	// Simulate a user edit: content changes, seed_hash stays the original.
	s := store.skills["gmail-web"]
	s.Files[0].Content = "user edited"
	s.Files[0].ContentHash = hashFor("user edited") // != SeedHash
	store.skills["gmail-web"] = s

	_ = BootstrapSkills(store, []domain.Skill{seedSkill("gmail-web", "v2")}, domain.AppDocument{}, false)

	if body := store.skills["gmail-web"].SkillBody(); body != "user edited" {
		t.Fatalf("user-modified builtin must be preserved, got %q", body)
	}
}

func TestBootstrapNeverTouchesUserSkill(t *testing.T) {
	store := newFakeSkillStore()
	store.skills["mine"] = domain.Skill{
		ID: "mine", Name: "mine", Origin: domain.SkillOriginUser, Enabled: true,
		Files: []domain.SkillFile{{Path: domain.SkillMarkdownPath, Content: "user content", ContentHash: hashFor("user content")}},
	}
	// A seed with the same id must be blocked from overriding a user skill.
	_ = BootstrapSkills(store, []domain.Skill{seedSkill("mine", "seed content")}, domain.AppDocument{}, false)

	if body := store.skills["mine"].SkillBody(); body != "user content" {
		t.Fatalf("user skill must never be touched, got %q", body)
	}
}

func TestBootstrapDoesNotResurrectSoftDeletedBuiltin(t *testing.T) {
	store := newFakeSkillStore()
	_ = BootstrapSkills(store, []domain.Skill{seedSkill("gmail-web", "v1")}, domain.AppDocument{}, false)
	_ = store.SoftDeleteSkill("gmail-web")

	_ = BootstrapSkills(store, []domain.Skill{seedSkill("gmail-web", "v2")}, domain.AppDocument{}, false)

	s := store.skills["gmail-web"]
	if !s.Deleted {
		t.Fatal("soft-deleted builtin must not be resurrected by bootstrap")
	}
	if s.SkillBody() == "v2" {
		t.Fatal("soft-deleted builtin content must not be updated")
	}
}

func TestBootstrapIsIdempotent(t *testing.T) {
	store := newFakeSkillStore()
	seeds := []domain.Skill{seedSkill("gmail-web", "v1")}
	_ = BootstrapSkills(store, seeds, domain.AppDocument{}, false)
	before := store.skills["gmail-web"]
	_ = BootstrapSkills(store, seeds, domain.AppDocument{}, false)
	after := store.skills["gmail-web"]
	if before.SkillBody() != after.SkillBody() || before.SeedVersion != after.SeedVersion {
		t.Fatal("second bootstrap with same seed must be a no-op")
	}
}

func TestBootstrapPreservesUserEditedAgentsDoc(t *testing.T) {
	store := newFakeSkillStore()
	_ = BootstrapSkills(store, nil, docSeed("AGENTS.md", "v1"), true)

	d := store.docs["AGENTS.md"]
	d.Content = "user edited agents"
	d.ContentHash = hashFor("user edited agents") // != SeedHash
	store.docs["AGENTS.md"] = d

	_ = BootstrapSkills(store, nil, docSeed("AGENTS.md", "v2"), true)
	if store.docs["AGENTS.md"].Content != "user edited agents" {
		t.Fatal("user-edited AGENTS.md must be preserved")
	}
}

func TestEffectiveSkillsFiltersAndOverride(t *testing.T) {
	store := newFakeSkillStore()
	store.skills["on"] = domain.Skill{ID: "on", Enabled: true}
	store.skills["off"] = domain.Skill{ID: "off", Enabled: false}
	store.skills["del"] = domain.Skill{ID: "del", Enabled: true, Deleted: true}

	list, source := EffectiveSkills(store, SkillsPromptInput{})
	if source != "vault" || len(list) != 1 || list[0].ID != "on" {
		t.Fatalf("vault effective skills wrong: %s %+v", source, list)
	}

	dev := SkillsPromptInput{Active: true, Skills: []domain.Skill{{ID: "dev"}}}
	list, source = EffectiveSkills(store, dev)
	if source != "dev-override" || len(list) != 1 || list[0].ID != "dev" {
		t.Fatalf("dev override should win: %s %+v", source, list)
	}

	store.unlocked = false
	if _, source := EffectiveSkills(store, SkillsPromptInput{}); source != "locked" {
		t.Fatalf("locked store should report locked, got %s", source)
	}
}

func TestSkillCatalogFlagsStaleCustomizedBuiltin(t *testing.T) {
	store := newFakeSkillStore()
	// Customized builtin (content diverged from its seed) whose bundle moved on.
	store.skills["google-workspace"] = domain.Skill{
		ID: "google-workspace", Name: "google-workspace", Enabled: true, Origin: domain.SkillOriginBuiltin,
		Files: []domain.SkillFile{
			{Path: domain.SkillMarkdownPath, Content: "edited", ContentHash: "h-edited", SeedHash: "h-v1"},
		},
	}
	// Unmodified builtin: no flags even though the bundle moved on (the
	// bootstrap safe-update path handles it).
	store.skills["gmail-web"] = domain.Skill{
		ID: "gmail-web", Name: "gmail-web", Enabled: true, Origin: domain.SkillOriginBuiltin,
		Files: []domain.SkillFile{
			{Path: domain.SkillMarkdownPath, Content: "same", ContentHash: "h-v1", SeedHash: "h-v1"},
		},
	}
	seeds := []domain.Skill{
		{ID: "google-workspace", Files: []domain.SkillFile{{Path: domain.SkillMarkdownPath, ContentHash: "h-v2"}}},
		{ID: "gmail-web", Files: []domain.SkillFile{{Path: domain.SkillMarkdownPath, ContentHash: "h-v2"}}},
	}

	cat := SkillCatalog(store, SkillsPromptInput{}, seeds)
	byID := map[string]SkillSummary{}
	for _, s := range cat {
		byID[s.ID] = s
	}
	gws := byID["google-workspace"]
	if !gws.Customized || !gws.UpdateAvailable {
		t.Fatalf("customized stale builtin should flag customized+updateAvailable: %+v", gws)
	}
	if !strings.Contains(gws.Guidance, "skill.reset") {
		t.Fatalf("guidance should point at skill.reset: %q", gws.Guidance)
	}
	web := byID["gmail-web"]
	if web.Customized || web.UpdateAvailable || web.Guidance != "" {
		t.Fatalf("unmodified builtin should carry no staleness flags: %+v", web)
	}
}

func TestSkillCatalogAndReadFile(t *testing.T) {
	store := newFakeSkillStore()
	store.skills["gmail-web"] = domain.Skill{
		ID: "gmail-web", Name: "gmail-web", Description: "operar Gmail", Enabled: true, Origin: domain.SkillOriginBuiltin,
		Files: []domain.SkillFile{
			{Path: domain.SkillMarkdownPath, Content: "FULL BODY"},
			{Path: "agents/openai.yaml", Content: "yaml content"},
		},
	}
	store.skills["off"] = domain.Skill{ID: "off", Name: "off", Enabled: false}

	cat := SkillCatalog(store, SkillsPromptInput{}, nil)
	if len(cat) != 1 || cat[0].ID != "gmail-web" {
		t.Fatalf("catalog should list only enabled skills: %+v", cat)
	}
	if len(cat[0].Files) != 2 {
		t.Fatalf("catalog entry should list file paths: %+v", cat[0])
	}

	// Default path -> SKILL.md body (level 2).
	body, err := ReadSkillFile(store, SkillsPromptInput{}, "gmail-web", "")
	if err != nil || body != "FULL BODY" {
		t.Fatalf("ReadSkillFile SKILL.md = %q, %v", body, err)
	}
	// Auxiliary file (level 3).
	aux, err := ReadSkillFile(store, SkillsPromptInput{}, "gmail-web", "agents/openai.yaml")
	if err != nil || aux != "yaml content" {
		t.Fatalf("ReadSkillFile aux = %q, %v", aux, err)
	}
	// Disabled / unknown skills are not readable.
	if _, err := ReadSkillFile(store, SkillsPromptInput{}, "off", ""); err == nil {
		t.Fatal("disabled skill should not be readable")
	}
	if _, err := ReadSkillFile(store, SkillsPromptInput{}, "nope", ""); err == nil {
		t.Fatal("unknown skill should error")
	}
}

type fakeSkillsSetter struct{ got string }

func (f *fakeSkillsSetter) SetSkillsContext(extra string) { f.got = extra }

func TestRefreshSkillsContextComposesAndClears(t *testing.T) {
	store := newFakeSkillStore()
	store.skills["gmail-web"] = seedSkill("gmail-web", "---\nname: gmail-web\ndescription: x\n---\n\nBODY")
	store.docs["AGENTS.md"] = docSeed("AGENTS.md", "RUNTIME AGENTS GUIDE")

	setter := &fakeSkillsSetter{}
	RefreshSkillsContext(setter, store, SkillsPromptInput{})
	if !strings.Contains(setter.got, "RUNTIME AGENTS GUIDE") || !strings.Contains(setter.got, "## Skills") {
		t.Fatalf("skills context should compose AGENTS + skills index:\n%s", setter.got)
	}
	if !strings.Contains(setter.got, "`gmail-web`") || strings.Contains(setter.got, "BODY") {
		t.Fatalf("skills context should be the index (trigger only, no body):\n%s", setter.got)
	}

	store.unlocked = false
	RefreshSkillsContext(setter, store, SkillsPromptInput{})
	if setter.got != "" {
		t.Fatalf("locked vault should clear skills context, got %q", setter.got)
	}
}
