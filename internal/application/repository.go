package application

import (
	"encoding/json"

	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/domain"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/persistence"
)

type Repository interface {
	Create(*domain.MigrationApplication) error
	Load(string) (*domain.MigrationApplication, error)
	List() ([]*domain.MigrationApplication, error)
	FindByTreeCode(string) ([]*domain.MigrationApplication, error)
	Save(int, *domain.MigrationApplication, string, string, json.RawMessage) (persistence.CommandResult, error)
	PreviousResult(string, string, string) (persistence.CommandResult, bool, error)
	Events(string) ([]persistence.EventRecord, error)
	SaveReceipt(domain.ArchiveIntegrityReceipt) error
	LatestReceipt(string) (*domain.ArchiveIntegrityReceipt, error)
}
