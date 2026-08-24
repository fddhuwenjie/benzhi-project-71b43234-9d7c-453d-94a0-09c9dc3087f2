package domain

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const RuleSetVersion = "2026.1"

type Severity string

const (
	SeverityBlocking Severity = "blocking"
	SeverityWarning  Severity = "warning"
)

type Finding struct {
	Code     string   `json:"code"`
	Field    string   `json:"field"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

type RuleAssessment struct {
	ID                  string    `json:"id"`
	ApplicationID       string    `json:"application_id"`
	ApplicationRevision int       `json:"application_revision"`
	RuleSetVersion      string    `json:"rule_set_version"`
	BlockingFindings    []Finding `json:"blocking_findings"`
	WarningFindings     []Finding `json:"warning_findings"`
	AssessedAt          time.Time `json:"assessed_at"`
	ResultDigest        string    `json:"result_digest"`
	PlanDigest          string    `json:"plan_digest"`
}

func AssessApplication(a *MigrationApplication, now time.Time) RuleAssessment {
	var blocking, warnings []Finding
	add := func(target *[]Finding, code, field, message string, severity Severity) {
		*target = append(*target, Finding{Code: code, Field: field, Severity: severity, Message: message})
	}
	required := []struct{ value, field, label string }{
		{a.TreeCode, "tree_code", "古树编号"}, {a.Species, "species", "树种"},
		{a.CurrentLocation, "current_location", "现位置"}, {a.TargetLocation, "target_location", "目标位置"},
		{a.MigrationReason, "migration_reason", "迁移原因"}, {a.ProtectionPlan.CanopyProtection, "protection_plan.canopy_protection", "树冠保护措施"},
		{a.ProtectionPlan.RootBallProtection, "protection_plan.root_ball_protection", "根球保护措施"},
		{a.ProtectionPlan.TransportProtection, "protection_plan.transport_protection", "运输保护措施"},
		{a.ProtectionPlan.PostPlantingCare, "protection_plan.post_planting_care", "栽后养护方案"},
	}
	for _, item := range required {
		if strings.TrimSpace(item.value) == "" {
			add(&blocking, "required", item.field, item.label+"不能为空", SeverityBlocking)
		}
	}
	start, startErr := time.Parse("2006-01-02", a.PlannedWindow.Start)
	end, endErr := time.Parse("2006-01-02", a.PlannedWindow.End)
	if startErr != nil {
		add(&blocking, "window_start_invalid", "planned_window.start", "迁移开始日期格式应为 YYYY-MM-DD", SeverityBlocking)
	}
	if endErr != nil {
		add(&blocking, "window_end_invalid", "planned_window.end", "迁移结束日期格式应为 YYYY-MM-DD", SeverityBlocking)
	}
	if startErr == nil && endErr == nil {
		if end.Before(start) {
			add(&blocking, "window_order", "planned_window.end", "迁移结束日期不得早于开始日期", SeverityBlocking)
		}
		if end.Sub(start) > 31*24*time.Hour {
			add(&warnings, "window_too_long", "planned_window", "迁移时间窗口超过 31 天，请确认施工排期", SeverityWarning)
		}
		for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
			if day.Month() >= time.June && day.Month() <= time.August {
				add(&warnings, "summer_window", "planned_window", "时间窗口包含高温季节，应加强保湿和遮阴", SeverityWarning)
				break
			}
		}
	}
	minimumRadius := a.ProtectionPlan.EstimatedTrunkDiameter * 0.06
	if a.ProtectionPlan.EstimatedTrunkDiameter <= 0 {
		add(&blocking, "diameter_required", "protection_plan.estimated_trunk_diameter_cm", "必须填写胸径估值", SeverityBlocking)
	}
	if a.ProtectionPlan.RootRadiusMeters <= 0 {
		add(&blocking, "root_radius_required", "protection_plan.root_radius_meters", "根系保护半径必须大于零", SeverityBlocking)
	} else if minimumRadius > 0 && a.ProtectionPlan.RootRadiusMeters < minimumRadius {
		add(&blocking, "root_radius_insufficient", "protection_plan.root_radius_meters", fmt.Sprintf("根系保护半径不得小于 %.1f 米", minimumRadius), SeverityBlocking)
	}
	if a.ProtectionPlan.TransportHours <= 0 {
		add(&blocking, "transport_time_required", "protection_plan.transport_hours", "运输时长必须大于零", SeverityBlocking)
	} else if a.ProtectionPlan.TransportHours > 8 {
		add(&blocking, "transport_time_excessive", "protection_plan.transport_hours", "预计运输时长不得超过 8 小时", SeverityBlocking)
	} else if a.ProtectionPlan.TransportHours > 4 {
		add(&warnings, "transport_time_warning", "protection_plan.transport_hours", "预计运输超过 4 小时，应补充途中保湿检查", SeverityWarning)
	}
	if !a.ProtectionPlan.TargetSoilReady {
		add(&blocking, "target_soil_unready", "protection_plan.target_soil_ready", "目标地土壤条件尚未确认", SeverityBlocking)
	}
	if !a.ProtectionPlan.TargetDrainageReady {
		add(&blocking, "target_drainage_unready", "protection_plan.target_drainage_ready", "目标地排水条件尚未确认", SeverityBlocking)
	}
	materialSet := make(map[string]bool)
	for _, material := range a.ProtectionPlan.AttachedMaterials {
		materialSet[material] = true
	}
	for _, material := range []string{"树木现状照片", "迁移路线图", "目标地检测报告"} {
		if !materialSet[material] {
			add(&blocking, "material_missing", "protection_plan.attached_materials", "缺少必填材料："+material, SeverityBlocking)
		}
	}
	sortFindings(blocking)
	sortFindings(warnings)
	encoded, _ := json.Marshal(struct{ Blocking, Warnings []Finding }{blocking, warnings})
	return RuleAssessment{ID: NewID("assessment", now, a.ID), ApplicationID: a.ID, ApplicationRevision: a.Revision, RuleSetVersion: RuleSetVersion, BlockingFindings: blocking, WarningFindings: warnings, AssessedAt: now.UTC(), ResultDigest: Digest(string(encoded)), PlanDigest: CalculatePlanDigest(a)}
}

func CalculateAssessmentResultDigest(assessment RuleAssessment) string {
	encoded, _ := json.Marshal(struct{ Blocking, Warnings []Finding }{assessment.BlockingFindings, assessment.WarningFindings})
	return Digest(string(encoded))
}

func sortFindings(items []Finding) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Field == items[j].Field {
			return items[i].Code < items[j].Code
		}
		return items[i].Field < items[j].Field
	})
}

func (r RuleAssessment) Passed() bool { return len(r.BlockingFindings) == 0 }

func CalculatePlanDigest(a *MigrationApplication) string {
	value := struct {
		TreeCode, Species, CurrentLocation, TargetLocation, MigrationReason string
		PlannedWindow                                                       PlannedWindow
		ProtectionPlan                                                      ProtectionPlan
	}{a.TreeCode, a.Species, a.CurrentLocation, a.TargetLocation, a.MigrationReason, a.PlannedWindow, NormalizePlan(a.ProtectionPlan)}
	encoded, _ := json.Marshal(value)
	return Digest(string(encoded))
}

func SortedUnique(values []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}
