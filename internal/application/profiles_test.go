package application

import (
	"errors"
	"path/filepath"
	"testing"

	"aw/internal/domain"
)

type memoryProfileStore struct {
	infos    map[string]domain.ProfileInfo
	touched  []string
	created  []domain.ProfileInfo
	migrated []string
}

func newMemoryProfileStore(infos ...domain.ProfileInfo) *memoryProfileStore {
	store := &memoryProfileStore{infos: map[string]domain.ProfileInfo{}}
	for _, info := range infos {
		store.infos[info.ID] = info
	}
	return store
}

func (store *memoryProfileStore) ListInfo() []domain.ProfileInfo {
	out := make([]domain.ProfileInfo, 0, len(store.infos))
	for _, info := range store.infos {
		out = append(out, info)
	}
	return out
}

func (store *memoryProfileStore) GetInfo(id string) (domain.ProfileInfo, bool) {
	info, ok := store.infos[id]
	return info, ok
}

func (store *memoryProfileStore) Create(name string, avatar string, vaultDir string) (domain.ProfileInfo, error) {
	info := domain.ProfileInfo{ID: "created", Name: name, Avatar: avatar, VaultDir: vaultDir}
	store.infos[info.ID] = info
	store.created = append(store.created, info)
	return info, nil
}

func (store *memoryProfileStore) CreateAtLocation(parentDir string, folderName string, avatar string) (domain.ProfileInfo, error) {
	return store.Create(folderName, avatar, filepath.Join(parentDir, folderName))
}

func (store *memoryProfileStore) Import(name string, avatar string, vaultDir string) (domain.ProfileInfo, error) {
	return store.Create(name, avatar, vaultDir)
}

func (store *memoryProfileStore) Touch(id string) (domain.ProfileInfo, error) {
	info, ok := store.infos[id]
	if !ok {
		return domain.ProfileInfo{}, errors.New("profile not found")
	}
	store.touched = append(store.touched, id)
	return info, nil
}

func (store *memoryProfileStore) Delete(id string) error {
	if _, ok := store.infos[id]; !ok {
		return errors.New("profile not found")
	}
	delete(store.infos, id)
	return nil
}

func (store *memoryProfileStore) MigrateExistingVault(vaultDir string) error {
	store.migrated = append(store.migrated, vaultDir)
	return nil
}

func (store *memoryProfileStore) GetDefaultInfo() (domain.ProfileInfo, bool) {
	for _, info := range store.infos {
		return info, true
	}
	return domain.ProfileInfo{}, false
}

type fakeProfileVault struct {
	dir      string
	unlocked bool
	locked   bool
}

func (vault *fakeProfileVault) Dir() string {
	return vault.dir
}

func (vault *fakeProfileVault) SetDir(dir string) error {
	vault.dir = dir
	return nil
}

func (vault *fakeProfileVault) IsUnlocked() bool {
	return vault.unlocked
}

func (vault *fakeProfileVault) Lock() error {
	vault.locked = true
	vault.unlocked = false
	return nil
}

type fakeProfileConfig struct {
	vaultDir string
}

func (config *fakeProfileConfig) SaveVaultDir(dir string) error {
	config.vaultDir = dir
	return nil
}

func (config *fakeProfileConfig) SaveAutoLockMinutes(int) error {
	return nil
}

func TestApplyProfileIsIdempotentForCurrentUnlockedProfile(t *testing.T) {
	info := domain.ProfileInfo{ID: "one", VaultDir: "/vault/one"}
	vault := &fakeProfileVault{dir: info.VaultDir, unlocked: true}
	config := &fakeProfileConfig{}
	result, err := ApplyProfile(vault, config, info.ID, info)
	if err != nil {
		t.Fatalf("ApplyProfile() error = %v", err)
	}
	if result.CurrentProfileID != info.ID || vault.locked || config.vaultDir != "" {
		t.Fatalf("result=%+v locked=%v saved=%q; want no-op", result, vault.locked, config.vaultDir)
	}
}

func TestBootstrapProfilesMigratesConfiguredAndDefaultVaults(t *testing.T) {
	defaultInfo := domain.ProfileInfo{ID: "default", VaultDir: "/vault/default"}
	profiles := newMemoryProfileStore(defaultInfo)
	result := BootstrapProfiles(profiles, ProfileBootstrapInput{
		ConfigVaultDir:  "/vault/config",
		DefaultVaultDir: "/vault/fallback",
	})
	if result.CurrentProfileID != defaultInfo.ID || result.VaultDir != defaultInfo.VaultDir {
		t.Fatalf("BootstrapProfiles() = %+v, want default profile", result)
	}
	if len(profiles.migrated) != 2 || profiles.migrated[0] != "/vault/config" || profiles.migrated[1] != "/vault/fallback" {
		t.Fatalf("migrated = %v, want configured and default vaults", profiles.migrated)
	}
}

