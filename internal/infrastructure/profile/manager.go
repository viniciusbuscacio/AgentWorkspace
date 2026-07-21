package profile

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"aw/internal/domain"
)

const profilesFile = "profiles.json"

type Profile struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Avatar    string `json:"avatar"`
	VaultDir  string `json:"vaultDir"`
	LastUsed  string `json:"lastUsed"`
	CreatedAt string `json:"createdAt"`
}

type Info = domain.ProfileInfo

type Manager struct {
	mu       sync.Mutex
	baseDir  string
	filePath string
	profiles []Profile
}

func DefaultBaseDir() string {
	if override := strings.TrimSpace(os.Getenv("aw_DATA_DIR")); override != "" {
		return override
	}
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "aw")
}

func NewManager(baseDir string) *Manager {
	if strings.TrimSpace(baseDir) == "" {
		baseDir = DefaultBaseDir()
	}
	manager := &Manager{
		baseDir:  baseDir,
		filePath: filepath.Join(baseDir, profilesFile),
	}
	manager.load()
	return manager
}

func (m *Manager) ListInfo() []Info {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.listInfoLocked()
}

func (m *Manager) GetInfo(id string) (Info, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	profile, ok := m.getLocked(id)
	if !ok {
		return Info{}, false
	}
	return profileToInfo(profile), true
}

func (m *Manager) GetDefaultInfo() (Info, bool) {
	profile, ok := m.GetDefault()
	if !ok {
		return Info{}, false
	}
	return profileToInfo(profile), true
}

func (m *Manager) GetDefault() (Profile, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.profiles) == 0 {
		return Profile{}, false
	}
	profiles := m.sortedLocked()
	return profiles[0], true
}

func (m *Manager) Get(id string) (Profile, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getLocked(id)
}

func (m *Manager) Create(name, avatar, vaultDir string) (Info, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	name = strings.TrimSpace(name)
	if name == "" {
		name = "AgentWorkspace"
	}
	if strings.TrimSpace(avatar) == "" {
		avatar = "database_upload"
	}
	if strings.TrimSpace(vaultDir) == "" {
		vaultDir = filepath.Join(m.baseDir, "profiles", "prof_"+randomHex(8))
	}
	normalizedDir, err := normalizeDir(vaultDir)
	if err != nil {
		return Info{}, err
	}
	if err := os.MkdirAll(normalizedDir, 0o700); err != nil {
		return Info{}, err
	}

	now := nowString()
	for idx := range m.profiles {
		if sameDir(m.profiles[idx].VaultDir, normalizedDir) {
			m.profiles[idx].Name = name
			m.profiles[idx].Avatar = avatar
			m.profiles[idx].LastUsed = now
			if err := m.saveLocked(); err != nil {
				return Info{}, err
			}
			return profileToInfo(m.profiles[idx]), nil
		}
	}

	profile := Profile{
		ID:        "prof_" + randomHex(8),
		Name:      name,
		Avatar:    avatar,
		VaultDir:  normalizedDir,
		LastUsed:  now,
		CreatedAt: now,
	}
	m.profiles = append(m.profiles, profile)
	if err := m.saveLocked(); err != nil {
		return Info{}, err
	}
	return profileToInfo(profile), nil
}

