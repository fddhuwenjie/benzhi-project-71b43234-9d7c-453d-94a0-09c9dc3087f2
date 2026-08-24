package domain

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

var RequiredMeasures = []string{"根球包扎完成", "树冠固定完成", "运输保湿就绪", "目标树穴验收"}
var RequiredPhotoCategories = []string{"迁移前全景", "根系保护", "树冠保护", "目标树穴"}

type MeasureCheck struct {
	Code      string `json:"code"`
	Confirmed bool   `json:"confirmed"`
	Note      string `json:"note,omitempty"`
}

type PhotoRecord struct {
	ID           string `json:"id"`
	FileName     string `json:"file_name"`
	Category     string `json:"category"`
	TakenAt      string `json:"taken_at"`
	LocationNote string `json:"location_note,omitempty"`
}

type EvidenceDraft struct {
	CapturedBy    string         `json:"captured_by"`
	Latitude      float64        `json:"latitude"`
	Longitude     float64        `json:"longitude"`
	Observations  string         `json:"observations"`
	MeasureChecks []MeasureCheck `json:"measure_checks"`
	PhotoRecords  []PhotoRecord  `json:"photo_records"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type EvidenceProgress struct {
	LocationComplete    bool     `json:"location_complete"`
	ObservationComplete bool     `json:"observation_complete"`
	MeasuresComplete    bool     `json:"measures_complete"`
	PhotosComplete      bool     `json:"photos_complete"`
	MissingItems        []string `json:"missing_items"`
}

type SiteEvidence struct {
	ID            string         `json:"id"`
	ApplicationID string         `json:"application_id"`
	CapturedBy    string         `json:"captured_by"`
	CapturedAt    time.Time      `json:"captured_at"`
	Latitude      float64        `json:"latitude"`
	Longitude     float64        `json:"longitude"`
	Observations  string         `json:"observations"`
	MeasureChecks []MeasureCheck `json:"measure_checks"`
	PhotoRecords  []PhotoRecord  `json:"photo_records"`
	ContentDigest string         `json:"content_digest"`
}

type EvidenceInput struct {
	CapturedBy    string         `json:"captured_by"`
	Latitude      float64        `json:"latitude"`
	Longitude     float64        `json:"longitude"`
	Observations  string         `json:"observations"`
	MeasureChecks []MeasureCheck `json:"measure_checks"`
	PhotoRecords  []PhotoRecord  `json:"photo_records"`
}

func BuildEvidence(applicationID string, input EvidenceInput, now time.Time) (SiteEvidence, error) {
	input.CapturedBy = strings.TrimSpace(input.CapturedBy)
	input.Observations = strings.TrimSpace(input.Observations)
	if input.CapturedBy == "" {
		return SiteEvidence{}, NewValidation("captured_by_required", "captured_by", "现场人员不能为空")
	}
	if input.Latitude < -90 || input.Latitude > 90 || input.Latitude == 0 {
		return SiteEvidence{}, NewValidation("latitude_invalid", "latitude", "纬度必须是有效的非零坐标")
	}
	if input.Longitude < -180 || input.Longitude > 180 || input.Longitude == 0 {
		return SiteEvidence{}, NewValidation("longitude_invalid", "longitude", "经度必须是有效的非零坐标")
	}
	if input.Observations == "" {
		return SiteEvidence{}, NewValidation("observations_required", "observations", "现场观察记录不能为空")
	}
	checks := make(map[string]bool)
	for i := range input.MeasureChecks {
		input.MeasureChecks[i].Code = strings.TrimSpace(input.MeasureChecks[i].Code)
		input.MeasureChecks[i].Note = strings.TrimSpace(input.MeasureChecks[i].Note)
		if input.MeasureChecks[i].Confirmed {
			checks[input.MeasureChecks[i].Code] = true
		}
	}
	for _, required := range RequiredMeasures {
		if !checks[required] {
			return SiteEvidence{}, NewValidation("measure_unconfirmed", "measure_checks", "保护措施未确认："+required)
		}
	}
	if len(input.PhotoRecords) == 0 {
		return SiteEvidence{}, NewValidation("photos_required", "photo_records", "至少登记一张定位照片")
	}
	for i := range input.PhotoRecords {
		input.PhotoRecords[i].FileName = strings.TrimSpace(input.PhotoRecords[i].FileName)
		input.PhotoRecords[i].Category = strings.TrimSpace(input.PhotoRecords[i].Category)
		if input.PhotoRecords[i].FileName == "" || input.PhotoRecords[i].Category == "" {
			return SiteEvidence{}, NewValidation("photo_metadata_invalid", "photo_records", "照片文件名和类别不能为空")
		}
		if input.PhotoRecords[i].ID == "" {
			input.PhotoRecords[i].ID = NewID("photo", now, applicationID+photoDuplicateKey(input.PhotoRecords[i]))
		}
	}
	evidence := SiteEvidence{ID: NewID("evidence", now, applicationID), ApplicationID: applicationID, CapturedBy: input.CapturedBy, CapturedAt: now.UTC(), Latitude: input.Latitude, Longitude: input.Longitude, Observations: input.Observations, MeasureChecks: input.MeasureChecks, PhotoRecords: input.PhotoRecords}
	evidence.ContentDigest = CalculateEvidenceDigest(evidence)
	return evidence, nil
}

func BuildEvidenceDraft(applicationID string, input EvidenceInput, now time.Time) (EvidenceDraft, error) {
	draft := EvidenceDraft{CapturedBy: strings.TrimSpace(input.CapturedBy), Latitude: input.Latitude, Longitude: input.Longitude, Observations: strings.TrimSpace(input.Observations), MeasureChecks: input.MeasureChecks, PhotoRecords: input.PhotoRecords, UpdatedAt: now.UTC()}
	if draft.Latitude < -90 || draft.Latitude > 90 {
		return EvidenceDraft{}, NewValidation("latitude_invalid", "latitude", "纬度必须在 -90 到 90 之间")
	}
	if draft.Longitude < -180 || draft.Longitude > 180 {
		return EvidenceDraft{}, NewValidation("longitude_invalid", "longitude", "经度必须在 -180 到 180 之间")
	}
	measureSeen := make(map[string]bool)
	for i := range draft.MeasureChecks {
		draft.MeasureChecks[i].Code = strings.TrimSpace(draft.MeasureChecks[i].Code)
		draft.MeasureChecks[i].Note = strings.TrimSpace(draft.MeasureChecks[i].Note)
		if draft.MeasureChecks[i].Code == "" || measureSeen[draft.MeasureChecks[i].Code] {
			return EvidenceDraft{}, NewValidation("measure_duplicate", "measure_checks", "保护措施代码为空或重复")
		}
		measureSeen[draft.MeasureChecks[i].Code] = true
	}
	photoSeen := make(map[string]bool)
	validCategories := make(map[string]bool)
	for _, category := range RequiredPhotoCategories {
		validCategories[category] = true
	}
	for i := range draft.PhotoRecords {
		photo := &draft.PhotoRecords[i]
		photo.FileName = strings.TrimSpace(photo.FileName)
		photo.Category = strings.TrimSpace(photo.Category)
		photo.TakenAt = strings.TrimSpace(photo.TakenAt)
		if photo.FileName == "" || photo.Category == "" || !validCategories[photo.Category] {
			return EvidenceDraft{}, NewValidation("photo_metadata_invalid", "photo_records", "照片文件名不能为空且类别必须为规定类别")
		}
		key := photoDuplicateKey(*photo)
		if photoSeen[key] {
			return EvidenceDraft{}, NewValidation("photo_duplicate", "photo_records", "照片元数据重复："+photo.FileName)
		}
		photoSeen[key] = true
		if photo.ID == "" {
			photo.ID = NewID("photo", now, applicationID+key)
		}
	}
	sort.Slice(draft.PhotoRecords, func(i, j int) bool {
		return photoDuplicateKey(draft.PhotoRecords[i]) < photoDuplicateKey(draft.PhotoRecords[j])
	})
	return draft, nil
}

func photoDuplicateKey(photo PhotoRecord) string {
	return strings.TrimSpace(photo.FileName) + "\x1f" + strings.TrimSpace(photo.TakenAt) + "\x1f" + strings.TrimSpace(photo.Category)
}

func EvidenceDraftProgress(draft *EvidenceDraft) EvidenceProgress {
	progress := EvidenceProgress{}
	if draft == nil {
		progress.MissingItems = []string{"有效非零定位", "现场人员", "观察记录", "全部必需保护措施", "全部必需照片类别"}
		return progress
	}
	progress.LocationComplete = draft.Latitude != 0 && draft.Longitude != 0
	if !progress.LocationComplete {
		progress.MissingItems = append(progress.MissingItems, "有效非零定位")
	}
	progress.ObservationComplete = strings.TrimSpace(draft.CapturedBy) != "" && strings.TrimSpace(draft.Observations) != ""
	if strings.TrimSpace(draft.CapturedBy) == "" {
		progress.MissingItems = append(progress.MissingItems, "现场人员")
	}
	if strings.TrimSpace(draft.Observations) == "" {
		progress.MissingItems = append(progress.MissingItems, "观察记录")
	}
	confirmed := make(map[string]bool)
	for _, check := range draft.MeasureChecks {
		confirmed[check.Code] = check.Confirmed
	}
	progress.MeasuresComplete = true
	for _, code := range RequiredMeasures {
		if !confirmed[code] {
			progress.MeasuresComplete = false
			progress.MissingItems = append(progress.MissingItems, "保护措施："+code)
		}
	}
	categories := make(map[string]bool)
	for _, photo := range draft.PhotoRecords {
		categories[photo.Category] = true
	}
	progress.PhotosComplete = true
	for _, category := range RequiredPhotoCategories {
		if !categories[category] {
			progress.PhotosComplete = false
			progress.MissingItems = append(progress.MissingItems, "照片类别："+category)
		}
	}
	return progress
}

func (a *MigrationApplication) SaveEvidenceDraft(input EvidenceInput, requestID, actor string, now time.Time) error {
	if a.Status != StatusSitePending {
		return NewState("site_state_invalid", "仅待现场核验申请可暂存证据")
	}
	draft, err := BuildEvidenceDraft(a.ID, input, now)
	if err != nil {
		return err
	}
	a.EvidenceDraft = &draft
	a.bump(now)
	a.Timeline = append(a.Timeline, StatusEvent{ID: NewID("evt", now, requestID), ApplicationID: a.ID, From: a.Status, To: a.Status, Action: "暂存现场证据", Actor: actor, RequestID: requestID, At: now.UTC()})
	return nil
}

func (a *MigrationApplication) DeleteEvidencePhoto(photoID, requestID, actor string, now time.Time) error {
	if a.Status != StatusSitePending || a.EvidenceDraft == nil {
		return NewState("site_draft_missing", "当前没有可修改的现场证据草稿")
	}
	kept := make([]PhotoRecord, 0, len(a.EvidenceDraft.PhotoRecords))
	found := false
	for _, photo := range a.EvidenceDraft.PhotoRecords {
		if photo.ID == photoID {
			found = true
		} else {
			kept = append(kept, photo)
		}
	}
	if !found {
		return NewValidation("photo_unknown", "photo_id", "未找到要删除的照片元数据")
	}
	a.EvidenceDraft.PhotoRecords = kept
	a.EvidenceDraft.UpdatedAt = now.UTC()
	a.bump(now)
	a.Timeline = append(a.Timeline, StatusEvent{ID: NewID("evt", now, requestID), ApplicationID: a.ID, From: a.Status, To: a.Status, Action: "删除误录现场照片", Actor: actor, RequestID: requestID, At: now.UTC()})
	return nil
}

func FreezeEvidence(applicationID string, draft *EvidenceDraft, now time.Time) (SiteEvidence, error) {
	progress := EvidenceDraftProgress(draft)
	if len(progress.MissingItems) > 0 {
		return SiteEvidence{}, NewValidation("site_evidence_incomplete", "evidence", "现场证据缺项："+strings.Join(progress.MissingItems, "、"))
	}
	evidence := SiteEvidence{ID: NewID("evidence", now, applicationID), ApplicationID: applicationID, CapturedBy: draft.CapturedBy, CapturedAt: now.UTC(), Latitude: draft.Latitude, Longitude: draft.Longitude, Observations: draft.Observations, MeasureChecks: append([]MeasureCheck(nil), draft.MeasureChecks...), PhotoRecords: append([]PhotoRecord(nil), draft.PhotoRecords...)}
	evidence.ContentDigest = CalculateEvidenceDigest(evidence)
	return evidence, nil
}

func CalculateEvidenceDigest(evidence SiteEvidence) string {
	evidence.ContentDigest = ""
	encoded, _ := json.Marshal(evidence)
	return Digest(string(encoded))
}
