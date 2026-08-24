package domain

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

type ReviewOutcome string

const (
	OutcomeApproved      ReviewOutcome = "approved"
	OutcomeRectification ReviewOutcome = "rectification"
)

type RectificationItem struct {
	ID          string `json:"id"`
	FindingCode string `json:"finding_code"`
	Requirement string `json:"requirement"`
}

type ReviewDecision struct {
	ID                 string              `json:"id"`
	ApplicationID      string              `json:"application_id"`
	ReviewedRevision   int                 `json:"reviewed_revision"`
	Reviewer           string              `json:"reviewer"`
	Outcome            ReviewOutcome       `json:"outcome"`
	Comments           string              `json:"comments"`
	RectificationItems []RectificationItem `json:"rectification_items,omitempty"`
	DecidedAt          time.Time           `json:"decided_at"`
	ArchiveDigest      string              `json:"archive_digest,omitempty"`
	SnapshotID         string              `json:"snapshot_id"`
	Matrix             []ReviewMatrixItem  `json:"matrix,omitempty"`
	MatrixDigest       string              `json:"matrix_digest,omitempty"`
}

type MatrixConclusion string

const (
	MatrixPassed        MatrixConclusion = "passed"
	MatrixRectification MatrixConclusion = "rectification"
)

type ReviewMatrixItem struct {
	ID                 string           `json:"id"`
	Kind               string           `json:"kind"`
	Label              string           `json:"label"`
	SourceID           string           `json:"source_id"`
	Conclusion         MatrixConclusion `json:"conclusion,omitempty"`
	ExpertNote         string           `json:"expert_note,omitempty"`
	EvidenceReferences []string         `json:"evidence_references,omitempty"`
}

type ReviewMatrixInput struct {
	ID                 string           `json:"id"`
	Conclusion         MatrixConclusion `json:"conclusion"`
	ExpertNote         string           `json:"expert_note"`
	EvidenceReferences []string         `json:"evidence_references"`
}

type ReviewInput struct {
	Reviewer           string              `json:"reviewer"`
	Outcome            ReviewOutcome       `json:"outcome"`
	Comments           string              `json:"comments"`
	RectificationItems []RectificationItem `json:"rectification_items"`
	Matrix             []ReviewMatrixInput `json:"matrix,omitempty"`
}

func BuildReview(applicationID string, revision int, input ReviewInput, now time.Time) (ReviewDecision, error) {
	input.Reviewer = strings.TrimSpace(input.Reviewer)
	input.Comments = strings.TrimSpace(input.Comments)
	if input.Reviewer == "" {
		return ReviewDecision{}, NewValidation("reviewer_required", "reviewer", "专家姓名不能为空")
	}
	if input.Outcome != OutcomeApproved && input.Outcome != OutcomeRectification {
		return ReviewDecision{}, NewValidation("outcome_invalid", "outcome", "复核结论无效")
	}
	if input.Comments == "" {
		return ReviewDecision{}, NewValidation("comments_required", "comments", "复核意见不能为空")
	}
	if input.Outcome == OutcomeRectification && len(input.RectificationItems) == 0 {
		return ReviewDecision{}, NewValidation("rectification_required", "rectification_items", "退回时必须一次性列出整改要求")
	}
	if input.Outcome == OutcomeApproved && len(input.RectificationItems) > 0 {
		return ReviewDecision{}, NewValidation("approved_with_rectification", "rectification_items", "批准结论不能包含整改要求")
	}
	seen := make(map[string]bool)
	for i := range input.RectificationItems {
		item := &input.RectificationItems[i]
		item.FindingCode = strings.TrimSpace(item.FindingCode)
		item.Requirement = strings.TrimSpace(item.Requirement)
		if item.Requirement == "" {
			return ReviewDecision{}, NewValidation("requirement_empty", "rectification_items", "整改要求不能为空")
		}
		if item.ID == "" {
			item.ID = NewID("rect", now, applicationID+item.FindingCode+item.Requirement)
		}
		if seen[item.ID] {
			return ReviewDecision{}, NewValidation("rectification_duplicate", "rectification_items", "整改项标识重复")
		}
		seen[item.ID] = true
	}
	return ReviewDecision{ID: NewID("review", now, applicationID), ApplicationID: applicationID, ReviewedRevision: revision, Reviewer: input.Reviewer, Outcome: input.Outcome, Comments: input.Comments, RectificationItems: input.RectificationItems, DecidedAt: now.UTC()}, nil
}

