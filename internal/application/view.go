package application

import (
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/domain"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/persistence"
)

type ApplicationSummary struct {
	ID             string        `json:"id"`
	TreeCode       string        `json:"tree_code"`
	Species        string        `json:"species"`
	TargetLocation string        `json:"target_location"`
	Status         domain.Status `json:"status"`
	Revision       int           `json:"revision"`
	UpdatedAt      string        `json:"updated_at"`
}

type DetailView struct {
	Application            *domain.MigrationApplication    `json:"application"`
	Events                 []persistence.EventRecord       `json:"persistence_events"`
	OpenRectifications     []domain.RectificationItem      `json:"open_rectifications"`
	CanEdit                bool                            `json:"can_edit"`
	CanAssess              bool                            `json:"can_assess"`
	CanSubmit              bool                            `json:"can_submit"`
	CanSite                bool                            `json:"can_site"`
	CanReview              bool                            `json:"can_review"`
	CanRectify             bool                            `json:"can_rectify"`
	HistoricalArchives     []HistoricalArchiveSummary      `json:"historical_archives"`
	LockedSnapshot         *domain.LockedPlanSnapshot      `json:"locked_snapshot,omitempty"`
	SnapshotDifferences    []domain.SnapshotDifference     `json:"snapshot_differences"`
	EvidenceProgress       domain.EvidenceProgress         `json:"evidence_progress"`
	ReviewMatrix           []domain.ReviewMatrixItem       `json:"review_matrix"`
	MissingWarningCount    int                             `json:"missing_warning_count"`
	LatestIntegrityReceipt *domain.ArchiveIntegrityReceipt `json:"latest_integrity_receipt,omitempty"`
}

type HistoricalArchiveSummary struct {
	ApplicationID    string `json:"application_id"`
	ApprovedRevision int    `json:"approved_revision"`
	ArchivedAt       string `json:"archived_at"`
}

func buildDetail(app *domain.MigrationApplication, events []persistence.EventRecord) DetailView {
	view := DetailView{Application: app, Events: events}
	view.CanEdit = app.Status == domain.StatusDraft || app.Status == domain.StatusRectifying
	view.CanAssess = view.CanEdit
	view.MissingWarningCount = len(app.MissingWarningDispositions())
	view.CanSubmit = app.Status == domain.StatusDraft && app.Assessment != nil && app.Assessment.Passed() && (app.Assessment.PlanDigest == "" || app.Assessment.PlanDigest == domain.CalculatePlanDigest(app)) && view.MissingWarningCount == 0
	view.CanSite = app.Status == domain.StatusSitePending
	view.CanReview = app.Status == domain.StatusReview
	view.CanRectify = app.Status == domain.StatusRectifying
	view.LockedSnapshot = domain.CurrentLockedSnapshot(app)
	view.SnapshotDifferences = domain.LockedSnapshotDifferences(app)
	view.EvidenceProgress = domain.EvidenceDraftProgress(app.EvidenceDraft)
	if view.CanReview {
		view.ReviewMatrix = domain.BuildExpectedReviewMatrix(app)
	}
	if view.CanRectify && len(app.Reviews) > 0 {
		view.OpenRectifications = append([]domain.RectificationItem(nil), app.Reviews[len(app.Reviews)-1].RectificationItems...)
	}
	return view
}
