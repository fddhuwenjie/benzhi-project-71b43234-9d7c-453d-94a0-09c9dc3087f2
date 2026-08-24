package application

import (
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/domain"
)

type CommandMeta struct {
	ExpectedRevision int    `json:"revision"`
	RequestID        string `json:"request_id"`
	Actor            string `json:"actor"`
}

type CreateDraftCommand struct {
	RequestID string            `json:"request_id"`
	Actor     string            `json:"actor"`
	Draft     domain.DraftInput `json:"draft"`
}

type SaveDraftCommand struct {
	Meta  CommandMeta       `json:"meta"`
	Draft domain.DraftInput `json:"draft"`
}

type AssessCommand struct {
	Meta CommandMeta `json:"meta"`
}

type SubmitCommand struct {
	Meta CommandMeta `json:"meta"`
}

type WarningDispositionCommand struct {
	Meta        CommandMeta                    `json:"meta"`
	Disposition domain.WarningDispositionInput `json:"disposition"`
}

type SiteCommand struct {
	Meta     CommandMeta          `json:"meta"`
	Evidence domain.EvidenceInput `json:"evidence"`
}

type SiteDraftCommand struct {
	Meta     CommandMeta          `json:"meta"`
	Evidence domain.EvidenceInput `json:"evidence"`
}

type DeletePhotoCommand struct {
	Meta CommandMeta `json:"meta"`
}
type ConfirmSiteCommand struct {
	Meta CommandMeta `json:"meta"`
}

type ReviewCommand struct {
	Meta   CommandMeta        `json:"meta"`
	Review domain.ReviewInput `json:"review"`
}

type RectifyCommand struct {
	Meta      CommandMeta                    `json:"meta"`
	Responses []domain.RectificationResponse `json:"responses"`
}

type ResubmitCommand struct {
	Meta CommandMeta `json:"meta"`
}

type VerifyArchiveCommand struct {
	RequestID string `json:"request_id"`
	Actor     string `json:"actor"`
}