func BuildExpectedReviewMatrix(a *MigrationApplication) []ReviewMatrixItem {
	items := make([]ReviewMatrixItem, 0)
	snapshot := CurrentLockedSnapshot(a)
	if snapshot != nil {
		for _, warning := range snapshot.Content.Assessment.WarningFindings {
			digest := FindingDigest(warning)
			items = append(items, ReviewMatrixItem{ID: "warning:" + digest[:16], Kind: "warning", Label: warning.Message, SourceID: warning.Code})
		}
	}
	for _, measure := range RequiredMeasures {
		items = append(items, ReviewMatrixItem{ID: "measure:" + Digest(measure)[:16], Kind: "measure", Label: measure, SourceID: measure})
	}
	for _, evidence := range a.Evidence {
		for _, photo := range evidence.PhotoRecords {
			photoID := photo.ID
			if photoID == "" {
				photoID = Digest(evidence.ID, photoDuplicateKey(photo))[:16]
			}
			items = append(items, ReviewMatrixItem{ID: "photo:" + photoID, Kind: "photo", Label: photo.FileName + "（" + photo.Category + "）", SourceID: evidence.ID + ":" + photoID})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Kind == items[j].Kind {
			return items[i].ID < items[j].ID
		}
		return items[i].Kind < items[j].Kind
	})
	return items
}

func BuildReviewForApplication(a *MigrationApplication, input ReviewInput, now time.Time) (ReviewDecision, error) {
	expected := BuildExpectedReviewMatrix(a)
	matrixExplicit := input.Matrix != nil
	// 兼容旧调用：未提供矩阵时按已有总体结论生成完整矩阵；公开工作台始终显式提交矩阵。
	if input.Matrix == nil {
		for _, item := range expected {
			conclusion := MatrixPassed
			if input.Outcome == OutcomeRectification {
				conclusion = MatrixRectification
			}
			input.Matrix = append(input.Matrix, ReviewMatrixInput{ID: item.ID, Conclusion: conclusion, ExpertNote: input.Comments, EvidenceReferences: []string{item.SourceID}})
		}
	}
	provided := make(map[string]ReviewMatrixInput)
	for _, item := range input.Matrix {
		item.ID = strings.TrimSpace(item.ID)
		item.ExpertNote = strings.TrimSpace(item.ExpertNote)
		item.EvidenceReferences = SortedUnique(item.EvidenceReferences)
		if item.ID == "" || provided[item.ID].ID != "" {
			return ReviewDecision{}, NewValidation("review_matrix_duplicate", "matrix", "复核矩阵项为空或重复："+item.ID)
		}
		provided[item.ID] = item
	}
	validRefs := make(map[string]bool)
	for _, expectedItem := range expected {
		validRefs[expectedItem.SourceID] = true
	}
	frozen := make([]ReviewMatrixItem, 0, len(expected))
	hasRectification := false
	for _, expectedItem := range expected {
		item, ok := provided[expectedItem.ID]
		if !ok {
			return ReviewDecision{}, NewValidation("review_matrix_incomplete", "matrix", "复核矩阵未完成："+expectedItem.ID)
		}
		if item.Conclusion != MatrixPassed && item.Conclusion != MatrixRectification {
			return ReviewDecision{}, NewValidation("review_matrix_conclusion_invalid", "matrix", "复核矩阵结论无效："+item.ID)
		}
		if item.ExpertNote == "" {
			return ReviewDecision{}, NewValidation("review_matrix_note_required", "matrix", "复核矩阵缺少专家说明："+item.ID)
		}
		if len(item.EvidenceReferences) == 0 {
			return ReviewDecision{}, NewValidation("review_matrix_reference_required", "matrix", "复核矩阵缺少依据引用："+item.ID)
		}
		for _, ref := range item.EvidenceReferences {
			if !validRefs[ref] {
				return ReviewDecision{}, NewValidation("review_matrix_reference_invalid", "matrix", "复核矩阵引用不属于当前受审版本："+ref)
			}
		}
		expectedItem.Conclusion, expectedItem.ExpertNote, expectedItem.EvidenceReferences = item.Conclusion, item.ExpertNote, item.EvidenceReferences
		if item.Conclusion == MatrixRectification {
			hasRectification = true
		}
		frozen = append(frozen, expectedItem)
	}
	if len(provided) != len(expected) {
		return ReviewDecision{}, NewValidation("review_matrix_unknown", "matrix", "复核矩阵包含未知项")
	}
	if input.Outcome == OutcomeApproved && hasRectification {
		return ReviewDecision{}, NewValidation("review_outcome_mismatch", "outcome", "存在需整改矩阵项时不能批准")
	}
	if input.Outcome == OutcomeRectification && !hasRectification {
		return ReviewDecision{}, NewValidation("review_outcome_mismatch", "outcome", "退回结论至少需要一项需整改矩阵项")
	}
	if hasRectification && (matrixExplicit || len(input.RectificationItems) == 0) {
		input.RectificationItems = nil
		for _, item := range frozen {
			if item.Conclusion == MatrixRectification {
				input.RectificationItems = append(input.RectificationItems, RectificationItem{FindingCode: item.ID, Requirement: item.Label + "：" + item.ExpertNote})
			}
		}
	}
	decision, err := BuildReview(a.ID, a.Revision, input, now)
	if err != nil {
		return ReviewDecision{}, err
	}
	decision.SnapshotID = a.SubmittedSnapshotID
	decision.Matrix = frozen
	raw, _ := json.Marshal(frozen)
	decision.MatrixDigest = Digest(string(raw), decision.SnapshotID)
	return decision, nil
}

