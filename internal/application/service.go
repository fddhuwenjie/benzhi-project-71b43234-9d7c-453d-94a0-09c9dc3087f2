package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/domain"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/persistence"
)

type Service struct {
	repository Repository
	clock      Clock
	listOnce   sync.Once
	listCache  []ApplicationSummary
	listErr    error
}

func NewService(repository Repository, clock Clock) *Service {
	if clock == nil {
		clock = RealClock{}
	}
	return &Service{repository: repository, clock: clock}
}

func (s *Service) CreateDraft(command CreateDraftCommand) (DetailView, error) {
	command.RequestID = strings.TrimSpace(command.RequestID)
	if command.RequestID == "" {
		return DetailView{}, domain.NewValidation("request_id_required", "request_id", "创建请求必须提供请求标识")
	}
	if len(command.RequestID) > 128 {
		return DetailView{}, domain.NewValidation("request_id_too_long", "request_id", "请求标识不能超过 128 个字符")
	}
	now := s.clock.Now()
	id := "app_" + domain.Digest(command.RequestID)[:16]
	app := domain.NewApplication(id, command.Draft, now)
	if err := s.repository.Create(app); err != nil {
		var duplicate *persistence.ActiveDuplicateError
		if errors.As(err, &duplicate) {
			return DetailView{}, translateRepositoryError(err)
		}
		existing, loadErr := s.repository.Load(id)
		if loadErr != nil {
			return DetailView{}, translateRepositoryError(err)
		}
		events, eventsErr := s.repository.Events(id)
		if eventsErr != nil {
			return DetailView{}, eventsErr
		}
		return s.buildDetail(existing, events)
	}
	return s.Get(id)
}

func (s *Service) Get(id string) (DetailView, error) {
	app, err := s.repository.Load(id)
	if err != nil {
		return DetailView{}, translateRepositoryError(err)
	}
	events, err := s.repository.Events(id)
	if err != nil {
		return DetailView{}, err
	}
	return s.buildDetail(app, events)
}

func (s *Service) buildDetail(app *domain.MigrationApplication, events []persistence.EventRecord) (DetailView, error) {
	if err := domain.ValidateLockedSnapshots(app); err != nil {
		return DetailView{}, err
	}
	view := buildDetail(app, events)
	matches, err := s.repository.FindByTreeCode(app.TreeCode)
	if err != nil {
		return DetailView{}, err
	}
	for _, match := range matches {
		if match.ID != app.ID && match.Status == domain.StatusArchived && match.Archive != nil {
			view.HistoricalArchives = append(view.HistoricalArchives, HistoricalArchiveSummary{ApplicationID: match.ID, ApprovedRevision: match.Archive.ApprovedRevision, ArchivedAt: match.Archive.ArchivedAt.Format(time.RFC3339)})
		}
	}
	receipt, err := s.repository.LatestReceipt(app.ID)
	if err != nil {
		return DetailView{}, err
	}
	view.LatestIntegrityReceipt = receipt
	return view, nil
}

func (s *Service) List() ([]ApplicationSummary, error) {
	s.listOnce.Do(func() {
		apps, err := s.repository.List()
		if err != nil {
			s.listErr = err
			return
		}
		result := make([]ApplicationSummary, 0, len(apps))
		for _, app := range apps {
			result = append(result, ApplicationSummary{ID: app.ID, TreeCode: app.TreeCode, Species: app.Species, TargetLocation: app.TargetLocation, Status: app.Status, Revision: app.Revision, UpdatedAt: app.UpdatedAt.Format("2006-01-02 15:04")})
		}
		s.listCache = result
	})
	if s.listErr != nil {
		return nil, s.listErr
	}
	return append([]ApplicationSummary(nil), s.listCache...), nil
}

func (s *Service) SaveDraft(id string, command SaveDraftCommand) (DetailView, error) {
	return s.mutate(id, "save_draft", command.Meta, func(app *domain.MigrationApplication) (any, error) {
		return nil, app.UpdateDraft(command.Draft, s.clock.Now())
	})
}

