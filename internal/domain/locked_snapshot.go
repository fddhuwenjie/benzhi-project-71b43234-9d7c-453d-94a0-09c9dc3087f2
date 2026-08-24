package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

type LockedPlanContent struct {
	TreeCode            string               `json:"tree_code"`
	Species             string               `json:"species"`
	CurrentLocation     string               `json:"current_location"`
	TargetLocation      string               `json:"target_location"`
	MigrationReason     string               `json:"migration_reason"`
	PlannedWindow       PlannedWindow        `json:"planned_window"`
	ProtectionPlan      ProtectionPlan       `json:"protection_plan"`
	Assessment          RuleAssessment       `json:"assessment"`
	WarningDispositions []WarningDisposition `json:"warning_dispositions"`
}

type LockedPlanSnapshot struct {
	ID                string            `json:"id"`
	ApplicationID     string            `json:"application_id"`
	SubmittedRevision int               `json:"submitted_revision"`
	Content           LockedPlanContent `json:"content"`
	ContentDigest     string            `json:"content_digest"`
	LockedAt          time.Time         `json:"locked_at"`
}

type SnapshotDifference struct {
	Field        string `json:"field"`
	LockedValue  any    `json:"locked_value"`
	CurrentValue any    `json:"current_value"`
}

func BuildLockedSnapshot(a *MigrationApplication, requestID string, now time.Time) (LockedPlanSnapshot, error) {
	if a.Assessment == nil {
		return LockedPlanSnapshot{}, NewState("assessment_missing", "尚未形成可锁定的规则核查结果")
	}
	content := LockedPlanContent{TreeCode: a.TreeCode, Species: a.Species, CurrentLocation: a.CurrentLocation, TargetLocation: a.TargetLocation, MigrationReason: a.MigrationReason, PlannedWindow: a.PlannedWindow, ProtectionPlan: NormalizePlan(a.ProtectionPlan), Assessment: *a.Assessment, WarningDispositions: append([]WarningDisposition(nil), a.WarningDispositions...)}
	snapshot := LockedPlanSnapshot{ID: NewID("snapshot", now, a.ID+requestID), ApplicationID: a.ID, SubmittedRevision: a.Revision, Content: content, LockedAt: now.UTC()}
	snapshot.ContentDigest = calculateLockedSnapshotDigest(snapshot)
	return snapshot, nil
}

func calculateLockedSnapshotDigest(snapshot LockedPlanSnapshot) string {
	snapshot.ContentDigest = ""
	encoded, _ := json.Marshal(snapshot)
	return Digest(string(encoded))
}

func ValidateLockedSnapshot(snapshot LockedPlanSnapshot) error {
	if snapshot.ID == "" || snapshot.ContentDigest == "" || snapshot.ContentDigest != calculateLockedSnapshotDigest(snapshot) {
		return NewState("locked_snapshot_integrity_error", fmt.Sprintf("锁定方案快照 %s 完整性校验失败", snapshot.ID))
	}
	return nil
}

func ValidateLockedSnapshots(a *MigrationApplication) error {
	for _, snapshot := range a.LockedSnapshots {
		if err := ValidateLockedSnapshot(snapshot); err != nil {
			return err
		}
	}
	return nil
}

func CurrentLockedSnapshot(a *MigrationApplication) *LockedPlanSnapshot {
	for i := len(a.LockedSnapshots) - 1; i >= 0; i-- {
		if a.LockedSnapshots[i].ID == a.SubmittedSnapshotID {
			copy := a.LockedSnapshots[i]
			return &copy
		}
	}
	return nil
}

func LockedSnapshotDifferences(a *MigrationApplication) []SnapshotDifference {
	snapshot := CurrentLockedSnapshot(a)
	if snapshot == nil {
		return nil
	}
	current := LockedPlanContent{TreeCode: a.TreeCode, Species: a.Species, CurrentLocation: a.CurrentLocation, TargetLocation: a.TargetLocation, MigrationReason: a.MigrationReason, PlannedWindow: a.PlannedWindow, ProtectionPlan: NormalizePlan(a.ProtectionPlan)}
	locked := snapshot.Content
	pairs := []struct {
		field         string
		before, after any
	}{
		{"tree_code", locked.TreeCode, current.TreeCode}, {"species", locked.Species, current.Species},
		{"current_location", locked.CurrentLocation, current.CurrentLocation}, {"target_location", locked.TargetLocation, current.TargetLocation},
		{"migration_reason", locked.MigrationReason, current.MigrationReason}, {"planned_window", locked.PlannedWindow, current.PlannedWindow},
		{"protection_plan", locked.ProtectionPlan, current.ProtectionPlan},
	}
	diffs := make([]SnapshotDifference, 0)
	for _, pair := range pairs {
		left, _ := json.Marshal(pair.before)
		right, _ := json.Marshal(pair.after)
		if string(left) != string(right) {
			diffs = append(diffs, SnapshotDifference{Field: pair.field, LockedValue: pair.before, CurrentValue: pair.after})
		}
	}
	return diffs
}
