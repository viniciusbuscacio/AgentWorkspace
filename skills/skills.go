// Package skills holds the bundled seed skills and runtime AGENTS.md (embedded
// in the binary), the cross-OS normalization + hashing used by the vault's
// update-safe seed mechanics, an optional on-disk dev override (AW_SKILLS_DIR),
// and the renderer that turns a skill set into a system-prompt block.
//
// It is intentionally storage-agnostic: skills live in the encrypted vault, not
// on disk. This package never reads the vault — it only produces the seed and
// renders an already-loaded set, so the agent runtime stays decoupled from it.
package skills

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"aw/internal/domain"
	"gopkg.in/yaml.v3"
)

//go:embed all:bundled
var bundledFS embed.FS

const bundledRoot = "bundled"

// SeedVersion is bumped when the bundled seed changes meaningfully. The
// update-safe rule keys on content hashes, not this value — it is telemetry
// only (a version can change without content changing and vice versa).
const SeedVersion = "1"

// DevSkillsDirEnv overrides the vault catalog with an on-disk skills directory
// for development. When set it substitutes the whole catalog (no overlay).
const DevSkillsDirEnv = "AW_SKILLS_DIR"

// Normalize makes file content stable across Mac/Windows/OneDrive so a line
// ending rewrite never masquerades as a user edit: LF endings, UTF-8 without
// BOM, no trailing whitespace, no trailing blank lines.
func Normalize(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	content = strings.TrimPrefix(content, "\ufeff")
	lines := strings.Split(content, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

// Hash returns the sha256 hex of the normalized content. Hashing always
// normalizes first so disk round-trips can never change the hash.
func Hash(content string) string {
	sum := sha256.Sum256([]byte(Normalize(content)))
	return hex.EncodeToString(sum[:])
}

// BundledSkills returns the seed skills embedded in the binary, each as a full
// folder (every file a SkillFile) with ContentHash == SeedHash precomputed.
func BundledSkills() ([]domain.Skill, error) {
	entries, err := bundledFS.ReadDir(bundledRoot)
	if err != nil {
		return nil, err
	}
	var skills []domain.Skill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue // top-level files (AGENTS.md) handled by BundledAgentsDoc
		}
		skill, ok, err := readBundledSkill(entry.Name())
		if err != nil {
			return nil, err
		}
		if ok {
			skills = append(skills, skill)
		}
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].ID < skills[j].ID })
	return skills, nil
}

func readBundledSkill(id string) (domain.Skill, bool, error) {
	base := path.Join(bundledRoot, id)
	var files []domain.SkillFile
	walkErr := fs.WalkDir(bundledFS, base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := bundledFS.ReadFile(p)
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(p, base+"/")
		content := Normalize(string(data))
		h := Hash(content)
		files = append(files, domain.SkillFile{Path: rel, Content: content, ContentHash: h, SeedHash: h})
		return nil
	})
	if walkErr != nil {
		return domain.Skill{}, false, walkErr
	}
	return assembleSeedSkill(id, files, domain.SkillOriginBuiltin, true), hasSkillMarkdown(files), nil
}

// BundledAgentsDoc returns the embedded runtime AGENTS.md as an app document
// seed; ok is false when the binary ships without one.
func BundledAgentsDoc() (domain.AppDocument, bool, error) {
	data, err := bundledFS.ReadFile(path.Join(bundledRoot, domain.AgentsDocumentID))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return domain.AppDocument{}, false, nil
		}
		return domain.AppDocument{}, false, err
	}
	content := Normalize(string(data))
	h := Hash(content)
	return domain.AppDocument{
		ID:          domain.AgentsDocumentID,
		Content:     content,
		ContentHash: h,
		SeedHash:    h,
		Origin:      domain.SkillOriginBuiltin,
		SeedVersion: SeedVersion,
	}, true, nil
}

// DevDir returns the trimmed AW_SKILLS_DIR override, or "" when unset.
func DevDir() string {
	return strings.TrimSpace(os.Getenv(DevSkillsDirEnv))
}