type RectificationResponse struct {
	ItemID               string                `json:"item_id"`
	Explanation          string                `json:"explanation"`
	ReplacementMaterials []string              `json:"replacement_materials"`
	Materials            []ReplacementMaterial `json:"materials,omitempty"`
}

type ReplacementMaterial struct {
	Name          string `json:"name"`
	Category      string `json:"category"`
	VersionNote   string `json:"version_note"`
	ContentDigest string `json:"content_digest"`
}

type RectificationDifference struct {
	ItemID string `json:"item_id"`
	Result string `json:"result"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type Rectification struct {
	ID                string                    `json:"id"`
	ApplicationID     string                    `json:"application_id"`
	SourceReviewID    string                    `json:"source_review_id"`
	Responses         []RectificationResponse   `json:"responses"`
	BeforeRevision    int                       `json:"before_revision"`
	AfterRevision     int                       `json:"after_revision"`
	DifferenceSummary []string                  `json:"difference_summary"`
	Differences       []RectificationDifference `json:"differences"`
	BoundSnapshotID   string                    `json:"bound_snapshot_id,omitempty"`
	BoundRevision     int                       `json:"bound_revision,omitempty"`
	SubmittedAt       time.Time                 `json:"submitted_at"`
}

func BuildRectification(a *MigrationApplication, responses []RectificationResponse, now time.Time) (Rectification, error) {
	if len(a.Reviews) == 0 || a.Reviews[len(a.Reviews)-1].Outcome != OutcomeRectification {
		return Rectification{}, NewState("no_open_rectification", "不存在待补正的整改要求")
	}
	review := a.Reviews[len(a.Reviews)-1]
	for _, existing := range a.Rectifications {
		if existing.SourceReviewID == review.ID {
			return Rectification{}, NewState("rectification_already_submitted", "本轮整改要求已提交补正")
		}
	}
	responseByID := make(map[string]RectificationResponse)
	materialDigests := make(map[string]bool)
	for _, response := range responses {
		response.ItemID = strings.TrimSpace(response.ItemID)
		response.Explanation = strings.TrimSpace(response.Explanation)
		response.ReplacementMaterials = SortedUnique(response.ReplacementMaterials)
		for _, legacy := range response.ReplacementMaterials {
			response.Materials = append(response.Materials, ReplacementMaterial{Name: legacy, Category: "其他", VersionNote: "旧版材料记录", ContentDigest: Digest(legacy)})
		}
		for i := range response.Materials {
			material := &response.Materials[i]
			material.Name, material.Category, material.VersionNote, material.ContentDigest = strings.TrimSpace(material.Name), strings.TrimSpace(material.Category), strings.TrimSpace(material.VersionNote), strings.TrimSpace(material.ContentDigest)
			if material.Name == "" || material.Category == "" || material.VersionNote == "" || material.ContentDigest == "" {
				return Rectification{}, NewValidation("replacement_material_invalid", "responses", "替换材料名称、类别、版本说明和内容摘要均不能为空")
			}
			if materialDigests[material.ContentDigest] {
				return Rectification{}, NewValidation("replacement_digest_duplicate", "responses", "替换材料内容摘要重复："+material.ContentDigest)
			}
			materialDigests[material.ContentDigest] = true
		}
		if response.Explanation == "" {
			return Rectification{}, NewValidation("explanation_required", "responses", "每条整改项都必须填写补正说明")
		}
		if _, exists := responseByID[response.ItemID]; exists {
			return Rectification{}, NewValidation("response_duplicate", "responses", "同一整改项不能重复提交补正响应")
		}
		responseByID[response.ItemID] = response
	}
	ordered := make([]RectificationResponse, 0, len(review.RectificationItems))
	diffs := make([]string, 0, len(review.RectificationItems))
	structured := make([]RectificationDifference, 0, len(review.RectificationItems))
	for _, item := range review.RectificationItems {
		response, ok := responseByID[item.ID]
		if !ok {
			return Rectification{}, NewValidation("response_missing", "responses", "缺少整改项补正："+item.Requirement)
		}
		ordered = append(ordered, response)
		detail := item.Requirement + " -> " + response.Explanation
		result := "explained"
		if len(response.Materials) > 0 {
			result = "changed"
			names := make([]string, 0, len(response.Materials))
			for _, material := range response.Materials {
				names = append(names, material.Name+"@"+material.VersionNote)
			}
			detail += "；替换材料：" + strings.Join(names, "、")
		}
		diffs = append(diffs, detail)
		structured = append(structured, RectificationDifference{ItemID: item.ID, Result: result, Before: rectificationBefore(a, review, item), After: response.Explanation})
	}
	if len(responseByID) != len(review.RectificationItems) {
		return Rectification{}, NewValidation("response_unknown", "responses", "补正响应包含未知整改项")
	}
	return Rectification{ID: NewID("correction", now, a.ID), ApplicationID: a.ID, SourceReviewID: review.ID, Responses: ordered, BeforeRevision: a.Revision, AfterRevision: a.Revision + 1, DifferenceSummary: diffs, Differences: structured, SubmittedAt: now.UTC()}, nil
}

func rectificationBefore(a *MigrationApplication, review ReviewDecision, item RectificationItem) string {
	for _, matrix := range review.Matrix {
		if matrix.ID != item.FindingCode {
			continue
		}
		switch matrix.Kind {
		case "warning":
			if snapshot := CurrentLockedSnapshot(a); snapshot != nil {
				for _, finding := range snapshot.Content.Assessment.WarningFindings {
					if finding.Code == matrix.SourceID {
						raw, _ := json.Marshal(finding)
						return string(raw)
					}
				}
			}
		case "measure":
			for _, evidence := range a.Evidence {
				for _, check := range evidence.MeasureChecks {
					if check.Code == matrix.SourceID {
						raw, _ := json.Marshal(check)
						return string(raw)
					}
				}
			}
		case "photo":
			for _, evidence := range a.Evidence {
				for _, photo := range evidence.PhotoRecords {
					if matrix.SourceID == evidence.ID+":"+photo.ID {
						raw, _ := json.Marshal(photo)
						return string(raw)
					}
				}
			}
		}
	}
	return item.Requirement
}
