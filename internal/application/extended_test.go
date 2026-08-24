package application

import (
	"errors"
	"sync"
	"testing"

	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/domain"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/persistence"
)

func TestConcurrentDuplicateDraftOnlyCreatesOneApplication(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(store, nil)
	commands := []CreateDraftCommand{{RequestID: "duplicate-a", Draft: validDraft()}, {RequestID: "duplicate-b", Draft: validDraft()}}
	errorsByRequest := make([]error, len(commands))
	var wait sync.WaitGroup
	for index := range commands {
		wait.Add(1)
		go func(index int) { defer wait.Done(); _, errorsByRequest[index] = service.CreateDraft(commands[index]) }(index)
	}
	wait.Wait()
	successes, conflicts := 0, 0
	for _, err := range errorsByRequest {
		if err == nil {
			successes++
			continue
		}
		var business *domain.BusinessError
		if errors.As(err, &business) && business.Code == "active_application_duplicate" && business.Details["application_id"] != "" {
			conflicts++
			continue
		}
		t.Fatalf("unexpected create error: %v", err)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
	apps, err := service.List()
	if err != nil || len(apps) != 1 {
		t.Fatalf("applications=%d error=%v", len(apps), err)
	}
	events, err := store.Events(apps[0].ID)
	if err != nil || len(events) != 1 {
		t.Fatalf("events=%d error=%v", len(events), err)
	}
}

func TestWarningsEvidenceMatrixAndArchiveReceipt(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(store, nil)
	draft := validDraft()
	draft.PlannedWindow = domain.PlannedWindow{Start: "2026-07-01", End: "2026-07-03"}
	draft.ProtectionPlan.TransportHours = 5
	view, err := service.CreateDraft(CreateDraftCommand{RequestID: "extended-create", Draft: draft})
	if err != nil {
		t.Fatal(err)
	}
	id := view.Application.ID
	meta := func(request string) CommandMeta {
		return CommandMeta{ExpectedRevision: view.Application.Revision, RequestID: request, Actor: "测试人员"}
	}
	view, err = service.Assess(id, AssessCommand{Meta: meta("extended-assess")})
	if err != nil || len(view.Application.Assessment.WarningFindings) != 2 {
		t.Fatalf("warnings=%v error=%v", view.Application.Assessment, err)
	}
	for index, finding := range view.Application.Assessment.WarningFindings {
		view, err = service.SaveWarningDisposition(id, WarningDispositionCommand{Meta: meta("warning-" + finding.Code), Disposition: domain.WarningDispositionInput{FindingCode: finding.Code, Action: domain.WarningActionMitigated, Note: "已落实专项保护措施", HandledBy: "审查员"}})
		if err != nil {
			t.Fatal(err)
		}
		if index == 0 {
			_, submitErr := service.Submit(id, SubmitCommand{Meta: meta("premature-submit")})
			var business *domain.BusinessError
			if !errors.As(submitErr, &business) || business.Code != "warning_disposition_missing" {
				t.Fatalf("submit error=%v", submitErr)
			}
		}
	}
	submit := SubmitCommand{Meta: meta("extended-submit")}
	view, err = service.Submit(id, submit)
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.Submit(id, submit)
	if err != nil || len(view.Application.LockedSnapshots) != 1 {
		t.Fatalf("duplicate submit snapshots=%d error=%v", len(view.Application.LockedSnapshots), err)
	}

	photos := make([]domain.PhotoRecord, 0, len(domain.RequiredPhotoCategories))
	for index, category := range domain.RequiredPhotoCategories {
		photos = append(photos, domain.PhotoRecord{FileName: category + ".jpg", Category: category, TakenAt: "2026-07-01T08:0" + string(rune('0'+index))})
	}
	checks := make([]domain.MeasureCheck, 0, len(domain.RequiredMeasures))
	for _, code := range domain.RequiredMeasures {
		checks = append(checks, domain.MeasureCheck{Code: code, Confirmed: true})
	}
	evidence := domain.EvidenceInput{CapturedBy: "现场员", Latitude: 31.2, Longitude: 121.4, Observations: "现场条件符合要求", MeasureChecks: checks, PhotoRecords: photos[:len(photos)-1]}
	view, err = service.SaveSiteDraft(id, SiteDraftCommand{Meta: meta("site-partial"), Evidence: evidence})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ConfirmSite(id, ConfirmSiteCommand{Meta: meta("site-incomplete")})
	var incomplete *domain.BusinessError
	if !errors.As(err, &incomplete) || incomplete.Code != "site_evidence_incomplete" {
		t.Fatalf("confirm error=%v", err)
	}
	evidence.PhotoRecords = photos
	view, err = service.SaveSiteDraft(id, SiteDraftCommand{Meta: meta("site-complete"), Evidence: evidence})
	if err != nil {
		t.Fatal(err)
	}
	view, err = service.ConfirmSite(id, ConfirmSiteCommand{Meta: meta("site-confirm")})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Review(id, ReviewCommand{Meta: meta("matrix-incomplete"), Review: domain.ReviewInput{Reviewer: "专家", Outcome: domain.OutcomeApproved, Comments: "同意", Matrix: []domain.ReviewMatrixInput{}}})
	var matrixError *domain.BusinessError
	if !errors.As(err, &matrixError) || matrixError.Code != "review_matrix_incomplete" {
		t.Fatalf("matrix error=%v", err)
	}
	matrix := make([]domain.ReviewMatrixInput, 0, len(view.ReviewMatrix))
	for _, item := range view.ReviewMatrix {
		matrix = append(matrix, domain.ReviewMatrixInput{ID: item.ID, Conclusion: domain.MatrixPassed, ExpertNote: "逐项核验通过", EvidenceReferences: []string{item.SourceID}})
	}
	view, err = service.Review(id, ReviewCommand{Meta: meta("matrix-approve"), Review: domain.ReviewInput{Reviewer: "专家", Outcome: domain.OutcomeApproved, Comments: "同意归档", Matrix: matrix}})
	if err != nil {
		t.Fatal(err)
	}
	revision := view.Application.Revision
	receipt, err := service.VerifyArchive(id, VerifyArchiveCommand{RequestID: "integrity-one", Actor: "归档员"})
	if err != nil || !receipt.Passed {
		t.Fatalf("receipt=%+v error=%v", receipt, err)
	}
	view, err = service.Get(id)
	if err != nil || view.Application.Revision != revision || view.LatestIntegrityReceipt == nil {
		t.Fatalf("revision changed or receipt missing: %v", err)
	}
	newView, err := service.CreateDraft(CreateDraftCommand{RequestID: "after-archive", Draft: draft})
	if err != nil || len(newView.HistoricalArchives) != 1 || newView.HistoricalArchives[0].ApplicationID != id {
		t.Fatalf("historical archive missing: %+v error=%v", newView.HistoricalArchives, err)
	}
	retried, err := service.CreateDraft(CreateDraftCommand{RequestID: "after-archive", Draft: domain.DraftInput{TreeCode: "OTHER"}})
	if err != nil || retried.Application.ID != newView.Application.ID || retried.Application.TreeCode != draft.TreeCode {
		t.Fatalf("create idempotency changed result: %+v error=%v", retried.Application, err)
	}
}
