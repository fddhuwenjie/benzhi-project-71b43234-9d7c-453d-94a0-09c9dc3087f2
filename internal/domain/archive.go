package domain

import (
	"encoding/json"
	"strconv"
	"time"
)

type StatusEvent struct {
	ID            string    `json:"id"`
	ApplicationID string    `json:"application_id"`
	From          Status    `json:"from,omitempty"`
	To            Status    `json:"to"`
	Action        string    `json:"action"`
	Actor         string    `json:"actor,omitempty"`
	RequestID     string    `json:"request_id,omitempty"`
	At            time.Time `json:"at"`
}

type ArchiveEvidenceItem struct {
	EvidenceID string   `json:"evidence_id"`
	Digest     string   `json:"digest"`
	Photos     []string `json:"photos"`
}

type ArchiveSummary struct {
	ID                   string                `json:"id"`
	ApplicationID        string                `json:"application_id"`
	TreeCode             string                `json:"tree_code"`
	Species              string                `json:"species"`
	CurrentLocation      string                `json:"current_location"`
	TargetLocation       string                `json:"target_location"`
	MigrationReason      string                `json:"migration_reason"`
	ApprovedRevision     int                   `json:"approved_revision"`
	RuleResultDigest     string                `json:"rule_result_digest"`
	EvidenceItems        []ArchiveEvidenceItem `json:"evidence_items"`
	Timeline             []StatusEvent         `json:"timeline"`
	RectificationDiffs   []string              `json:"rectification_diffs"`
	WarningDispositions  []WarningDisposition  `json:"warning_dispositions,omitempty"`
	FinalReviewMatrix    []ReviewMatrixItem    `json:"final_review_matrix,omitempty"`
	LockedSnapshotID     string                `json:"locked_snapshot_id"`
	LockedSnapshotDigest string                `json:"locked_snapshot_digest"`
	ArchivedAt           time.Time             `json:"archived_at"`
	Digest               string                `json:"digest"`
}

func BuildArchive(a *MigrationApplication, now time.Time) (ArchiveSummary, error) {
	if a.Status != StatusReview {
		return ArchiveSummary{}, NewState("archive_state_invalid", "仅待专家复核申请可归档")
	}
	if a.Assessment == nil || !a.Assessment.Passed() {
		return ArchiveSummary{}, NewState("archive_assessment_invalid", "归档前规则核查必须通过")
	}
	if len(a.Evidence) == 0 {
		return ArchiveSummary{}, NewState("archive_evidence_missing", "归档前必须存在现场证据")
	}
	items := make([]ArchiveEvidenceItem, 0, len(a.Evidence))
	for _, evidence := range a.Evidence {
		photos := make([]string, 0, len(evidence.PhotoRecords))
		for _, photo := range evidence.PhotoRecords {
			photos = append(photos, photo.FileName+"（"+photo.Category+"）")
		}
		items = append(items, ArchiveEvidenceItem{EvidenceID: evidence.ID, Digest: evidence.ContentDigest, Photos: photos})
	}
	diffs := make([]string, 0)
	for _, correction := range a.Rectifications {
		diffs = append(diffs, correction.DifferenceSummary...)
	}
	snapshot := CurrentLockedSnapshot(a)
	if snapshot == nil {
		return ArchiveSummary{}, NewState("locked_snapshot_missing", "归档前必须存在有效锁定方案")
	}
	archive := ArchiveSummary{ID: NewID("archive", now, a.ID), ApplicationID: a.ID, TreeCode: a.TreeCode, Species: a.Species, CurrentLocation: a.CurrentLocation, TargetLocation: a.TargetLocation, MigrationReason: a.MigrationReason, ApprovedRevision: a.Revision + 1, RuleResultDigest: a.Assessment.ResultDigest, EvidenceItems: items, Timeline: append([]StatusEvent(nil), a.Timeline...), RectificationDiffs: diffs, WarningDispositions: append([]WarningDisposition(nil), a.WarningDispositions...), LockedSnapshotID: snapshot.ID, LockedSnapshotDigest: snapshot.ContentDigest, ArchivedAt: now.UTC()}
	if len(a.Reviews) > 0 {
		archive.FinalReviewMatrix = append([]ReviewMatrixItem(nil), a.Reviews[len(a.Reviews)-1].Matrix...)
	}
	archive.Digest = CalculateArchiveDigest(archive)
	return archive, nil
}

func CalculateArchiveDigest(archive ArchiveSummary) string {
	archive.Digest = ""
	encoded, _ := json.Marshal(archive)
	return Digest(string(encoded), strconv.Itoa(archive.ApprovedRevision))
}
