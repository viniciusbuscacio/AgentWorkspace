package ports

import "aw/internal/domain"

type ProfileReader interface {
	ListInfo() []domain.ProfileInfo
	GetInfo(id string) (domain.ProfileInfo, bool)
}

type ProfileStore interface {
	ProfileReader
	Create(name string, avatar string, vaultDir string) (domain.ProfileInfo, error)
	CreateAtLocation(parentDir string, folderName string, avatar string) (domain.ProfileInfo, error)
	Import(name string, avatar string, vaultDir string) (domain.ProfileInfo, error)
	Touch(id string) (domain.ProfileInfo, error)
	Delete(id string) error
}

type ProfileBootstrapStore interface {
	MigrateExistingVault(vaultDir string) error
	GetDefaultInfo() (domain.ProfileInfo, bool)
}
