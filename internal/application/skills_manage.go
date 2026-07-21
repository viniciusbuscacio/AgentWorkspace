package application

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"aw/internal/domain"
	"aw/internal/domain/ports"
	"aw/skills"
)

// skillIDPattern is the v1 rule for a created skill id: lowercase alphanumerics
// and hyphens, starting with an alphanumeric. The id is authored explicitly (no
// auto-slugging), so "My Skill"/"my_skill"/"my.skill" are rejected rather than
// silently rewritten.
var skillIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func isWindowsDriveAbsolute(p string) bool {
	return len(p) >= 3 && ((p[0] >= 'A' && p[0] <= 'Z') || (p[0] >= 'a' && p[0] <= 'z')) && p[1] == ':' && (p[2] == '\\' || p[2] == '/')
}

// validateSkillFilePath rejects agent-supplied paths that escape the skill
// folder or are not relative: empty, absolute (unix or windows), or containing
// a ".." segment. Skill files always live under the skill's own folder.
func validateSkillFilePath(p string) error {
	p = strings.TrimSpace(p)
	if p == "" {
		return errors.New("every skill file needs a path")
	}
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "\\") || filepath.VolumeName(p) != "" || isWindowsDriveAbsolute(p) {
		return fmt.Errorf("skill file path must be relative: %q", p)
	}
	for _, seg := range strings.Split(strings.ReplaceAll(p, "\\", "/"), "/") {
		if seg == ".." {
			return fmt.Errorf("skill file path must not contain '..': %q", p)
		}
	}
	return nil
}

// validateSkillFiles checks an agent-supplied file set: at least SKILL.md, every
// path safe, no duplicates.
func validateSkillFiles(files []domain.SkillFile) error {
	if len(files) == 0 {
		return errors.New("at least SKILL.md is required")
	}
	seen := make(map[string]bool, len(files))
	hasMarkdown := false
	for _, f := range files {
		path := strings.TrimSpace(f.Path)
		if err := validateSkillFilePath(path); err != nil {
			return err
		}
		if seen[path] {
			return fmt.Errorf("duplicate skill file path: %q", path)
		}
		seen[path] = true
		if path == domain.SkillMarkdownPath {
			hasMarkdown = true
		}
	}
	if !hasMarkdown {
		return errors.New("SKILL.md is required")
	}
	return nil
}

func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// CreateSkill creates a new user skill directly from supplied files (no
// filesystem import). id must be unique; origin is always user. Explicit
// name/description are optional convenience and must not diverge from the
// SKILL.md frontmatter (the frontmatter is authoritative).
func CreateSkill(store ports.SkillStore, id, name, description string, enabled bool, files []domain.SkillFile) error {
	if store == nil {
		return errors.New("skill store is required")
	}
	if !store.IsUnlocked() {
		return errVaultLocked
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("skill id is required")
	}
	if !skillIDPattern.MatchString(id) {
		return fmt.Errorf("skill id %q must match %s (lowercase letters, digits and hyphens)", id, skillIDPattern.String())
	}
	if err := validateSkillFiles(files); err != nil {
		return err
	}
	if _, ok, err := store.GetSkill(id); err != nil {
		return err
	} else if ok {
		return fmt.Errorf("skill %q already exists; choose a new id", id)
	}
	normalized := make([]domain.SkillFile, 0, len(files))
	for _, f := range files {
		content := skills.Normalize(f.Content)
		normalized = append(normalized, domain.SkillFile{
			Path: strings.TrimSpace(f.Path), Content: content, ContentHash: skills.Hash(content),
		})
	}
	parsedName, parsedDesc := skillMetaFromFiles(normalized)
	if strings.TrimSpace(name) != "" && collapseWhitespace(name) != collapseWhitespace(parsedName) {
		return fmt.Errorf("name %q diverges from SKILL.md frontmatter %q", name, parsedName)
	}
	if strings.TrimSpace(description) != "" && collapseWhitespace(description) != collapseWhitespace(parsedDesc) {
		return errors.New("description diverges from SKILL.md frontmatter; omit it or match the frontmatter")
	}
	finalName := parsedName
	if finalName == "" {
		finalName = id
	}
	return store.UpsertSkill(domain.Skill{
		ID:          id,
		Name:        finalName,
		Description: parsedDesc,
		Origin:      domain.SkillOriginUser,
		Enabled:     enabled,
		Files:       normalized,
	})
}

// SkillView is the management-panel row: a skill plus the derived state the UI
// needs (customized = user edited a builtin; updateAvailable = a newer bundled
// seed exists for a customized builtin that the safe-update path did not apply).
type SkillView struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	Origin          string `json:"origin"`
	Enabled         bool   `json:"enabled"`
	Customized      bool   `json:"customized"`
	UpdateAvailable bool   `json:"updateAvailable"`
	Deleted         bool   `json:"deleted"`
	FileCount       int    `json:"fileCount"`
}

