package receiptcachealias

import (
	"testing"
	"time"

	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/application"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/domain"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/persistence"
)

func TestReceiptCacheMutationDoesNotPolluteLaterDetail(t *testing.T) {
	store, err := persistence.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewService(store, nil)
	created, err := service.CreateDraft(application.CreateDraftCommand{
		RequestID: "cache-alias-create",
		Draft: domain.DraftInput{
			TreeCode: "GS-CACHE-01",
			Species:  "香樟",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt := domain.ArchiveIntegrityReceipt{
		ID:            "receipt-cache-alias",
		ApplicationID: created.Application.ID,
		ArchiveDigest: "archive-digest",
		CheckedAt:     time.Date(2026, 8, 24, 8, 0, 0, 0, time.UTC),
		Passed:        true,
		Results: []domain.IntegrityCheckResult{{
			Component: "archive",
			RecordID:  "archive-one",
			Passed:    true,
			Message:   "归档摘要复算通过",
		}},
	}
	if err := store.SaveReceipt(receipt); err != nil {
		t.Fatal(err)
	}

	first, err := service.Get(created.Application.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.LatestIntegrityReceipt == nil || len(first.LatestIntegrityReceipt.Results) != 1 {
		t.Fatalf("首次查询未返回完整性回执: %+v", first.LatestIntegrityReceipt)
	}
	first.LatestIntegrityReceipt.Results[0].Message = "被前一次查询的调用方篡改"

	second, err := service.Get(created.Application.ID)
	if err != nil {
		t.Fatal(err)
	}
	if second.LatestIntegrityReceipt.Results[0].Message != "归档摘要复算通过" {
		t.Fatalf("TestReceiptCacheMutationDoesNotPolluteLaterDetail: 后续查询被缓存别名污染，得到 %q", second.LatestIntegrityReceipt.Results[0].Message)
	}
}