func (m *Manager) CreateAtLocation(parentDir, folderName, avatar string) (Info, error) {
	parentDir = strings.TrimSpace(parentDir)
	if parentDir == "" {
		return Info{}, errors.New("parent folder is required")
	}
	folderName = strings.TrimSpace(folderName)
	if folderName == "" {
		folderName = "AgentWorkspace"
	}
	if strings.ContainsAny(folderName, `/\`) || folderName == "." || folderName == ".." {
		return Info{}, errors.New("folder name must be a simple folder name")
	}
	vaultDir := filepath.Join(parentDir, folderName)
	return m.Create(folderName, avatar, vaultDir)
}

func (m *Manager) Import(name, avatar, vaultDir string) (Info, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = filepath.Base(vaultDir)
	}
	if name == "." || name == string(filepath.Separator) || name == "" {
		name = "AgentWorkspace"
	}
	normalizedDir, err := normalizeDir(vaultDir)
	if err != nil {
		return Info{}, err
	}
	if !fileExists(filepath.Join(normalizedDir, "vault.db")) {
		return Info{}, errors.New("no vault.db found in the selected folder")
	}
	return m.Create(name, avatar, normalizedDir)
}

func (m *Manager) Touch(id string) (Info, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for idx := range m.profiles {
		if m.profiles[idx].ID == id {
			m.profiles[idx].LastUsed = nowString()
			if err := m.saveLocked(); err != nil {
				return Info{}, err
			}
			return profileToInfo(m.profiles[idx]), nil
		}
	}
	return Info{}, errors.New("profile not found")
}

func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for idx := range m.profiles {
		if m.profiles[idx].ID == id {
			m.profiles = append(m.profiles[:idx], m.profiles[idx+1:]...)
			return m.saveLocked()
		}
	}
	return errors.New("profile not found")
}

func (m *Manager) MigrateExistingVault(vaultDir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.profiles) > 0 {
		return nil
	}
	normalizedDir, err := normalizeDir(vaultDir)
	if err != nil {
		return err
	}
	if !fileExists(filepath.Join(normalizedDir, "vault.db")) {
		return nil
	}
	now := nowString()
	m.profiles = append(m.profiles, Profile{
		ID:        "prof_" + randomHex(8),
		Name:      filepath.Base(normalizedDir),
		Avatar:    "database_upload",
		VaultDir:  normalizedDir,
		LastUsed:  now,
		CreatedAt: now,
	})
	return m.saveLocked()
}

func (m *Manager) load() {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		m.profiles = []Profile{}
		return
	}
	if err := json.Unmarshal(data, &m.profiles); err != nil {
		m.profiles = []Profile{}
		return
	}
	if m.dedupeLocked() {
		_ = m.saveLocked()
	}
}

func (m *Manager) listInfoLocked() []Info {
	profiles := m.sortedLocked()
	result := make([]Info, 0, len(profiles))
	for _, profile := range profiles {
		result = append(result, profileToInfo(profile))
	}
	return result
}

func (m *Manager) sortedLocked() []Profile {
	profiles := append([]Profile(nil), m.profiles...)
	sort.SliceStable(profiles, func(i, j int) bool {
		return profiles[i].LastUsed > profiles[j].LastUsed
	})
	return profiles
}

func (m *Manager) getLocked(id string) (Profile, bool) {
	for _, profile := range m.profiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return Profile{}, false
}

func (m *Manager) dedupeLocked() bool {
	seen := map[string]Profile{}
	changed := false
	for _, profile := range m.profiles {
		key := normalizeDirBestEffort(profile.VaultDir)
		existing, ok := seen[key]
		if !ok || profile.LastUsed > existing.LastUsed {
			seen[key] = profile
		}
		if ok {
			changed = true
		}
	}
	if !changed {
		return false
	}
	m.profiles = make([]Profile, 0, len(seen))
	for _, profile := range seen {
		m.profiles = append(m.profiles, profile)
	}
	return true
}

func (m *Manager) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(m.filePath), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m.profiles, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.filePath, data, 0o600)
}

func profileToInfo(profile Profile) Info {
	return Info{
		ID:          profile.ID,
		Name:        profile.Name,
		Avatar:      profile.Avatar,
		VaultDir:    profile.VaultDir,
		LastUsed:    profile.LastUsed,
		CreatedAt:   profile.CreatedAt,
		HasVault:    fileExists(filepath.Join(profile.VaultDir, "vault.db")),
		HasRecovery: fileExists(filepath.Join(profile.VaultDir, "vault.recovery")),
	}
}

func normalizeDir(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", errors.New("vault dir is required")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func normalizeDirBestEffort(dir string) string {
	normalized, err := normalizeDir(dir)
	if err != nil {
		return filepath.Clean(dir)
	}
	return normalized
}

func sameDir(a, b string) bool {
	return strings.EqualFold(normalizeDirBestEffort(a), normalizeDirBestEffort(b))
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func nowString() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	const alphabet = "0123456789abcdef"
	out := make([]byte, len(buf)*2)
	for i, b := range buf {
		out[i*2] = alphabet[b>>4]
		out[i*2+1] = alphabet[b&0x0f]
	}
	return string(out)
}
