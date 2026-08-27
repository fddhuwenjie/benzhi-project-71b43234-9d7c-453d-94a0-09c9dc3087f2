package snapshotrollbackdataloss_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/application"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/domain"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/persistence"
)

func TestSnapshotRollbackPreservesLastCommittedApplication(t *testing.T) {
	dataDir := t.TempDir()
	store, err := persistence.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store, nil)
	created, err := service.CreateDraft(application.CreateDraftCommand{
		RequestID: "create-before-event-failure",
		Draft:     validDraft("原目标位置"),
	})
	if err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}

	eventPath := filepath.Join(dataDir, "events.jsonl")
	if err := os.Remove(eventPath); err != nil {
		t.Fatalf("remove active event file: %v", err)
	}
	if err := os.Mkdir(eventPath, 0o700); err != nil {
		t.Fatalf("invalidate active event path: %v", err)
	}

	_, err = service.SaveDraft(created.Application.ID, application.SaveDraftCommand{
		Meta: application.CommandMeta{
			ExpectedRevision: created.Application.Revision,
			RequestID:        "save-with-invalid-event-resource",
			Actor:            "测试人员",
		},
		Draft: validDraft("新目标位置"),
	})
	if err == nil {
		t.Fatal("SaveDraft() unexpectedly succeeded after event resource invalidation")
	}

	restored, loadErr := store.Load(created.Application.ID)
	if errors.Is(loadErr, persistence.ErrNotFound) {
		t.Fatalf("failed save deleted the last committed application: %v", loadErr)
	}
	if loadErr != nil {
		t.Fatalf("Load() after failed save error = %v", loadErr)
	}
	if restored.Revision != created.Application.Revision || restored.TargetLocation != "原目标位置" {
		t.Fatalf("failed save changed committed snapshot: revision=%d target=%q", restored.Revision, restored.TargetLocation)
	}
}

func validDraft(target string) domain.DraftInput {
	return domain.DraftInput{
		TreeCode:        "GS-ROLLBACK-001",
		Species:         "香樟",
		CurrentLocation: "原址",
		TargetLocation:  target,
		MigrationReason: "保护性迁移",
		PlannedWindow: domain.PlannedWindow{
			Start: "2026-11-01",
			End:   "2026-11-03",
		},
		ProtectionPlan: domain.ProtectionPlan{
			RootRadiusMeters:       4,
			TransportHours:         2,
			TargetSoilReady:        true,
			TargetDrainageReady:    true,
			CanopyProtection:       "树冠固定",
			RootBallProtection:     "根球包扎",
			TransportProtection:    "全程保湿",
			PostPlantingCare:       "连续养护",
			AttachedMaterials:      []string{"树木现状照片", "迁移路线图", "目标地检测报告"},
			EstimatedTrunkDiameter: 60,
		},
	}
}