func (s *Service) Assess(id string, command AssessCommand) (DetailView, error) {
	return s.mutate(id, "assess", command.Meta, func(app *domain.MigrationApplication) (any, error) {
		now := s.clock.Now()
		assessment := domain.AssessApplication(app, now)
		return assessment, app.SetAssessment(assessment, now)
	})
}

func (s *Service) Submit(id string, command SubmitCommand) (DetailView, error) {
	return s.mutate(id, "submit", command.Meta, func(app *domain.MigrationApplication) (any, error) {
		now := s.clock.Now()
		snapshot, err := domain.BuildLockedSnapshot(app, command.Meta.RequestID, now)
		if err != nil {
			return nil, err
		}
		return snapshot, app.Submit(snapshot, command.Meta.Actor, command.Meta.RequestID, now)
	})
}

func (s *Service) SaveWarningDisposition(id string, command WarningDispositionCommand) (DetailView, error) {
	return s.mutate(id, "warning_disposition", command.Meta, func(app *domain.MigrationApplication) (any, error) {
		return command.Disposition, app.SaveWarningDisposition(command.Disposition, command.Meta.Actor, command.Meta.RequestID, s.clock.Now())
	})
}

func (s *Service) CompleteSite(id string, command SiteCommand) (DetailView, error) {
	return s.mutate(id, "complete_site", command.Meta, func(app *domain.MigrationApplication) (any, error) {
		now := s.clock.Now()
		evidence, err := domain.BuildEvidence(app.ID, command.Evidence, now)
		if err != nil {
			return nil, err
		}
		return evidence, app.CompleteSite(evidence, command.Meta.Actor, command.Meta.RequestID, now)
	})
}

func (s *Service) CompleteSiteStrict(id string, command SiteCommand) (DetailView, error) {
	return s.mutate(id, "complete_site", command.Meta, func(app *domain.MigrationApplication) (any, error) {
		now := s.clock.Now()
		draft, err := domain.BuildEvidenceDraft(app.ID, command.Evidence, now)
		if err != nil {
			return nil, err
		}
		evidence, err := domain.FreezeEvidence(app.ID, &draft, now)
		if err != nil {
			return nil, err
		}
		return evidence, app.CompleteSite(evidence, command.Meta.Actor, command.Meta.RequestID, now)
	})
}

func (s *Service) SaveSiteDraft(id string, command SiteDraftCommand) (DetailView, error) {
	return s.mutate(id, "save_site_draft", command.Meta, func(app *domain.MigrationApplication) (any, error) {
		return command.Evidence, app.SaveEvidenceDraft(command.Evidence, command.Meta.RequestID, command.Meta.Actor, s.clock.Now())
	})
}

func (s *Service) DeleteSitePhoto(id, photoID string, command DeletePhotoCommand) (DetailView, error) {
	return s.mutate(id, "delete_site_photo", command.Meta, func(app *domain.MigrationApplication) (any, error) {
		return map[string]string{"photo_id": photoID}, app.DeleteEvidencePhoto(photoID, command.Meta.RequestID, command.Meta.Actor, s.clock.Now())
	})
}

func (s *Service) ConfirmSite(id string, command ConfirmSiteCommand) (DetailView, error) {
	return s.mutate(id, "confirm_site", command.Meta, func(app *domain.MigrationApplication) (any, error) {
		now := s.clock.Now()
		evidence, err := domain.FreezeEvidence(app.ID, app.EvidenceDraft, now)
		if err != nil {
			return nil, err
		}
		return evidence, app.CompleteSite(evidence, command.Meta.Actor, command.Meta.RequestID, now)
	})
}

