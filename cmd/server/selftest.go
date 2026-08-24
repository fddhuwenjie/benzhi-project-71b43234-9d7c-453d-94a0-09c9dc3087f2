package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/application"
	"benzhi-project-71b43234-9d7c-453d-94a0-09c9dc3087f2/internal/domain"
)

type selftestClient struct {
	baseURL string
	client  *http.Client
}

func runSelftest(cfg config, logger *slog.Logger) error {
	tempDir, err := os.MkdirTemp("", "benzhi-tree-selftest-*")
	if err != nil {
		return fmt.Errorf("创建自检数据目录: %w", err)
	}
	defer os.RemoveAll(tempDir)
	runtime, err := buildRuntime(cfg.address, tempDir, logger)
	if err != nil {
		return err
	}
	errors := make(chan error, 1)
	go runtime.serve(errors)
	client := &selftestClient{baseURL: "http://" + runtime.listener.Addr().String(), client: &http.Client{Timeout: 4 * time.Second}}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	flowErr := client.fullFlow(ctx)
	shutdownErr := runtime.shutdown()
	serveErr := <-errors
	if flowErr != nil {
		return flowErr
	}
	if shutdownErr != nil {
		return fmt.Errorf("自检关闭失败: %w", shutdownErr)
	}
	if serveErr != nil {
		return fmt.Errorf("自检服务失败: %w", serveErr)
	}
	logger.Info("完整业务流程自检通过")
	return nil
}