// DevCatalog loads skills and (optionally) AGENTS.md from AW_SKILLS_DIR. The
// active return is true whenever the env var is set, even if the directory is
// empty or unreadable — so the caller can honor "dev override substitutes the
// whole vault catalog" and log it. Dev content is origin=user (it never feeds
// the seed update path).
func DevCatalog() (skills []domain.Skill, agents domain.AppDocument, hasAgents bool, root string, active bool) {
	root = DevDir()
	if root == "" {
		return nil, domain.AppDocument{}, false, "", false
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, domain.AppDocument{}, false, root, true
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if skill, ok := readDiskSkill(root, entry.Name()); ok {
				skills = append(skills, skill)
			}
			continue
		}
		if entry.Name() == domain.AgentsDocumentID {
			if data, err := os.ReadFile(filepath.Join(root, entry.Name())); err == nil {
				content := Normalize(string(data))
				agents = domain.AppDocument{
					ID:          domain.AgentsDocumentID,
					Content:     content,
					ContentHash: Hash(content),
					Origin:      domain.SkillOriginUser,
				}
				hasAgents = true
			}
		}
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].ID < skills[j].ID })
	return skills, agents, hasAgents, root, true
}

func readDiskSkill(root, id string) (domain.Skill, bool) {
	base := filepath.Join(root, id)
	var files []domain.SkillFile
	walkErr := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(base, p)
		if err != nil {
			return err
		}
		content := Normalize(string(data))
		files = append(files, domain.SkillFile{Path: filepath.ToSlash(rel), Content: content, ContentHash: Hash(content)})
		return nil
	})
	if walkErr != nil || !hasSkillMarkdown(files) {
		return domain.Skill{}, false
	}
	return assembleSeedSkill(id, files, domain.SkillOriginUser, true), true
}

// LoadSkillFromMarkdown loads the skill folder that contains the given SKILL.md
// file (in-app "Import Skill"). The basename must be exactly "SKILL.md"
// (case-sensitive): a folder is recognized as a skill only by that exact file,
// so accepting skill.md/Skill.md would load the folder but yield no usable
// skill. The id is the folder name; the result is origin=user.
func LoadSkillFromMarkdown(skillMdPath string) (domain.Skill, error) {
	skillMdPath = filepath.Clean(skillMdPath)
	if filepath.Base(skillMdPath) != domain.SkillMarkdownPath {
		return domain.Skill{}, errors.New("select a SKILL.md file")
	}
	dir := filepath.Dir(skillMdPath)
	parent := filepath.Dir(dir)
	id := filepath.Base(dir)
	skill, ok := readDiskSkill(parent, id)
	if !ok {
		return domain.Skill{}, errors.New("no SKILL.md found in " + dir)
	}
	return skill, nil
}

// LoadSkillsFromParent scans the DIRECT subfolders of root (one level, not
// recursive — so a SKILL.md nested under references/ is never captured) and
// loads each subfolder that contains a SKILL.md. skippedDirs holds the names of
// direct subfolders without a SKILL.md; loose files in root are ignored. err is
// only for a fatal failure (root missing or unreadable). It loads, it does not
// import — turning results into a per-skill report is the use case's job.
func LoadSkillsFromParent(root string) (loaded []domain.Skill, skippedDirs []string, err error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if skill, ok := readDiskSkill(root, name); ok {
			loaded = append(loaded, skill)
		} else {
			skippedDirs = append(skippedDirs, name)
		}
	}
	sort.Slice(loaded, func(i, j int) bool { return loaded[i].ID < loaded[j].ID })
	sort.Strings(skippedDirs)
	return loaded, skippedDirs, nil
}

// DirHasSkillMarkdown reports whether dir contains a SKILL.md directly at its
// root (used to detect "Import Folder pointed at a single skill").
func DirHasSkillMarkdown(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, domain.SkillMarkdownPath))
	return err == nil && !info.IsDir()
}

