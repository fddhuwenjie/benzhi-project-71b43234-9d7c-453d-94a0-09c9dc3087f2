package list_cache_stale_after_create_test

import (
	"testing"

	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/application"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/domain"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/persistence"
)

func TestListCacheReflectsApplicationCreatedAfterFirstQuery(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store, nil)

	initial, err := service.List()
	if err != nil {
		t.Fatalf("initial List() error = %v", err)
	}
	if len(initial) != 0 {
		t.Fatalf("initial List() length = %d, want 0", len(initial))
	}

	created, err := service.CreateDraft(application.CreateDraftCommand{
		RequestID: "list-cache-create",
		Draft: domain.DraftInput{
			TreeCode:        "GS-CACHE-001",
			Species:         "香樟",
			CurrentLocation: "现址 A",
			TargetLocation:  "目标地 B",
			MigrationReason: "保护性迁移",
		},
	})
	if err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}
	if _, err := store.Load(created.Application.ID); err != nil {
		t.Fatalf("created application was not persisted: %v", err)
	}

	afterCreate, err := service.List()
	if err != nil {
		t.Fatalf("List() after CreateDraft() error = %v", err)
	}
	if len(afterCreate) != 1 || afterCreate[0].ID != created.Application.ID {
		t.Fatalf("List() after CreateDraft() = %+v, want persisted application %s", afterCreate, created.Application.ID)
	}
}
