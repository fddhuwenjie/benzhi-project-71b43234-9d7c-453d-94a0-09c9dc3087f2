package domain

import (
	"reflect"
	"testing"
	"time"
)

func TestAssessApplicationStableAndPassing(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	app := NewApplication("app-test", completeDraft(), now)
	first := AssessApplication(app, now)
	second := AssessApplication(app, now)
	if !first.Passed() {
		t.Fatalf("expected passing assessment, findings = %+v", first.BlockingFindings)
	}
	if first.ResultDigest != second.ResultDigest || !reflect.DeepEqual(first.WarningFindings, second.WarningFindings) {
		t.Fatal("assessment must be deterministic")
	}
}

func TestAssessApplicationFindingsHaveStableOrder(t *testing.T) {
	app := NewApplication("app-invalid", DraftInput{}, time.Now())
	assessment := AssessApplication(app, time.Now())
	if len(assessment.BlockingFindings) < 10 {
		t.Fatalf("expected comprehensive findings, got %d", len(assessment.BlockingFindings))
	}
	for index := 1; index < len(assessment.BlockingFindings); index++ {
		previous := assessment.BlockingFindings[index-1]
		current := assessment.BlockingFindings[index]
		if previous.Field > current.Field || previous.Field == current.Field && previous.Code > current.Code {
			t.Fatalf("findings not sorted at %d: %+v then %+v", index, previous, current)
		}
	}
}

func TestBuildEvidenceRequiresEveryMeasure(t *testing.T) {
	input := EvidenceInput{CapturedBy: "现场员", Latitude: 31.2, Longitude: 121.4, Observations: "正常", PhotoRecords: []PhotoRecord{{FileName: "a.jpg", Category: "全景"}}}
	_, err := BuildEvidence("app", input, time.Now())
	if err == nil {
		t.Fatal("expected missing measure error")
	}
	for _, code := range RequiredMeasures {
		input.MeasureChecks = append(input.MeasureChecks, MeasureCheck{Code: code, Confirmed: true})
	}
	if _, err := BuildEvidence("app", input, time.Now()); err != nil {
		t.Fatalf("complete evidence rejected: %v", err)
	}
}

func TestRectificationRequiresExactResponses(t *testing.T) {
	now := time.Now().UTC()
	app := NewApplication("app", completeDraft(), now)
	app.Status = StatusRectifying
	app.Reviews = []ReviewDecision{{ID: "review", Outcome: OutcomeRectification, RectificationItems: []RectificationItem{{ID: "one", Requirement: "补充材料"}, {ID: "two", Requirement: "修订措施"}}}}
	_, err := BuildRectification(app, []RectificationResponse{{ItemID: "one", Explanation: "已补充"}}, now)
	if err == nil {
		t.Fatal("expected missing response error")
	}
	result, err := BuildRectification(app, []RectificationResponse{{ItemID: "one", Explanation: "已补充", ReplacementMaterials: []string{"B", "A"}}, {ItemID: "two", Explanation: "已修订"}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.DifferenceSummary) != 2 || result.Responses[0].ReplacementMaterials[0] != "A" {
		t.Fatalf("unexpected rectification: %+v", result)
	}
}

func completeDraft() DraftInput {
	return DraftInput{
		TreeCode: "GS-1", Species: "香樟", CurrentLocation: "现址", TargetLocation: "目标地", MigrationReason: "保护迁移",
		PlannedWindow:  PlannedWindow{Start: "2026-11-01", End: "2026-11-03"},
		ProtectionPlan: ProtectionPlan{RootRadiusMeters: 4, TransportHours: 2, TargetSoilReady: true, TargetDrainageReady: true, CanopyProtection: "收冠", RootBallProtection: "包扎", TransportProtection: "固定", PostPlantingCare: "养护", AttachedMaterials: []string{"目标地检测报告", "树木现状照片", "迁移路线图"}, EstimatedTrunkDiameter: 60},
	}
}
