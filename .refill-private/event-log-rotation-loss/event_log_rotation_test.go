package eventlogrotation_test

import (
	"os"
	"path/filepath"
	"testing"

	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/application"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/domain"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/persistence"
)

func TestEventLogRotationKeepsSubsequentStateEvent(t *testing.T) {
	dir := t.TempDir()
	store, err := persistence.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store, nil)
	original := draft("香樟")
	view, err := service.CreateDraft(application.CreateDraftCommand{RequestID: "rotation-create", Draft: original})
	if err != nil {
		t.Fatal(err)
	}

	eventPath := filepath.Join(dir, "events.jsonl")
	rotatedPath := filepath.Join(dir, "events-previous.jsonl")
	initialEvents, err := os.ReadFile(eventPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(eventPath, rotatedPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(eventPath, initialEvents, 0o600); err != nil {
		t.Fatal(err)
	}

	updated := draft("银杏")
	view, err = service.SaveDraft(view.Application.ID, application.SaveDraftCommand{
		Meta:  application.CommandMeta{ExpectedRevision: view.Application.Revision, RequestID: "rotation-save", Actor: "审查员"},
		Draft: updated,
	})
	if err != nil {
		t.Fatalf("SaveDraft() error = %v", err)
	}
	if view.Application.Revision != 2 || view.Application.Species != "银杏" {
		t.Fatalf("updated snapshot = %+v", view.Application)
	}

	reopened, err := persistence.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := reopened.Load(view.Application.ID)
	if err != nil {
		t.Fatal(err)
	}
	events, err := reopened.Events(view.Application.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.RequestID == "rotation-save" {
			return
		}
	}
	t.Fatalf("revision %d was recovered but request %q is absent from the active event log", recovered.Revision, "rotation-save")
}

func draft(species string) domain.DraftInput {
	return domain.DraftInput{
		TreeCode:        "GS-ROTATION-001",
		Species:         species,
		CurrentLocation: "原址",
		TargetLocation:  "迁移目标地",
		MigrationReason: "保护性迁移",
		PlannedWindow:   domain.PlannedWindow{Start: "2026-11-01", End: "2026-11-03"},
		ProtectionPlan: domain.ProtectionPlan{
			RootRadiusMeters: 4, TransportHours: 2, TargetSoilReady: true, TargetDrainageReady: true,
			CanopyProtection: "树冠保护", RootBallProtection: "根球保护", TransportProtection: "运输保护",
			PostPlantingCare: "栽后养护", AttachedMaterials: []string{"树木现状照片", "迁移路线图", "目标地检测报告"},
		},
	}
}
