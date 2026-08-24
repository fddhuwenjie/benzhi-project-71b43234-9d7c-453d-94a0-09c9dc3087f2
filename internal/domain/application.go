package domain

import (
	"strings"
	"time"
)

type Status string

const (
	StatusDraft       Status = "draft"
	StatusSitePending Status = "site_pending"
	StatusReview      Status = "expert_review"
	StatusRectifying  Status = "rectifying"
	StatusArchived    Status = "archived"
)

type PlannedWindow struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type ProtectionPlan struct {
	RootRadiusMeters       float64  `json:"root_radius_meters"`
	TransportHours         float64  `json:"transport_hours"`
	TargetSoilReady        bool     `json:"target_soil_ready"`
	TargetDrainageReady    bool     `json:"target_drainage_ready"`
	CanopyProtection       string   `json:"canopy_protection"`
	RootBallProtection     string   `json:"root_ball_protection"`
	TransportProtection    string   `json:"transport_protection"`
	PostPlantingCare       string   `json:"post_planting_care"`
	AttachedMaterials      []string `json:"attached_materials"`
	EstimatedTrunkDiameter float64  `json:"estimated_trunk_diameter_cm"`
}

type MigrationApplication struct {
	ID                  string               `json:"id"`
	TreeCode            string               `json:"tree_code"`
	Species             string               `json:"species"`
	CurrentLocation     string               `json:"current_location"`
	TargetLocation      string               `json:"target_location"`
	MigrationReason     string               `json:"migration_reason"`
	PlannedWindow       PlannedWindow        `json:"planned_window"`
	ProtectionPlan      ProtectionPlan       `json:"protection_plan"`
	Status              Status               `json:"status"`
	Revision            int                  `json:"revision"`
	SubmittedSnapshotID string               `json:"submitted_snapshot_id,omitempty"`
	CreatedAt           time.Time            `json:"created_at"`
	UpdatedAt           time.Time            `json:"updated_at"`
	Assessment          *RuleAssessment      `json:"assessment,omitempty"`
	WarningDispositions []WarningDisposition `json:"warning_dispositions,omitempty"`
	LockedSnapshots     []LockedPlanSnapshot `json:"locked_snapshots,omitempty"`
	EvidenceDraft       *EvidenceDraft       `json:"evidence_draft,omitempty"`
	Evidence            []SiteEvidence       `json:"evidence,omitempty"`
	Reviews             []ReviewDecision     `json:"reviews,omitempty"`
	Rectifications      []Rectification      `json:"rectifications,omitempty"`
	Timeline            []StatusEvent        `json:"timeline,omitempty"`
	Archive             *ArchiveSummary      `json:"archive,omitempty"`
}

type DraftInput struct {
	TreeCode        string         `json:"tree_code"`
	Species         string         `json:"species"`
	CurrentLocation string         `json:"current_location"`
	TargetLocation  string         `json:"target_location"`
	MigrationReason string         `json:"migration_reason"`
	PlannedWindow   PlannedWindow  `json:"planned_window"`
	ProtectionPlan  ProtectionPlan `json:"protection_plan"`
}

func NewApplication(id string, input DraftInput, now time.Time) *MigrationApplication {
	return &MigrationApplication{
		ID: id, Status: StatusDraft, Revision: 1, CreatedAt: now.UTC(), UpdatedAt: now.UTC(),
		Timeline: []StatusEvent{{ID: NewID("evt", now, id), ApplicationID: id, From: "", To: StatusDraft, Action: "创建草稿", At: now.UTC()}},
		TreeCode: NormalizeTreeCode(input.TreeCode), Species: strings.TrimSpace(input.Species),
		CurrentLocation: strings.TrimSpace(input.CurrentLocation), TargetLocation: strings.TrimSpace(input.TargetLocation),
		MigrationReason: strings.TrimSpace(input.MigrationReason), PlannedWindow: input.PlannedWindow,
		ProtectionPlan: NormalizePlan(input.ProtectionPlan),
	}
}

func (a *MigrationApplication) EnsureMutable(expected int) error {
	if a.Status == StatusArchived {
		return NewState("application_archived", "申请已归档，禁止任何业务修改")
	}
	if expected != a.Revision {
		return RevisionConflict(expected, a.Revision)
	}
	return nil
}

func (a *MigrationApplication) UpdateDraft(input DraftInput, now time.Time) error {
	if a.Status != StatusDraft && a.Status != StatusRectifying {
		return NewState("draft_not_editable", "当前状态不允许编辑方案")
	}
	a.TreeCode = NormalizeTreeCode(input.TreeCode)
	a.Species = strings.TrimSpace(input.Species)
	a.CurrentLocation = strings.TrimSpace(input.CurrentLocation)
	a.TargetLocation = strings.TrimSpace(input.TargetLocation)
	a.MigrationReason = strings.TrimSpace(input.MigrationReason)
	a.PlannedWindow = input.PlannedWindow
	a.ProtectionPlan = NormalizePlan(input.ProtectionPlan)
	a.Assessment = nil
	a.bump(now)
	return nil
}

func NormalizeTreeCode(value string) string { return strings.TrimSpace(value) }

func IsActiveStatus(status Status) bool { return status != StatusArchived }

func (a *MigrationApplication) transition(to Status, action, actor, requestID string, now time.Time) error {
	allowed := map[Status]map[Status]bool{
		StatusDraft:       {StatusSitePending: true},
		StatusSitePending: {StatusReview: true},
		StatusReview:      {StatusRectifying: true, StatusArchived: true},
		StatusRectifying:  {StatusReview: true},
	}
	if !allowed[a.Status][to] {
		return NewState("invalid_transition", "当前状态不能执行该推进操作")
	}
	from := a.Status
	a.Status = to
	a.bump(now)
	a.Timeline = append(a.Timeline, StatusEvent{ID: NewID("evt", now, requestID), ApplicationID: a.ID, From: from, To: to, Action: action, Actor: strings.TrimSpace(actor), RequestID: requestID, At: now.UTC()})
	return nil
}

func (a *MigrationApplication) bump(now time.Time) {
	a.Revision++
	a.UpdatedAt = now.UTC()
}

func NormalizePlan(plan ProtectionPlan) ProtectionPlan {
	plan.CanopyProtection = strings.TrimSpace(plan.CanopyProtection)
	plan.RootBallProtection = strings.TrimSpace(plan.RootBallProtection)
	plan.TransportProtection = strings.TrimSpace(plan.TransportProtection)
	plan.PostPlantingCare = strings.TrimSpace(plan.PostPlantingCare)
	plan.AttachedMaterials = SortedUnique(plan.AttachedMaterials)
	return plan
}