// ListSkillViews returns every skill (including soft-deleted, so the UI can
// offer "restore") with derived flags. seeds is the current bundled seed set,
// used to detect an available update on a customized builtin.
func ListSkillViews(store ports.SkillStore, seeds []domain.Skill) ([]SkillView, error) {
	if store == nil {
		return nil, errors.New("skill store is required")
	}
	if !store.IsUnlocked() {
		return nil, errVaultLocked
	}
	list, err := store.ListSkills(true)
	if err != nil {
		return nil, err
	}
	seedMdHash := make(map[string]string, len(seeds))
	for _, s := range seeds {
		if md, ok := skillMdFile(s); ok {
			seedMdHash[s.ID] = md.ContentHash
		}
	}
	views := make([]SkillView, 0, len(list))
	for _, s := range list {
		v := SkillView{
			ID: s.ID, Name: s.Name, Description: s.Description, Origin: s.Origin,
			Enabled: s.Enabled, Deleted: s.Deleted, FileCount: len(s.Files),
		}
		if s.Origin == domain.SkillOriginBuiltin {
			if md, ok := skillMdFile(s); ok && md.SeedHash != "" && md.ContentHash != md.SeedHash {
				v.Customized = true
				if seedHash, ok := seedMdHash[s.ID]; ok && seedHash != md.SeedHash {
					v.UpdateAvailable = true
				}
			}
		}
		views = append(views, v)
	}
	sort.Slice(views, func(i, j int) bool { return views[i].ID < views[j].ID })
	return views, nil
}

// GetSkillDetail returns one skill with all its files (for view/edit).
func GetSkillDetail(store ports.SkillStore, id string) (domain.Skill, error) {
	if store == nil {
		return domain.Skill{}, errors.New("skill store is required")
	}
	if !store.IsUnlocked() {
		return domain.Skill{}, errVaultLocked
	}
	skill, ok, err := store.GetSkill(id)
	if err != nil {
		return domain.Skill{}, err
	}
	if !ok {
		return domain.Skill{}, fmt.Errorf("skill %q not found", id)
	}
	return skill, nil
}

// SetSkillEnabled toggles a skill on/off.
func SetSkillEnabled(store ports.SkillStore, id string, enabled bool) error {
	if store == nil {
		return errors.New("skill store is required")
	}
	if !store.IsUnlocked() {
		return errVaultLocked
	}
	return store.SetSkillEnabled(id, enabled)
}

// SaveSkillFiles persists an edited skill folder. Content is normalized and
// re-hashed; the per-file seed_hash is preserved so a builtin edited by the
// user reads as customized (content_hash != seed_hash) and the seed update path
// leaves it alone. name/description are refreshed from the edited SKILL.md.
func SaveSkillFiles(store ports.SkillStore, id string, files []domain.SkillFile) error {
	if store == nil {
		return errors.New("skill store is required")
	}
	if !store.IsUnlocked() {
		return errVaultLocked
	}
	cur, ok, err := store.GetSkill(id)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("skill %q not found", id)
	}
	if err := validateSkillFiles(files); err != nil {
		return err
	}
	seedByPath := make(map[string]string, len(cur.Files))
	for _, f := range cur.Files {
		seedByPath[f.Path] = f.SeedHash
	}
	normalized := make([]domain.SkillFile, 0, len(files))
	for _, f := range files {
		path := strings.TrimSpace(f.Path)
		content := skills.Normalize(f.Content)
		normalized = append(normalized, domain.SkillFile{
			Path: path, Content: content, ContentHash: skills.Hash(content), SeedHash: seedByPath[path],
		})
	}
	cur.Files = normalized
	if name, desc := skillMetaFromFiles(normalized); name != "" {
		cur.Name = name
		cur.Description = desc
	}
	return store.UpsertSkill(cur)
}

// ImportSkill adds a user skill. It refuses an id that already exists (decision
// #6: no implicit override). Content is normalized + hashed; origin=user with no
// seed tracking, so the seed update path never touches it.
func ImportSkill(store ports.SkillStore, skill domain.Skill) error {
	if store == nil {
		return errors.New("skill store is required")
	}
	if !store.IsUnlocked() {
		return errVaultLocked
	}
	skill.ID = strings.TrimSpace(skill.ID)
	if skill.ID == "" {
		return errors.New("skill id is required")
	}
	if _, ok, err := store.GetSkill(skill.ID); err != nil {
		return err
	} else if ok {
		return fmt.Errorf("skill %q already exists; choose a new id", skill.ID)
	}
	normalized := make([]domain.SkillFile, 0, len(skill.Files))
	hasMarkdown := false
	for _, f := range skill.Files {
		path := strings.TrimSpace(f.Path)
		if path == "" {
			continue
		}
		content := skills.Normalize(f.Content)
		normalized = append(normalized, domain.SkillFile{Path: path, Content: content, ContentHash: skills.Hash(content)})
		if path == domain.SkillMarkdownPath {
			hasMarkdown = true
		}
	}
	if !hasMarkdown {
		return errors.New("an imported skill must contain SKILL.md")
	}
	skill.Files = normalized
	skill.Origin = domain.SkillOriginUser
	skill.Enabled = true
	skill.Deleted = false
	skill.SeedVersion = ""
	if name, desc := skillMetaFromFiles(normalized); name != "" {
		skill.Name = name
		skill.Description = desc
	}
	if strings.TrimSpace(skill.Name) == "" {
		skill.Name = skill.ID
	}
	return store.UpsertSkill(skill)
}

