package archive_verification_cache_race_test

import (
	"sync"
	"testing"
	"time"

	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/application"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/domain"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/persistence"
)

type receiptBarrierRepository struct {
	application.Repository
	arrived chan struct{}
	release chan struct{}
}

func (r *receiptBarrierRepository) SaveReceipt(domain.ArchiveIntegrityReceipt) error {
	r.arrived <- struct{}{}
	<-r.release
	return nil
}

func TestConcurrentArchiveVerificationCacheIsSynchronized(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	app := domain.NewApplication("app-archived", domain.DraftInput{TreeCode: "GS-RACE"}, now)
	app.Status = domain.StatusArchived
	app.Archive = &domain.ArchiveSummary{ID: "archive-race", ApplicationID: app.ID, ApprovedRevision: app.Revision, ArchivedAt: now}
	app.Archive.Digest = domain.CalculateArchiveDigest(*app.Archive)
	if err := store.Create(app); err != nil {
		t.Fatal(err)
	}

	repository := &receiptBarrierRepository{
		Repository: store,
		arrived:    make(chan struct{}, 2),
		release:    make(chan struct{}),
	}
	service := application.NewService(repository, nil)
	requests := []string{"verification-a", "verification-b"}
	errorsByRequest := make([]error, len(requests))
	start := make(chan struct{})
	var wait sync.WaitGroup
	for index, requestID := range requests {
		wait.Add(1)
		go func(index int, requestID string) {
			defer wait.Done()
			<-start
			_, errorsByRequest[index] = service.VerifyArchive(app.ID, application.VerifyArchiveCommand{RequestID: requestID, Actor: "归档员"})
		}(index, requestID)
	}
	close(start)
	<-repository.arrived
	<-repository.arrived
	close(repository.release)
	wait.Wait()
	for _, verifyErr := range errorsByRequest {
		if verifyErr != nil {
			t.Fatalf("VerifyArchive() error = %v", verifyErr)
		}
	}
}