func (c *selftestClient) fullFlow(ctx context.Context) error {
	var view application.DetailView
	draft := domain.DraftInput{
		TreeCode: "SELFTEST-GS-001", Species: "香樟", CurrentLocation: "示范大道 12 号", TargetLocation: "城市植物园东区", MigrationReason: "道路保护性施工需要避让",
		PlannedWindow:  domain.PlannedWindow{Start: "2026-11-02", End: "2026-11-05"},
		ProtectionPlan: domain.ProtectionPlan{RootRadiusMeters: 4, TransportHours: 2.5, TargetSoilReady: true, TargetDrainageReady: true, CanopyProtection: "软带分层收冠并加装防磨垫", RootBallProtection: "根球麻布和钢丝网双层包扎", TransportProtection: "专用车辆固定并持续保湿", PostPlantingCare: "支撑、灌溉和一年复壮监测", AttachedMaterials: []string{"树木现状照片", "迁移路线图", "目标地检测报告"}, EstimatedTrunkDiameter: 60},
	}
	if err := c.json(ctx, http.MethodPost, "/api/applications", application.CreateDraftCommand{RequestID: "selftest-create", Actor: "自检编制员", Draft: draft}, &view); err != nil {
		return err
	}
	id := view.Application.ID
	meta := func(request, actor string) application.CommandMeta {
		return application.CommandMeta{ExpectedRevision: view.Application.Revision, RequestID: request, Actor: actor}
	}
	if err := c.json(ctx, http.MethodPost, "/api/applications/"+id+"/assess", application.AssessCommand{Meta: meta("selftest-assess-1", "自检审查员")}, &view); err != nil {
		return err
	}
	submit := application.SubmitCommand{Meta: meta("selftest-submit", "自检编制员")}
	if err := c.json(ctx, http.MethodPost, "/api/applications/"+id+"/submit", submit, &view); err != nil {
		return err
	}
	if err := c.json(ctx, http.MethodPost, "/api/applications/"+id+"/submit", submit, &view); err != nil {
		return fmt.Errorf("提交幂等校验失败: %w", err)
	}
	photos := make([]domain.PhotoRecord, 0, len(domain.RequiredPhotoCategories))
	for index, category := range domain.RequiredPhotoCategories {
		photos = append(photos, domain.PhotoRecord{FileName: fmt.Sprintf("selftest-%d.jpg", index+1), Category: category, TakenAt: fmt.Sprintf("2026-11-02T08:%02d", 30+index)})
	}
	evidence := domain.EvidenceInput{CapturedBy: "自检现场员", Latitude: 31.230416, Longitude: 121.473701, Observations: "树势稳定，施工通道和目标树穴均符合方案", MeasureChecks: confirmedMeasures(), PhotoRecords: photos}
	if err := c.json(ctx, http.MethodPost, "/api/applications/"+id+"/site-evidence", application.SiteCommand{Meta: meta("selftest-site", "自检现场员"), Evidence: evidence}, &view); err != nil {
		return err
	}
	review := domain.ReviewInput{Reviewer: "自检专家", Outcome: domain.OutcomeRectification, Comments: "运输保湿记录需落实责任人", Matrix: reviewMatrix(view.ReviewMatrix, true, "运输保湿责任需进一步明确")}
	if err := c.json(ctx, http.MethodPost, "/api/applications/"+id+"/review", application.ReviewCommand{Meta: meta("selftest-return", "自检专家"), Review: review}, &view); err != nil {
		return err
	}
	if len(view.OpenRectifications) != 1 {
		return fmt.Errorf("自检未获得整改项")
	}
	responses := []domain.RectificationResponse{{ItemID: view.OpenRectifications[0].ID, Explanation: "已明确现场员每小时记录含水状态", Materials: []domain.ReplacementMaterial{{Name: "运输保湿记录表", Category: "现场记录", VersionNote: "V2 补正版", ContentDigest: domain.Digest("selftest-moisture-v2")}}}}
	if err := c.json(ctx, http.MethodPost, "/api/applications/"+id+"/rectifications", application.RectifyCommand{Meta: meta("selftest-rectify", "自检编制员"), Responses: responses}, &view); err != nil {
		return err
	}
	if err := c.json(ctx, http.MethodPost, "/api/applications/"+id+"/assess", application.AssessCommand{Meta: meta("selftest-assess-2", "自检审查员")}, &view); err != nil {
		return err
	}
	if err := c.json(ctx, http.MethodPost, "/api/applications/"+id+"/resubmit", application.ResubmitCommand{Meta: meta("selftest-resubmit", "自检编制员")}, &view); err != nil {
		return err
	}
	approval := domain.ReviewInput{Reviewer: "自检专家", Outcome: domain.OutcomeApproved, Comments: "方案和补正材料符合保护要求", Matrix: reviewMatrix(view.ReviewMatrix, false, "核查依据完整并符合要求")}
	if err := c.json(ctx, http.MethodPost, "/api/applications/"+id+"/review", application.ReviewCommand{Meta: meta("selftest-approve", "自检专家"), Review: approval}, &view); err != nil {
		return err
	}
	if view.Application.Status != domain.StatusArchived || view.Application.Archive == nil {
		return fmt.Errorf("自检归档状态断言失败")
	}
	archivedRevision := view.Application.Revision
	var receipt domain.ArchiveIntegrityReceipt
	if err := c.json(ctx, http.MethodPost, "/api/applications/"+id+"/archive-integrity", application.VerifyArchiveCommand{RequestID: "selftest-integrity", Actor: "自检归档员"}, &receipt); err != nil {
		return err
	}
	if !receipt.Passed {
		return fmt.Errorf("自检归档完整性核验未通过")
	}
	if err := c.json(ctx, http.MethodGet, "/api/applications/"+id, nil, &view); err != nil {
		return err
	}
	if view.Application.Revision != archivedRevision {
		return fmt.Errorf("归档核验不应改变申请修订号")
	}
	var archiveHTML string
	if err := c.text(ctx, "/archive/"+id, &archiveHTML); err != nil {
		return err
	}
	if len(archiveHTML) < 500 {
		return fmt.Errorf("自检归档页面内容不完整")
	}
	return nil
}

func reviewMatrix(items []domain.ReviewMatrixItem, rectifyFirst bool, note string) []domain.ReviewMatrixInput {
	result := make([]domain.ReviewMatrixInput, 0, len(items))
	for index, item := range items {
		conclusion := domain.MatrixPassed
		if rectifyFirst && index == 0 {
			conclusion = domain.MatrixRectification
		}
		result = append(result, domain.ReviewMatrixInput{ID: item.ID, Conclusion: conclusion, ExpertNote: note, EvidenceReferences: []string{item.SourceID}})
	}
	return result
}

func confirmedMeasures() []domain.MeasureCheck {
	result := make([]domain.MeasureCheck, 0, len(domain.RequiredMeasures))
	for _, code := range domain.RequiredMeasures {
		result = append(result, domain.MeasureCheck{Code: code, Confirmed: true})
	}
	return result
}

func (c *selftestClient) json(ctx context.Context, method, path string, body, target any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("自检请求 %s: %w", path, err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("自检请求 %s 返回 %d: %s", path, response.StatusCode, string(responseBody))
	}
	if err := json.Unmarshal(responseBody, target); err != nil {
		return fmt.Errorf("解析自检响应 %s: %w", path, err)
	}
	return nil
}

func (c *selftestClient) text(ctx context.Context, path string, target *string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	response, err := c.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("自检页面返回 %d", response.StatusCode)
	}
	*target = string(body)
	return nil
}
