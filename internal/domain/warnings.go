package domain

import (
	"encoding/json"
	"strings"
	"time"
)

type WarningDispositionAction string

const (
	WarningActionMitigated    WarningDispositionAction = "mitigated"
	WarningActionAcknowledged WarningDispositionAction = "acknowledged"
)

type WarningDisposition struct {
	FindingCode      string                   `json:"finding_code"`
	FindingField     string                   `json:"finding_field"`
	FindingDigest    string                   `json:"finding_digest"`
	AssessmentDigest string                   `json:"assessment_digest"`
	Action           WarningDispositionAction `json:"action"`
	Note             string                   `json:"note"`
	HandledBy        string                   `json:"handled_by"`
	HandledAt        time.Time                `json:"handled_at"`
}

type WarningDispositionInput struct {
	FindingCode string                   `json:"finding_code"`
	Action      WarningDispositionAction `json:"action"`
	Note        string                   `json:"note"`
	HandledBy   string                   `json:"handled_by"`
}

func FindingDigest(f Finding) string {
	encoded, _ := json.Marshal(f)
	return Digest(string(encoded))
}

func (a *MigrationApplication) SetAssessment(assessment RuleAssessment, now time.Time) error {
	if a.Status != StatusDraft && a.Status != StatusRectifying {
		return NewState("assessment_state_invalid", "当前状态不能执行规则核查")
	}
	if assessment.ApplicationRevision != a.Revision {
		return RevisionConflict(assessment.ApplicationRevision, a.Revision)
	}
	valid := make(map[string]Finding)
	for _, finding := range assessment.WarningFindings {
		valid[FindingDigest(finding)] = finding
	}
	kept := make([]WarningDisposition, 0, len(a.WarningDispositions))
	for _, disposition := range a.WarningDispositions {
		if finding, ok := valid[disposition.FindingDigest]; ok {
			disposition.FindingCode = finding.Code
			disposition.FindingField = finding.Field
			disposition.AssessmentDigest = assessment.ResultDigest
			kept = append(kept, disposition)
		}
	}
	a.WarningDispositions = kept
	a.Assessment = &assessment
	a.bump(now)
	return nil
}

func (a *MigrationApplication) SaveWarningDisposition(input WarningDispositionInput, actor, requestID string, now time.Time) error {
	if a.Status != StatusDraft || a.Assessment == nil {
		return NewState("warning_disposition_state_invalid", "仅已核查的草稿可处置警示项")
	}
	input.FindingCode = strings.TrimSpace(input.FindingCode)
	input.Note = strings.TrimSpace(input.Note)
	input.HandledBy = strings.TrimSpace(input.HandledBy)
	if input.HandledBy == "" {
		input.HandledBy = strings.TrimSpace(actor)
	}
	if input.Action != WarningActionMitigated && input.Action != WarningActionAcknowledged {
		return NewValidation("warning_action_invalid", "action", "警示处置只能选择已采取措施或知悉风险")
	}
	if input.Note == "" {
		return NewValidation("warning_note_required", "note", "警示处置说明不能为空")
	}
	if input.HandledBy == "" {
		return NewValidation("warning_handler_required", "handled_by", "警示处置经办人不能为空")
	}
	var target *Finding
	for i := range a.Assessment.WarningFindings {
		if a.Assessment.WarningFindings[i].Code == input.FindingCode {
			target = &a.Assessment.WarningFindings[i]
			break
		}
	}
	if target == nil {
		return NewValidation("warning_finding_unknown", "finding_code", "该代码不是当前核查结果中的警示项")
	}
	disposition := WarningDisposition{FindingCode: target.Code, FindingField: target.Field, FindingDigest: FindingDigest(*target), AssessmentDigest: a.Assessment.ResultDigest, Action: input.Action, Note: input.Note, HandledBy: input.HandledBy, HandledAt: now.UTC()}
	replaced := false
	for i := range a.WarningDispositions {
		if a.WarningDispositions[i].FindingDigest == disposition.FindingDigest {
			a.WarningDispositions[i] = disposition
			replaced = true
			break
		}
	}
	if !replaced {
		a.WarningDispositions = append(a.WarningDispositions, disposition)
	}
	a.bump(now)
	a.Timeline = append(a.Timeline, StatusEvent{ID: NewID("evt", now, requestID), ApplicationID: a.ID, From: a.Status, To: a.Status, Action: "保存核查警示处置", Actor: input.HandledBy, RequestID: requestID, At: now.UTC()})
	return nil
}

func (a *MigrationApplication) MissingWarningDispositions() []Finding {
	if a.Assessment == nil {
		return nil
	}
	handled := make(map[string]bool)
	for _, item := range a.WarningDispositions {
		if item.AssessmentDigest == a.Assessment.ResultDigest {
			handled[item.FindingDigest] = true
		}
	}
	missing := make([]Finding, 0)
	for _, finding := range a.Assessment.WarningFindings {
		if !handled[FindingDigest(finding)] {
			missing = append(missing, finding)
		}
	}
	return missing
}
