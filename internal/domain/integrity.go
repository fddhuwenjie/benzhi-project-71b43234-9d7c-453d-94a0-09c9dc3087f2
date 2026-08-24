package domain

import (
	"encoding/json"
	"strconv"
	"time"
)

type IntegrityEvent struct {
	ID        string `json:"id"`
	Revision  int    `json:"revision"`
	Operation string `json:"operation"`
	Status    Status `json:"status"`
}

type IntegrityCheckResult struct {
	Component string `json:"component"`
	RecordID  string `json:"record_id,omitempty"`
	Passed    bool   `json:"passed"`
	Message   string `json:"message"`
}

type ArchiveIntegrityReceipt struct {
	ID            string                 `json:"id"`
	ApplicationID string                 `json:"application_id"`
	ArchiveDigest string                 `json:"archive_digest"`
	CheckedAt     time.Time              `json:"checked_at"`
	Passed        bool                   `json:"passed"`
	Results       []IntegrityCheckResult `json:"results"`
}

func VerifyArchiveIntegrity(a *MigrationApplication, events []IntegrityEvent, now time.Time) (ArchiveIntegrityReceipt, error) {
	if a.Status != StatusArchived || a.Archive == nil {
		return ArchiveIntegrityReceipt{}, NewState("archive_unavailable", "仅已归档申请可执行完整性核验")
	}
	receipt := ArchiveIntegrityReceipt{ID: NewID("receipt", now, a.ID), ApplicationID: a.ID, ArchiveDigest: a.Archive.Digest, CheckedAt: now.UTC(), Passed: true}
	add := func(component, recordID string, passed bool, message string) {
		receipt.Results = append(receipt.Results, IntegrityCheckResult{Component: component, RecordID: recordID, Passed: passed, Message: message})
		if !passed {
			receipt.Passed = false
		}
	}
	add("archive", a.Archive.ID, CalculateArchiveDigest(*a.Archive) == a.Archive.Digest, "归档摘要复算")
	rulePassed := a.Assessment != nil && CalculateAssessmentResultDigest(*a.Assessment) == a.Assessment.ResultDigest && a.Assessment.ResultDigest == a.Archive.RuleResultDigest
	add("rules", func() string {
		if a.Assessment == nil {
			return ""
		}
		return a.Assessment.ID
	}(), rulePassed, "最终规则结果摘要")
	archivedEvidence := make(map[string]string)
	for _, item := range a.Archive.EvidenceItems {
		archivedEvidence[item.EvidenceID] = item.Digest
	}
	currentEvidence := make(map[string]bool)
	for _, evidence := range a.Evidence {
		currentEvidence[evidence.ID] = true
		add("evidence", evidence.ID, CalculateEvidenceDigest(evidence) == evidence.ContentDigest && archivedEvidence[evidence.ID] == evidence.ContentDigest, "现场证据摘要")
	}
	for _, item := range a.Archive.EvidenceItems {
		if !currentEvidence[item.EvidenceID] {
			add("evidence", item.EvidenceID, false, "归档引用的现场证据缺失")
		}
	}
	timelineRaw, _ := json.Marshal(a.Archive.Timeline)
	currentRaw, _ := json.Marshal(a.Timeline)
	add("timeline", a.ID, string(timelineRaw) == string(currentRaw), "状态时间线")
	approvedEvent := false
	for _, event := range events {
		if event.Operation == "review_approve" && event.Status == StatusArchived && event.Revision == a.Archive.ApprovedRevision {
			approvedEvent = true
		}
	}
	add("approval_event", a.ID, approvedEvent, "批准事件、归档状态与批准修订")
	snapshot := CurrentLockedSnapshot(a)
	snapshotPassed := snapshot != nil && snapshot.ID == a.Archive.LockedSnapshotID && snapshot.ContentDigest == a.Archive.LockedSnapshotDigest && ValidateLockedSnapshot(*snapshot) == nil
	add("locked_snapshot", a.Archive.LockedSnapshotID, snapshotPassed, "锁定方案快照")
	receipt.ID = NewID("receipt", now, a.ID+a.Archive.Digest+strconv.FormatBool(receipt.Passed))
	return receipt, nil
}
