package persistence

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/domain"
)

func TestStoreConflictIdempotencyAndRecovery(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	app := domain.NewApplication("app-one", domain.DraftInput{TreeCode: "GS-1"}, time.Now())
	if err := store.Create(app); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(app.ID)
	if err != nil {
		t.Fatal(err)
	}
	loaded.TreeCode = "GS-2"
	loaded.Revision++
	result, err := store.Save(1, loaded, "save", "req-one", json.RawMessage(`{"ok":true}`))
	if err != nil {
		t.Fatal(err)
	}
	loaded.Revision++
	if _, err := store.Save(1, loaded, "other", "req-two", nil); err == nil {
		t.Fatal("expected revision conflict")
	} else {
		var conflict *ConflictError
		if !errors.As(err, &conflict) || conflict.Actual != 2 {
			t.Fatalf("unexpected conflict: %v", err)
		}
	}
	previous, ok, err := store.PreviousResult(app.ID, "save", "req-one")
	if err != nil || !ok || previous.Revision != result.Revision {
		t.Fatalf("idempotency result missing: %+v %v", previous, err)
	}
	reopened, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := reopened.Load(app.ID)
	if err != nil || recovered.TreeCode != "GS-2" {
		t.Fatalf("recovery failed: %+v %v", recovered, err)
	}
	events, err := reopened.Events(app.ID)
	if err != nil || len(events) != 2 {
		t.Fatalf("events = %d, error = %v", len(events), err)
	}
}

func TestOpenRejectsCorruptSnapshot(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	app := domain.NewApplication("corrupt", domain.DraftInput{}, time.Now())
	if err := store.Create(app); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "snapshots", "corrupt.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)/2] ^= 1
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(dir); err == nil {
		t.Fatal("expected digest validation failure")
	}
}
