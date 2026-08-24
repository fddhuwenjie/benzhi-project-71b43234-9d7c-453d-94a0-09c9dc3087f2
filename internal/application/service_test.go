package application

import (
	"errors"
	"testing"

	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/domain"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/persistence"
)

func TestCreateDraft(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(store, nil)
	view, err := service.CreateDraft(CreateDraftCommand{RequestID: "create-test", Draft: validDraft()})
	if err != nil {
		t.Fatalf("CreateDraft() error = %v", err)
	}
	if view.Application.Status != domain.StatusDraft || view.Application.Revision != 1 {
		t.Fatalf("unexpected application: %+v", view.Application)
	}
}

func TestFullWorkflowIdempotencyAndArchiveFreeze(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(store, nil)
	view, err := service.CreateDraft(CreateDraftCommand{RequestID: "full-create", Draft: validDraft()})
	if err != nil {
		t.Fatal(err)
	}
	id := view.Application.ID
	meta := func(revision int, request string) CommandMeta {
		return CommandMeta{ExpectedRevision: revision, RequestID: request, Actor: "测试人员"}
	}
	view, err = service.Assess(id, AssessCommand{Meta: meta(view.Application.Revision, "assess")})
	if err != nil || !view.Application.Assessment.Passed() {
		t.Fatalf("assessment failed: %v", err)
	}
	submit := SubmitCommand{Meta: meta(view.Application.Revision, "submit")}
	view, err = service.Submit(id, submit)
	if err != nil {
		t.Fatal(err)
	}
	timelineLength := len(view.Application.Timeline)
	view, err = service.Submit(id, submit)
	if err != nil || len(view.Application.Timeline) != timelineLength {
		t.Fatalf("duplicate submit changed timeline: %v", err)
	}
	checks := make([]domain.MeasureCheck, 0, len(domain.RequiredMeasures))
	for _, code := range domain.RequiredMeasures {
		checks = append(checks, domain.MeasureCheck{Code: code, Confirmed: true})
	}
	evidence := domain.EvidenceInput{CapturedBy: "现场员", Latitude: 31.2, Longitude: 121.4, Observations: "符合要求", MeasureChecks: checks, PhotoRecords: []domain.PhotoRecord{{FileName: "site.jpg", Category: "全景"}}}
	view, err = service.CompleteSite(id, SiteCommand{Meta: meta(view.Application.Revision, "site"), Evidence: evidence})
	if err != nil {
		t.Fatal(err)
	}
	approval := domain.ReviewInput{Reviewer: "专家", Outcome: domain.OutcomeApproved, Comments: "同意迁移"}
	view, err = service.Review(id, ReviewCommand{Meta: meta(view.Application.Revision, "approve"), Review: approval})
	if err != nil {
		t.Fatal(err)
	}
	if view.Application.Archive == nil || view.Application.Archive.Digest != domain.CalculateArchiveDigest(*view.Application.Archive) {
		t.Fatal("archive digest does not cover final archive")
	}
	_, err = service.SaveDraft(id, SaveDraftCommand{Meta: meta(view.Application.Revision, "late-write"), Draft: validDraft()})
	var business *domain.BusinessError
	if !errors.As(err, &business) || business.Code != "application_archived" {
		t.Fatalf("archived write error = %v", err)
	}
}

func validDraft() domain.DraftInput {
	return domain.DraftInput{
		TreeCode: "GS-001", Species: "香樟", CurrentLocation: "A", TargetLocation: "B", MigrationReason: "保护性迁移",
		PlannedWindow:  domain.PlannedWindow{Start: "2026-11-01", End: "2026-11-03"},
		ProtectionPlan: domain.ProtectionPlan{RootRadiusMeters: 4, TransportHours: 2, TargetSoilReady: true, TargetDrainageReady: true, CanopyProtection: "树冠保护", RootBallProtection: "根球保护", TransportProtection: "运输保护", PostPlantingCare: "栽后养护", AttachedMaterials: []string{"树木现状照片", "迁移路线图", "目标地检测报告"}, EstimatedTrunkDiameter: 60},
	}
}
