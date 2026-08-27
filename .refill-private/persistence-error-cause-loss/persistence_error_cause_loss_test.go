package persistence_error_cause_loss_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/application"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/domain"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/persistence"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/web"
)

type staleLoadRepository struct {
	*persistence.Store
	application *domain.MigrationApplication
}

func (r *staleLoadRepository) Load(string) (*domain.MigrationApplication, error) {
	copy := *r.application
	return &copy, nil
}

func TestPersistenceConflictCauseSurvivesApplicationBoundary(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	baseTime := time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)
	draft := domain.DraftInput{TreeCode: "GS-ERR-1", Species: "香樟"}
	app := domain.NewApplication("app-conflict", draft, baseTime)
	if err := store.Create(app); err != nil {
		t.Fatal(err)
	}
	stale, err := store.Load(app.ID)
	if err != nil {
		t.Fatal(err)
	}
	winner, err := store.Load(app.ID)
	if err != nil {
		t.Fatal(err)
	}
	winnerDraft := draft
	winnerDraft.Species = "银杏"
	if err := winner.UpdateDraft(winnerDraft, baseTime.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(1, winner, "save_draft", "winning-request", nil); err != nil {
		t.Fatal(err)
	}

	repository := &staleLoadRepository{Store: store, application: stale}
	service := application.NewService(repository, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := web.NewServer(service, logger).Handler()

	command := application.SaveDraftCommand{
		Meta:  application.CommandMeta{ExpectedRevision: 1, RequestID: "conflict-request", Actor: "测试人员"},
		Draft: draft,
	}
	body, err := json.Marshal(command)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPut, "/api/applications/app-conflict/draft", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("expected typed persistence conflict to remain HTTP 409, got %d: %s", response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte(`"code":"revision_conflict"`)) {
		t.Fatalf("expected revision_conflict response, got %s", response.Body.String())
	}
}
