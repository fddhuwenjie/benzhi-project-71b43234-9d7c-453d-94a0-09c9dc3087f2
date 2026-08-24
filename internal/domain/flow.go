package domain

import "time"

func (a *MigrationApplication) Submit(snapshot LockedPlanSnapshot, actor, requestID string, now time.Time) error {
	if a.Status != StatusDraft {
		return NewState("submit_state_invalid", "仅草稿可提交现场核验")
	}
	if a.Assessment == nil || !a.Assessment.Passed() {
		return NewState("blocking_findings_exist", "存在阻断项或尚未核查，不能提交")
	}
	if a.Assessment.PlanDigest != "" && a.Assessment.PlanDigest != CalculatePlanDigest(a) {
		return NewState("assessment_stale", "方案已变更，请重新执行规则核查")
	}
	if missing := a.MissingWarningDispositions(); len(missing) > 0 {
		return NewValidation("warning_disposition_missing", missing[0].Field, "警示项尚未处置："+missing[0].Code)
	}
	if err := ValidateLockedSnapshot(snapshot); err != nil {
		return err
	}
	a.SubmittedSnapshotID = snapshot.ID
	a.LockedSnapshots = append(a.LockedSnapshots, snapshot)
	return a.transition(StatusSitePending, "提交并锁定方案", actor, requestID, now)
}

func (a *MigrationApplication) CompleteSite(evidence SiteEvidence, actor, requestID string, now time.Time) error {
	if a.Status != StatusSitePending {
		return NewState("site_state_invalid", "当前状态不能登记完成现场核验")
	}
	a.Evidence = append(a.Evidence, evidence)
	a.EvidenceDraft = nil
	return a.transition(StatusReview, "现场证据齐备", actor, requestID, now)
}

func (a *MigrationApplication) ReturnForRectification(decision ReviewDecision, requestID string, now time.Time) error {
	if a.Status != StatusReview || decision.Outcome != OutcomeRectification {
		return NewState("review_state_invalid", "当前复核结论不能退回整改")
	}
	a.Reviews = append(a.Reviews, decision)
	return a.transition(StatusRectifying, "专家退回整改", decision.Reviewer, requestID, now)
}

func (a *MigrationApplication) ApplyRectification(rectification Rectification, actor, requestID string, now time.Time) error {
	if a.Status != StatusRectifying {
		return NewState("rectification_state_invalid", "当前状态不能提交补正")
	}
	a.Rectifications = append(a.Rectifications, rectification)
	a.Assessment = nil
	// 补正本身形成一个新修订，但仍处于整改状态，等待重新核查。
	a.bump(now)
	a.Timeline = append(a.Timeline, StatusEvent{ID: NewID("evt", now, requestID), ApplicationID: a.ID, From: StatusRectifying, To: StatusRectifying, Action: "提交整改补正", Actor: actor, RequestID: requestID, At: now.UTC()})
	return nil
}

func (a *MigrationApplication) Resubmit(snapshot LockedPlanSnapshot, actor, requestID string, now time.Time) error {
	if a.Status != StatusRectifying {
		return NewState("resubmit_state_invalid", "仅整改中的申请可重新提交")
	}
	if a.Assessment == nil || !a.Assessment.Passed() {
		return NewState("blocking_findings_exist", "补正版本存在阻断项或尚未核查")
	}
	if a.Assessment.PlanDigest != "" && a.Assessment.PlanDigest != CalculatePlanDigest(a) {
		return NewState("assessment_stale", "补正方案已变更，请重新核查")
	}
	if len(a.Rectifications) == 0 {
		return NewState("rectification_missing", "必须先提交本轮整改对照")
	}
	if err := ValidateLockedSnapshot(snapshot); err != nil {
		return err
	}
	a.Rectifications[len(a.Rectifications)-1].BoundSnapshotID = snapshot.ID
	a.Rectifications[len(a.Rectifications)-1].BoundRevision = snapshot.SubmittedRevision
	a.SubmittedSnapshotID = snapshot.ID
	a.LockedSnapshots = append(a.LockedSnapshots, snapshot)
	return a.transition(StatusReview, "整改后重新提交", actor, requestID, now)
}

func (a *MigrationApplication) Approve(decision ReviewDecision, archive ArchiveSummary, requestID string, now time.Time) error {
	if a.Status != StatusReview || decision.Outcome != OutcomeApproved {
		return NewState("approval_state_invalid", "当前状态不能批准归档")
	}
	if err := a.transition(StatusArchived, "专家批准并归档", decision.Reviewer, requestID, now); err != nil {
		return err
	}
	archive.Timeline = append(archive.Timeline, a.Timeline[len(a.Timeline)-1])
	archive.ApprovedRevision = a.Revision
	archive.Digest = CalculateArchiveDigest(archive)
	decision.ArchiveDigest = archive.Digest
	a.Reviews = append(a.Reviews, decision)
	a.Archive = &archive
	return nil
}