func (s *Service) Review(id string, command ReviewCommand) (DetailView, error) {
	operation := "review_rectification"
	if command.Review.Outcome == domain.OutcomeApproved {
		operation = "review_approve"
	}
	return s.mutate(id, operation, command.Meta, func(app *domain.MigrationApplication) (any, error) {
		now := s.clock.Now()
		decision, err := domain.BuildReviewForApplication(app, command.Review, now)
		if err != nil {
			return nil, err
		}
		if decision.Outcome == domain.OutcomeRectification {
			return decision, app.ReturnForRectification(decision, command.Meta.RequestID, now)
		}
		archive, err := domain.BuildArchive(app, now)
		if err != nil {
			return nil, err
		}
		archive.FinalReviewMatrix = append([]domain.ReviewMatrixItem(nil), decision.Matrix...)
		return archive, app.Approve(decision, archive, command.Meta.RequestID, now)
	})
}

func (s *Service) Rectify(id string, command RectifyCommand) (DetailView, error) {
	return s.mutate(id, "rectify", command.Meta, func(app *domain.MigrationApplication) (any, error) {
		now := s.clock.Now()
		correction, err := domain.BuildRectification(app, command.Responses, now)
		if err != nil {
			return nil, err
		}
		return correction, app.ApplyRectification(correction, command.Meta.Actor, command.Meta.RequestID, now)
	})
}

func (s *Service) Resubmit(id string, command ResubmitCommand) (DetailView, error) {
	return s.mutate(id, "resubmit", command.Meta, func(app *domain.MigrationApplication) (any, error) {
		now := s.clock.Now()
		snapshot, err := domain.BuildLockedSnapshot(app, command.Meta.RequestID, now)
		if err != nil {
			return nil, err
		}
		return snapshot, app.Resubmit(snapshot, command.Meta.Actor, command.Meta.RequestID, now)
	})
}

func (s *Service) VerifyArchive(id string, command VerifyArchiveCommand) (domain.ArchiveIntegrityReceipt, error) {
	if strings.TrimSpace(command.RequestID) == "" {
		return domain.ArchiveIntegrityReceipt{}, domain.NewValidation("request_id_required", "request_id", "核验请求必须提供请求标识")
	}
	app, err := s.repository.Load(id)
	if err != nil {
		return domain.ArchiveIntegrityReceipt{}, translateRepositoryError(err)
	}
	events, err := s.repository.Events(id)
	if err != nil {
		return domain.ArchiveIntegrityReceipt{}, err
	}
	integrityEvents := make([]domain.IntegrityEvent, 0, len(events))
	for _, event := range events {
		integrityEvents = append(integrityEvents, domain.IntegrityEvent{ID: strconv.FormatInt(event.Sequence, 10), Revision: event.Revision, Operation: event.Operation, Status: event.Status})
	}
	receipt, err := domain.VerifyArchiveIntegrity(app, integrityEvents, s.clock.Now())
	if err != nil {
		return domain.ArchiveIntegrityReceipt{}, err
	}
	if err := s.repository.SaveReceipt(receipt); err != nil {
		return domain.ArchiveIntegrityReceipt{}, err
	}
	return receipt, nil
}

func (s *Service) mutate(id, operation string, meta CommandMeta, change func(*domain.MigrationApplication) (any, error)) (DetailView, error) {
	if err := validateMeta(meta); err != nil {
		return DetailView{}, err
	}
	if previous, ok, err := s.repository.PreviousResult(id, operation, meta.RequestID); err != nil {
		return DetailView{}, translateRepositoryError(err)
	} else if ok {
		_ = previous
		return s.Get(id)
	}
	app, err := s.repository.Load(id)
	if err != nil {
		return DetailView{}, translateRepositoryError(err)
	}
	if err := app.EnsureMutable(meta.ExpectedRevision); err != nil {
		return DetailView{}, err
	}
	before := app.Revision
	payload, err := change(app)
	if err != nil {
		return DetailView{}, err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return DetailView{}, fmt.Errorf("编码命令结果: %w", err)
	}
	if _, err := s.repository.Save(before, app, operation, meta.RequestID, raw); err != nil {
		return DetailView{}, translateRepositoryError(err)
	}
	return s.Get(id)
}