// ImportSummary reports the result of a batch folder import.
type ImportSummary struct {
	Imported []string    `json:"imported"`
	Skipped  []SkillSkip `json:"skipped"`
}

// SkillSkip is one skill that was not imported, with the reason.
type SkillSkip struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// ImportSkillFromFile imports the single skill whose SKILL.md is at skillMdPath
// ("Import Skill"). Preserves the import rules (origin=user, enabled, blocks an
// existing id, requires SKILL.md, name/description from the YAML).
func ImportSkillFromFile(store ports.SkillStore, skillMdPath string) error {
	if store == nil {
		return errors.New("skill store is required")
	}
	if !store.IsUnlocked() {
		return errVaultLocked
	}
	skill, err := skills.LoadSkillFromMarkdown(skillMdPath)
	if err != nil {
		return err
	}
	return ImportSkill(store, skill)
}

// ImportSkillsFromParent imports every direct subfolder of root that has a
// SKILL.md ("Import Folder"). It is a partial import with a report: an
// individual skill failing (no SKILL.md, duplicate id, read/persist error)
// becomes a Skipped entry and never aborts the batch. Strict mode: if root
// itself is a single skill (SKILL.md at its root), nothing is imported and the
// caller is told to use Import Skill instead.
func ImportSkillsFromParent(store ports.SkillStore, root string) (ImportSummary, error) {
	if store == nil {
		return ImportSummary{}, errors.New("skill store is required")
	}
	if !store.IsUnlocked() {
		return ImportSummary{}, errVaultLocked
	}
	if skills.DirHasSkillMarkdown(root) {
		return ImportSummary{}, errors.New("this folder looks like a single skill; use Import Skill and select its SKILL.md")
	}
	loaded, skippedDirs, err := skills.LoadSkillsFromParent(root)
	if err != nil {
		return ImportSummary{}, err
	}
	summary := ImportSummary{Imported: []string{}, Skipped: []SkillSkip{}}
	for _, dir := range skippedDirs {
		summary.Skipped = append(summary.Skipped, SkillSkip{ID: dir, Reason: "no SKILL.md found"})
	}
	for _, skill := range loaded {
		if err := ImportSkill(store, skill); err != nil {
			summary.Skipped = append(summary.Skipped, SkillSkip{ID: skill.ID, Reason: importSkipReason(err)})
			continue
		}
		summary.Imported = append(summary.Imported, skill.ID)
	}
	sort.Strings(summary.Imported)
	sort.Slice(summary.Skipped, func(i, j int) bool { return summary.Skipped[i].ID < summary.Skipped[j].ID })
	return summary, nil
}

func importSkipReason(err error) string {
	msg := err.Error()
	if strings.Contains(msg, "already exists") {
		return "already exists"
	}
	return msg
}

// DeleteSkill removes a skill: soft for builtins (so the seed bootstrap won't
// resurrect them), hard for user skills.
func DeleteSkill(store ports.SkillStore, id string) error {
	if store == nil {
		return errors.New("skill store is required")
	}
	if !store.IsUnlocked() {
		return errVaultLocked
	}
	cur, ok, err := store.GetSkill(id)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("skill %q not found", id)
	}
	if cur.Origin == domain.SkillOriginBuiltin {
		return store.SoftDeleteSkill(id)
	}
	return store.DeleteSkill(id)
}

// ResetSkillToSeed restores a builtin skill to its bundled seed, discarding the
// user's customization and clearing the soft-delete. enabled is preserved.
func ResetSkillToSeed(store ports.SkillStore, seeds []domain.Skill, id string) error {
	if store == nil {
		return errors.New("skill store is required")
	}
	if !store.IsUnlocked() {
		return errVaultLocked
	}
	var seed domain.Skill
	found := false
	for _, s := range seeds {
		if s.ID == id {
			seed = s
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("no bundled seed for skill %q", id)
	}
	seed.Origin = domain.SkillOriginBuiltin
	seed.Deleted = false
	seed.Enabled = true
	if cur, ok, err := store.GetSkill(id); err == nil && ok {
		seed.Enabled = cur.Enabled && !cur.Deleted
		if cur.Deleted {
			seed.Enabled = true
		}
	}
	return store.UpsertSkill(seed)
}

func skillMdFile(s domain.Skill) (domain.SkillFile, bool) {
	for _, f := range s.Files {
		if f.Path == domain.SkillMarkdownPath {
			return f, true
		}
	}
	return domain.SkillFile{}, false
}

func skillMetaFromFiles(files []domain.SkillFile) (name, description string) {
	for _, f := range files {
		if f.Path == domain.SkillMarkdownPath {
			return skills.ParseSkillMeta(f.Content)
		}
	}
	return "", ""
}