func TestApplyProfileLocksBeforeSwitchingUnlockedVault(t *testing.T) {
	info := domain.ProfileInfo{ID: "two", VaultDir: "/vault/two"}
	vault := &fakeProfileVault{dir: "/vault/one", unlocked: true}
	config := &fakeProfileConfig{}
	result, err := ApplyProfile(vault, config, "one", info)
	if err != nil {
		t.Fatalf("ApplyProfile() error = %v", err)
	}
	if !vault.locked || vault.dir != info.VaultDir || config.vaultDir != info.VaultDir || result.CurrentProfileID != info.ID {
		t.Fatalf("vault/config/result = %+v/%+v/%+v; want switched", vault, config, result)
	}
}

func TestCreateVaultForProfileCreatesMissingProfileAndTouchesIt(t *testing.T) {
	profiles := newMemoryProfileStore()
	vault := &fakeVaultLifecycle{recoveryKey: "recovery"}
	vaultWithDir := &fakeVaultCreateStore{fakeVaultLifecycle: vault, dir: "/tmp/awVault"}
	result, err := CreateVaultForProfile(vaultWithDir, profiles, "", "senha")
	if err != nil {
		t.Fatalf("CreateVaultForProfile() error = %v", err)
	}
	if result.CurrentProfileID != "created" || result.RecoveryKey != "recovery" || vault.createdWith != "senha" {
		t.Fatalf("result=%+v createdWith=%q; want created profile and vault", result, vault.createdWith)
	}
	if len(profiles.touched) != 1 || profiles.touched[0] != "created" {
		t.Fatalf("touched=%v, want [created]", profiles.touched)
	}
}

type fakeVaultCreateStore struct {
	*fakeVaultLifecycle
	dir string
}

func (vault *fakeVaultCreateStore) Dir() string {
	return vault.dir
}

func (vault *fakeVaultCreateStore) SetDir(dir string) error {
	vault.dir = dir
	return nil
}

func TestEnsureProfileByNameReturnsExistingMatch(t *testing.T) {
	store := newMemoryProfileStore(domain.ProfileInfo{ID: "p1", Name: "1234", VaultDir: "/dev/vault"})
	info, err := EnsureProfileByName(store, "1234")
	if err != nil {
		t.Fatalf("EnsureProfileByName error = %v", err)
	}
	if info.ID != "p1" {
		t.Fatalf("expected existing p1, got %+v", info)
	}
	if len(store.created) != 0 {
		t.Fatalf("should not create when a match exists, created = %v", store.created)
	}
}

func TestEnsureProfileByNameCreatesWhenMissing(t *testing.T) {
	store := newMemoryProfileStore(domain.ProfileInfo{ID: "other", Name: "Real"})
	info, err := EnsureProfileByName(store, "1234")
	if err != nil {
		t.Fatalf("EnsureProfileByName error = %v", err)
	}
	if info.Name != "1234" {
		t.Fatalf("expected created profile named 1234, got %+v", info)
	}
	if len(store.created) != 1 {
		t.Fatalf("expected exactly one create, got %v", store.created)
	}
}

func TestEnsureProfileByNameNilStore(t *testing.T) {
	if _, err := EnsureProfileByName(nil, "1234"); err == nil {
		t.Fatal("expected error for nil store")
	}
}

func TestForgetAllProfilesEmptiesTheStore(t *testing.T) {
	store := newMemoryProfileStore(
		domain.ProfileInfo{ID: "p1", Name: "One", VaultDir: "/vaults/one"},
		domain.ProfileInfo{ID: "p2", Name: "Two", VaultDir: "/vaults/two"},
	)
	if err := ForgetAllProfiles(store); err != nil {
		t.Fatalf("ForgetAllProfiles error = %v", err)
	}
	if remaining := store.ListInfo(); len(remaining) != 0 {
		t.Fatalf("expected empty store, got %v", remaining)
	}
}

func TestForgetAllProfilesNilStore(t *testing.T) {
	if err := ForgetAllProfiles(nil); err == nil {
		t.Fatal("expected error for nil store")
	}
}