func assembleSeedSkill(id string, files []domain.SkillFile, origin string, enabled bool) domain.Skill {
	name, desc := "", ""
	for _, f := range files {
		if f.Path == domain.SkillMarkdownPath {
			name, desc = ParseSkillMeta(f.Content)
		}
	}
	if name == "" {
		name = id
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	seedVersion := ""
	if origin == domain.SkillOriginBuiltin {
		seedVersion = SeedVersion
	}
	return domain.Skill{
		ID:          id,
		Name:        name,
		Description: desc,
		Enabled:     enabled,
		Origin:      origin,
		SeedVersion: seedVersion,
		Files:       files,
	}
}

func hasSkillMarkdown(files []domain.SkillFile) bool {
	for _, f := range files {
		if f.Path == domain.SkillMarkdownPath {
			return true
		}
	}
	return false
}

// ParseSkillMeta extracts name and description from a SKILL.md frontmatter using
// a real YAML parser, so block scalars (`>`, `>-`, `|`) and quoted values with
// colons parse correctly. The description is normalized to a single readable
// trigger line (whitespace and line breaks collapsed) for the catalog/prompt;
// the stored SKILL.md content is never altered.
func ParseSkillMeta(content string) (name, description string) {
	frontmatter, _, ok := splitFrontmatter(content)
	if !ok {
		return "", ""
	}
	var meta struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(frontmatter), &meta); err != nil {
		return "", ""
	}
	return strings.TrimSpace(meta.Name), normalizeTriggerText(meta.Description)
}

// SkillMarkdownBody returns the markdown body of a SKILL.md (frontmatter
// stripped), or the trimmed content when there is no frontmatter.
func SkillMarkdownBody(content string) string {
	_, body, ok := splitFrontmatter(content)
	if !ok {
		return strings.TrimSpace(content)
	}
	return body
}

// splitFrontmatter separates a leading YAML frontmatter block from the markdown
// body. The block must open with a standalone `---` line at the top and close
// with the next standalone `---` line, so `---` (horizontal rules, em-dashes)
// inside the body never trips it. Returns ok=false when there is no valid
// frontmatter (body is then the whole trimmed content).
func splitFrontmatter(content string) (frontmatter, body string, ok bool) {
	lines := strings.Split(content, "\n")
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	if start >= len(lines) || strings.TrimSpace(lines[start]) != "---" {
		return "", strings.TrimSpace(content), false
	}
	for end := start + 1; end < len(lines); end++ {
		if strings.TrimSpace(lines[end]) == "---" {
			frontmatter = strings.Join(lines[start+1:end], "\n")
			body = strings.TrimSpace(strings.Join(lines[end+1:], "\n"))
			return frontmatter, body, true
		}
	}
	return "", strings.TrimSpace(content), false
}

// normalizeTriggerText collapses all whitespace (including the line breaks a
// `|` literal scalar preserves) into single spaces, yielding a one-line trigger.
func normalizeTriggerText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// InstructionForSkills renders the skills INDEX for the system prompt
// (progressive disclosure level 1): only each skill's id + trigger
// description, never the body. The agent pulls the full procedure on demand
// with the aw tool action `skill.read`. Empty when the set is empty.
func InstructionForSkills(list []domain.Skill) string {
	if len(list) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Skills\n\n")
	b.WriteString("These skills are available. When a task matches a skill's \"Use when\", " +
		"load its full procedure with the aw tool action `skill.read { id: \"<id>\" }` BEFORE acting " +
		"(and `skill.read { id, path }` for a listed auxiliary file). `skill.list` enumerates them.\n")
	for _, skill := range list {
		id := skill.ID
		b.WriteString("\n- `")
		b.WriteString(id)
		b.WriteString("`")
		if name := skill.Name; name != "" && name != id {
			b.WriteString(" (")
			b.WriteString(name)
			b.WriteString(")")
		}
		if skill.Description != "" {
			b.WriteString(" — Use when: ")
			b.WriteString(skill.Description)
		}
	}
	return strings.TrimSpace(b.String())
}
