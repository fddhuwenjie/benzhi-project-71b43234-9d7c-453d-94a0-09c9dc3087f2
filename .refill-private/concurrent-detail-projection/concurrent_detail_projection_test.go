package concurrentdetailprojection

import (
	"testing"
	"time"

	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/application"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/domain"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/persistence"
)

type stagedEventsRepository struct {
	application.Repository
	blockedID string
	entered   chan struct{}
	release   chan struct{}
}

func (r *stagedEventsRepository) Events(id string) ([]persistence.EventRecord, error) {
	if id == r.blockedID {
		r.entered <- struct{}{}
		<-r.release
	}
	return r.Repository.Events(id)
}

func TestConcurrentDetailQueriesKeepRequestProjectionIsolated(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
	first := domain.NewApplication("app-first", domain.DraftInput{TreeCode: "TREE-FIRST", Species: "香樟"}, now)
	second := domain.NewApplication("app-second", domain.DraftInput{TreeCode: "TREE-SECOND", Species: "银杏"}, now.Add(time.Minute))
	if err := store.Create(first); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(second); err != nil {
		t.Fatal(err)
	}

	repository := &stagedEventsRepository{
		Repository: store,
		blockedID:  first.ID,
		entered:    make(chan struct{}),
		release:    make(chan struct{}),
	}
	service := application.NewService(repository, nil)
	type result struct {
		view application.DetailView
		err  error
	}
	firstResult := make(chan result, 1)
	go func() {
		view, err := service.Get(first.ID)
		firstResult <- result{view: view, err: err}
	}()

	<-repository.entered
	secondView, err := service.Get(second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if secondView.Application.ID != second.ID {
		t.Fatalf("second query returned application %q", secondView.Application.ID)
	}
	close(repository.release)
	got := <-firstResult
	if got.err != nil {
		t.Fatal(got.err)
	}
	if got.view.Application.ID != first.ID {
		t.Fatalf("concurrent detail projection leaked: first query returned %q, want %q", got.view.Application.ID, first.ID)
	}
	if len(got.view.Events) != 1 || got.view.Events[0].ApplicationID != first.ID {
		t.Fatalf("first query events do not match application: %+v", got.view.Events)
	}
}
