"use strict";

const MATERIALS = ["树木现状照片", "迁移路线图", "目标地检测报告", "专家踏勘记录", "交通组织方案"];
const MEASURES = ["根球包扎完成", "树冠固定完成", "运输保湿就绪", "目标树穴验收"];
const STATUS = {
  draft: "草稿",
  site_pending: "待现场核验",
  expert_review: "待专家复核",
  rectifying: "整改中",
  archived: "已归档"
};

const state = { applications: [], view: null, search: "", activeTab: "plan", creating: false, sitePhotos: [] };
const $ = (selector) => document.querySelector(selector);
const $$ = (selector) => [...document.querySelectorAll(selector)];

function escapeHTML(value) {
  return String(value ?? "").replace(/[&<>'"]/g, char => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "'": "&#39;", '"': "&quot;" })[char]);
}

function requestID(prefix) {
  const suffix = globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  return `${prefix}-${suffix}`;
}

async function api(path, options = {}) {
  const response = await fetch(path, { ...options, headers: { "Content-Type": "application/json", ...(options.headers || {}) } });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) {
    const error = new Error(body.error?.message || `请求失败（${response.status}）`);
    error.details = body.error?.details;
    throw error;
  }
  return body;
}

function toast(message, isError = false) {
  const node = $("#toast");
  node.textContent = message;
  node.className = `toast show${isError ? " error" : ""}`;
  clearTimeout(toast.timer);
  toast.timer = setTimeout(() => { node.className = "toast"; }, 3200);
}

function isoDate(offsetDays) {
  const date = new Date();
  date.setDate(date.getDate() + offsetDays);
  return date.toISOString().slice(0, 10);
}

function blankApplication() {
  return {
    id: "",
    tree_code: "",
    species: "",
    current_location: "",
    target_location: "",
    migration_reason: "",
    planned_window: { start: isoDate(45), end: isoDate(48) },
    protection_plan: {
      root_radius_meters: 3.6,
      transport_hours: 2,
      target_soil_ready: true,
      target_drainage_ready: true,
      canopy_protection: "分层收拢树冠并使用软质绑带固定，保留通风间隙。",
      root_ball_protection: "按测算半径起挖根球，麻布与钢丝网双层包扎。",
      transport_protection: "使用专用树木运输车辆固定，途中每小时检查保湿。",
      post_planting_care: "定植后设置支撑，连续监测土壤含水率并开展复壮养护。",
      attached_materials: ["树木现状照片", "迁移路线图", "目标地检测报告"],
      estimated_trunk_diameter_cm: 60
    },
    status: "draft",
    revision: 0,
    evidence: [], reviews: [], rectifications: [], timeline: []
  };
}

async function loadApplications(selectFirst = false) {
  const result = await api("/api/applications");
  state.applications = result.applications || [];
  renderList();
  if (selectFirst && !state.view && state.applications.length) await selectApplication(state.applications[0].id);
}

function renderList() {
  const query = state.search.trim().toLowerCase();
  const apps = state.applications.filter(app => [app.tree_code, app.species, app.target_location].join(" ").toLowerCase().includes(query));
  $("#application-count").textContent = `${apps.length} 项`;
  $("#application-list").innerHTML = apps.length ? apps.map(app => `
    <button class="application-item ${state.view?.application.id === app.id ? "active" : ""}" data-id="${escapeHTML(app.id)}" type="button">
      <strong>${escapeHTML(app.tree_code || "未填写编号")}</strong>
      <span>${escapeHTML(app.species || "未填写树种")}</span>
      <span class="row"><span>${escapeHTML(STATUS[app.status] || app.status)}</span><span>R${app.revision}</span></span>
    </button>`).join("") : '<div class="none-record">暂无匹配申请</div>';
  $$(".application-item").forEach(button => button.addEventListener("click", () => selectApplication(button.dataset.id)));
}

async function selectApplication(id) {
  try {
    state.view = await api(`/api/applications/${encodeURIComponent(id)}`);
    state.creating = false;
    state.activeTab = state.view.application.status === "archived" ? "archive" : "plan";
    renderAll();
  } catch (error) {
    toast(error.details?.application_id ? `${error.message}：${error.details.application_id}（${STATUS[error.details.status] || error.details.status}，R${error.details.revision}）` : error.message, true);
    if (error.details?.application_id) await selectApplication(error.details.application_id);
  }
}

function beginCreate() {
  state.creating = true;
  state.view = {
    application: blankApplication(), persistence_events: [], open_rectifications: [],
    can_edit: true, can_assess: false, can_submit: false, can_site: false, can_review: false, can_rectify: false
  };
  state.activeTab = "plan";
  renderAll();
  $("[name=tree_code]").focus();
}

function renderAll() {
  const hasView = Boolean(state.view);
  $("#empty-state").classList.toggle("hidden", hasView);
  $("#detail").classList.toggle("hidden", !hasView);
  renderList();
  if (!hasView) return;
  const app = state.view.application;
  $("#detail-id").textContent = app.id || "新申请";
  $("#detail-title").textContent = app.tree_code || "未命名迁移申请";
  $("#detail-subtitle").textContent = [app.species, app.current_location && `现址：${app.current_location}`].filter(Boolean).join(" · ") || "填写树木身份与迁移方案";
  $("#status-badge").textContent = state.creating ? "新建草稿" : STATUS[app.status] || app.status;
  $("#revision-badge").textContent = state.creating ? "未保存" : `修订 R${app.revision}`;
  renderTabs();
  renderDraft();
  renderAssessment();
  renderEvidence();
  renderReview();
  renderArchive();
}

function renderTabs() {
  $$(".tab").forEach(tab => tab.classList.toggle("active", tab.dataset.tab === state.activeTab));
  $$(".tab-panel").forEach(panel => panel.classList.toggle("active", panel.id === `tab-${state.activeTab}`));
}

function setField(name, value) {
  const field = $(`[name="${name}"]`);
  if (!field) return;
  if (field.type === "checkbox") field.checked = Boolean(value); else field.value = value ?? "";
}

function renderDraft() {
  const app = state.view.application;
  const plan = app.protection_plan || {};
  const fields = {
    tree_code: app.tree_code, species: app.species, current_location: app.current_location,
    target_location: app.target_location, migration_reason: app.migration_reason,
    window_start: app.planned_window?.start, window_end: app.planned_window?.end,
    trunk_diameter: plan.estimated_trunk_diameter_cm, root_radius: plan.root_radius_meters,
    transport_hours: plan.transport_hours, soil_ready: plan.target_soil_ready,
    drainage_ready: plan.target_drainage_ready, canopy: plan.canopy_protection,
    root_ball: plan.root_ball_protection, transport: plan.transport_protection, care: plan.post_planting_care
  };
  Object.entries(fields).forEach(([name, value]) => setField(name, value));
  $("#materials").innerHTML = MATERIALS.map(material => `<label class="check"><input type="checkbox" value="${escapeHTML(material)}" ${plan.attached_materials?.includes(material) ? "checked" : ""}> ${escapeHTML(material)}</label>`).join("");
  const editable = state.creating || state.view.can_edit;
  $$("#draft-form input, #draft-form textarea, #draft-form select").forEach(field => field.disabled = !editable);
  $("#save-draft").classList.toggle("hidden", !editable);
  $("#assess-from-plan").classList.toggle("hidden", !state.view.can_assess);
  const history = state.view.historical_archives || [];
  $("#tree-code-notices").innerHTML = history.length ? `<div class="assessment-summary"><strong>发现 ${history.length} 条同编号历史归档</strong>${history.map(item => `<p><button class="link-button history-link" data-id="${escapeHTML(item.application_id)}" type="button">${escapeHTML(item.application_id)}</button> · 批准 R${item.approved_revision} · ${formatTime(item.archived_at)}</p>`).join("")}</div>` : "";
  $$(".history-link").forEach(button => button.addEventListener("click", () => selectApplication(button.dataset.id)));
  const snapshot = state.view.locked_snapshot;
  const differences = state.view.snapshot_differences || [];
  $("#locked-snapshot-panel").innerHTML = snapshot ? `<div class="assessment-summary pass"><strong>当前核验依据：${escapeHTML(snapshot.id)}</strong><p>提交修订 R${snapshot.submitted_revision} · 内容摘要 <code>${escapeHTML(snapshot.content_digest)}</code></p>${differences.length ? `<p>整改后当前方案有 ${differences.length} 处差异，仅供对照。</p>${differences.map(diff => `<div class="record"><strong>${escapeHTML(diff.field)}</strong><p>锁定值：${escapeHTML(JSON.stringify(diff.locked_value))}</p><p>当前值：${escapeHTML(JSON.stringify(diff.current_value))}</p></div>`).join("")}` : "<p>当前方案与锁定方案一致。</p>"}</div>` : "";
}

function draftFromForm() {
  const form = $("#draft-form");
  return {
    tree_code: form.elements.tree_code.value,
    species: form.elements.species.value,
    current_location: form.elements.current_location.value,
    target_location: form.elements.target_location.value,
    migration_reason: form.elements.migration_reason.value,
    planned_window: { start: form.elements.window_start.value, end: form.elements.window_end.value },
    protection_plan: {
      root_radius_meters: Number(form.elements.root_radius.value),
      transport_hours: Number(form.elements.transport_hours.value),
      target_soil_ready: form.elements.soil_ready.checked,
      target_drainage_ready: form.elements.drainage_ready.checked,
      canopy_protection: form.elements.canopy.value,
      root_ball_protection: form.elements.root_ball.value,
      transport_protection: form.elements.transport.value,
      post_planting_care: form.elements.care.value,
      attached_materials: $$("#materials input:checked").map(input => input.value),
      estimated_trunk_diameter_cm: Number(form.elements.trunk_diameter.value)
    }
  };
}

function findingHTML(finding, kind) {
  return `<article class="finding ${kind}"><strong>${escapeHTML(finding.message)}</strong><code>${escapeHTML(finding.field)} · ${escapeHTML(finding.code)}</code></article>`;
}

function renderAssessment() {
  const app = state.view.application;
  const assessment = app.assessment;
  const blocking = assessment?.blocking_findings || [];
  const warnings = assessment?.warning_findings || [];
  $("#finding-count").textContent = assessment ? String(blocking.length + warnings.length) : "";
  $("#assessment-meta").textContent = assessment ? `规则集 ${assessment.rule_set_version} · 核查修订 R${assessment.application_revision}` : "尚未核查";
  $("#assessment-summary").className = `assessment-summary${assessment && !blocking.length ? " pass" : ""}`;
  $("#assessment-summary").textContent = !assessment ? "当前版本尚未执行规则核查。" : blocking.length ? `发现 ${blocking.length} 个阻断项和 ${warnings.length} 个警示项。` : `阻断项已清零，保留 ${warnings.length} 个警示项。`;
  $("#blocking-findings").innerHTML = blocking.length ? blocking.map(item => findingHTML(item, "blocking")).join("") : '<div class="none-record">无阻断项</div>';
  const dispositions = app.warning_dispositions || [];
  $("#warning-findings").innerHTML = warnings.length ? warnings.map(item => {
    const disposition = dispositions.find(record => record.finding_code === item.code && record.assessment_digest === assessment.result_digest);
    return `<article class="finding warning warning-disposition" data-code="${escapeHTML(item.code)}"><strong>${escapeHTML(item.message)}</strong><code>${escapeHTML(item.field)} · ${escapeHTML(item.code)}</code><label>处置选择<select name="warning_action"><option value="mitigated" ${disposition?.action === "mitigated" ? "selected" : ""}>已采取措施</option><option value="acknowledged" ${disposition?.action === "acknowledged" ? "selected" : ""}>知悉风险</option></select></label><label>处置说明<textarea name="warning_note" rows="2">${escapeHTML(disposition?.note || "")}</textarea></label><label>经办人<input name="warning_handler" value="${escapeHTML(disposition?.handled_by || "")}"></label><button class="button secondary save-warning" type="button">保存处置</button></article>`;
  }).join("") : '<div class="none-record">无警示项</div>';
  $("#assess-button").classList.toggle("hidden", !state.view.can_assess);
  $("#submit-button").classList.toggle("hidden", !state.view.can_submit || app.status !== "draft");
  $("#submit-button").textContent = state.view.missing_warning_count ? `尚有 ${state.view.missing_warning_count} 项警示未处置` : "提交并锁定版本";
  const correctionSubmitted = latestCorrectionMatchesReview(app);
  $("#resubmit-button").classList.toggle("hidden", !(app.status === "rectifying" && correctionSubmitted && assessment && !blocking.length && assessment.application_revision === app.revision - 1));
}

function renderEvidence() {
  const app = state.view.application;
  $("#evidence-form").classList.toggle("hidden", !state.view.can_site);
  const draft = app.evidence_draft || {};
  const checks = new Map((draft.measure_checks || []).map(item => [item.code, item]));
  $("#measure-checks").innerHTML = MEASURES.map(code => `<label class="check"><input type="checkbox" value="${escapeHTML(code)}" ${checks.get(code)?.confirmed ? "checked" : ""}> ${escapeHTML(code)}</label>`).join("");
  if (state.view.can_site) {
    setField("captured_by", draft.captured_by);
    setField("observations", draft.observations);
    setField("latitude", draft.latitude || "");
    setField("longitude", draft.longitude || "");
    state.sitePhotos = (draft.photo_records || []).map(photo => ({ ...photo }));
  }
  renderSitePhotos();
  const progress = state.view.evidence_progress || { missing_items: [] };
  $("#evidence-progress").className = `assessment-summary full${progress.missing_items?.length ? "" : " pass"}`;
  $("#evidence-progress").textContent = progress.missing_items?.length ? `尚缺：${progress.missing_items.join("、")}` : "现场证据已经齐套，可以最终确认。";
  $("#confirm-evidence").disabled = !state.view.can_site;
  const evidence = app.evidence || [];
  $("#evidence-history").innerHTML = evidence.length ? evidence.map(item => `
    <article class="record"><div class="record-head"><strong>${escapeHTML(item.captured_by)}</strong><small>${formatTime(item.captured_at)}</small></div>
    <p>${escapeHTML(item.observations)}</p><p>定位：${item.latitude.toFixed(6)}, ${item.longitude.toFixed(6)}</p>
    <small>${item.photo_records.map(photo => escapeHTML(`${photo.file_name}（${photo.category}）`)).join(" · ")}</small></article>`).join("") : '<div class="none-record">尚无现场证据</div>';
}

function renderSitePhotos() {
  $("#site-photo-list").innerHTML = state.sitePhotos.length ? state.sitePhotos.map((photo, index) => `<article class="record"><div class="record-head"><strong>${escapeHTML(photo.file_name)} · ${escapeHTML(photo.category)}</strong><button class="button secondary remove-photo" data-index="${index}" type="button">删除</button></div><small>${escapeHTML(photo.taken_at || "未填写拍摄时间")}</small></article>`).join("") : '<div class="none-record">尚未暂存照片元数据</div>';
}

async function removeSitePhoto(index) {
  const photo = state.sitePhotos[index];
  if (!photo?.id) {
    state.sitePhotos.splice(index, 1);
    renderSitePhotos();
    return;
  }
  try {
    state.view = await api(`/api/applications/${encodeURIComponent(state.view.application.id)}/site-evidence/photos/${encodeURIComponent(photo.id)}`, { method: "DELETE", body: JSON.stringify({ meta: meta("delete-photo", "现场核验人员") }) });
    renderAll();
    toast("误录照片已删除");
  } catch (error) { toast(error.message, true); }
}

function latestCorrectionMatchesReview(app) {
  if (!app.reviews?.length || !app.rectifications?.length) return false;
  return app.rectifications[app.rectifications.length - 1].source_review_id === app.reviews[app.reviews.length - 1].id;
}

function renderReview() {
  const app = state.view.application;
  $("#review-form").classList.toggle("hidden", !state.view.can_review);
  const showRectify = state.view.can_rectify && !latestCorrectionMatchesReview(app);
  $("#rectification-form").classList.toggle("hidden", !showRectify);
  $("#rectification-items").innerHTML = showRectify ? (state.view.open_rectifications || []).map((item, index) => `
    <article class="rectification-item" data-item-id="${escapeHTML(item.id)}">
      <strong>${index + 1}. ${escapeHTML(item.requirement)}</strong><small>${escapeHTML(item.finding_code || "专家要求")}</small>
      <label>补正说明<textarea name="explanation" rows="2" required></textarea></label>
      <div class="photo-row"><label>替换材料名称<input name="material_name"></label><label>材料类别<input name="material_category"></label><label>版本说明<input name="material_version"></label><label>内容摘要<input name="material_digest"></label></div>
    </article>`).join("") : "";
  const snapshot = state.view.locked_snapshot;
  $("#review-snapshot").innerHTML = snapshot ? `受审方案 ${escapeHTML(snapshot.id)} · 提交修订 R${snapshot.submitted_revision}` : "尚无受审方案快照";
  $("#review-matrix").innerHTML = state.view.can_review ? (state.view.review_matrix || []).map((item, index) => `<article class="rectification-item matrix-item" data-id="${escapeHTML(item.id)}" data-source="${escapeHTML(item.source_id)}"><strong>${index + 1}. ${escapeHTML(item.label)}</strong><small>${escapeHTML(item.kind)} · ${escapeHTML(item.source_id)}</small><label>逐项结论<select name="matrix_conclusion"><option value="passed">通过</option><option value="rectification">需整改</option></select></label><label>专家说明<textarea name="matrix_note" rows="2" required></textarea></label></article>`).join("") : "";
  const records = [];
  (app.reviews || []).forEach(review => records.push(`<article class="record"><div class="record-head"><strong>${review.outcome === "approved" ? "批准归档" : "退回整改"} · ${escapeHTML(review.reviewer)}</strong><small>${formatTime(review.decided_at)}</small></div><p>受审快照：${escapeHTML(review.snapshot_id || "旧版记录")}</p><p>${escapeHTML(review.comments)}</p>${review.matrix?.map(item => `<p>${item.conclusion === "passed" ? "通过" : "需整改"} · ${escapeHTML(item.label)} · ${escapeHTML(item.expert_note)}</p>`).join("") || ""}${review.rectification_items?.map(item => `<p>整改：${escapeHTML(item.requirement)}</p>`).join("") || ""}</article>`));
  (app.rectifications || []).forEach(correction => records.push(`<article class="record"><div class="record-head"><strong>补正差异 R${correction.before_revision} → R${correction.after_revision}</strong><small>${formatTime(correction.submitted_at)}</small></div>${correction.differences?.map(diff => `<p>${diff.result === "changed" ? "已变更" : diff.result === "explained" ? "仅说明" : "未覆盖"} · ${escapeHTML(diff.before)} → ${escapeHTML(diff.after)}</p>`).join("") || correction.difference_summary.map(diff => `<p>${escapeHTML(diff)}</p>`).join("")}${correction.bound_snapshot_id ? `<p>绑定重提快照：${escapeHTML(correction.bound_snapshot_id)} · R${correction.bound_revision}</p>` : ""}</article>`));
  $("#review-history").innerHTML = records.length ? records.join("") : '<div class="none-record">尚无复核或整改记录</div>';
}

function renderArchive() {
  const app = state.view.application;
  const archive = app.archive;
  if (!archive) { $("#archive-content").innerHTML = '<div class="none-record">批准后生成不可变归档摘要</div>'; return; }
  const receipt = state.view.latest_integrity_receipt;
  $("#archive-content").innerHTML = `
    <div class="archive-banner"><div><h3>申请已批准归档</h3><p>批准修订 R${archive.approved_revision} · ${formatTime(archive.archived_at)}</p></div><div class="form-actions"><button id="verify-archive" class="button secondary" type="button">执行完整性核验</button>${receipt?.passed ? `<a class="button primary" href="/archive/${encodeURIComponent(app.id)}" target="_blank" rel="noopener">打开打印视图</a>` : '<button class="button primary" type="button" disabled>核验通过后打印</button>'}</div></div>
    ${receipt ? `<div class="assessment-summary ${receipt.passed ? "pass" : ""}"><strong>${receipt.passed ? "归档完整性核验通过" : "归档完整性存在异常"}</strong><p>回执 ${escapeHTML(receipt.id)} · ${formatTime(receipt.checked_at)}</p>${receipt.results.map(item => `<p>${item.passed ? "通过" : "异常"} · ${escapeHTML(item.component)} · ${escapeHTML(item.record_id || "")} · ${escapeHTML(item.message)}</p>`).join("")}</div>` : '<div class="assessment-summary">尚未执行人工完整性核验。</div>'}
    <div class="section-heading"><h3>证据清单</h3><p>${archive.evidence_items.length} 组现场记录</p></div>
    <div class="record-list">${archive.evidence_items.map(item => `<article class="record"><strong>${escapeHTML(item.evidence_id)}</strong><p>${item.photos.map(escapeHTML).join(" · ")}</p><code>${escapeHTML(item.digest)}</code></article>`).join("")}</div>
    ${archive.rectification_diffs?.length ? `<div class="section-heading"><h3>整改差异</h3></div><div class="record-list">${archive.rectification_diffs.map(diff => `<div class="record">${escapeHTML(diff)}</div>`).join("")}</div>` : ""}
    <div class="section-heading"><h3>状态时间线</h3><p>完整业务变更记录</p></div>
    <div class="timeline">${(archive.timeline || []).map(event => `<div class="timeline-item"><strong>${escapeHTML(event.action)}</strong><p>${escapeHTML(STATUS[event.to] || event.to)}${event.actor ? ` · ${escapeHTML(event.actor)}` : ""}</p><small>${formatTime(event.at)}</small></div>`).join("")}</div>
    <div class="record"><small>归档完整性摘要</small><code>${escapeHTML(archive.digest)}</code></div>`;
  $("#verify-archive")?.addEventListener("click", verifyArchive);
}

function formatTime(value) {
  if (!value) return "";
  return new Intl.DateTimeFormat("zh-CN", { dateStyle: "medium", timeStyle: "short" }).format(new Date(value));
}

async function saveDraft(event) {
  event.preventDefault();
  if (!event.currentTarget.reportValidity()) return;
  try {
    const draft = draftFromForm();
    if (state.creating) {
      state.view = await api("/api/applications", { method: "POST", body: JSON.stringify({ request_id: requestID("create"), actor: "方案编制人员", draft }) });
      state.creating = false;
      toast("申请草稿已创建");
    } else {
      state.view = await api(`/api/applications/${encodeURIComponent(state.view.application.id)}/draft`, { method: "PUT", body: JSON.stringify({ meta: meta("save", "方案编制人员"), draft }) });
      toast("方案已保存，新核查前结果已清除");
    }
    await loadApplications();
    renderAll();
  } catch (error) {
    toast(error.details?.application_id ? `${error.message}：${error.details.application_id}（${STATUS[error.details.status] || error.details.status}，R${error.details.revision}）` : error.message, true);
    if (error.details?.application_id) await selectApplication(error.details.application_id);
  }
}

function meta(prefix, actor) {
  return { revision: state.view.application.revision, request_id: requestID(prefix), actor };
}

async function runAction(path, body, success, tab, method = "POST") {
  try {
    state.view = await api(`/api/applications/${encodeURIComponent(state.view.application.id)}/${path}`, { method, body: JSON.stringify(body) });
    if (tab) state.activeTab = tab;
    await loadApplications();
    renderAll();
    toast(success);
  } catch (error) { toast(error.message, true); }
}

function runAssessment() {
  runAction("assess", { meta: meta("assess", "园林保护审查人员") }, "规则核查已完成", "assessment");
}

function submitApplication() {
  runAction("submit", { meta: meta("submit", "方案编制人员") }, "方案已锁定并提交现场核验", "evidence");
}

async function saveWarning(button) {
  const item = button.closest(".warning-disposition");
  const disposition = { finding_code: item.dataset.code, action: item.querySelector('[name="warning_action"]').value, note: item.querySelector('[name="warning_note"]').value, handled_by: item.querySelector('[name="warning_handler"]').value };
  await runAction("warning-dispositions", { meta: meta("warning", disposition.handled_by || "园林保护审查人员"), disposition }, "警示处置已保存", "assessment", "PUT");
}

function addSitePhoto() {
  const form = $("#evidence-form");
  const photo = { file_name: form.elements.photo_name.value.trim(), category: form.elements.photo_category.value, taken_at: form.elements.photo_taken_at.value, location_note: "现场登记" };
  if (!photo.file_name || !photo.taken_at) { toast("请填写照片文件名和拍摄时间", true); return; }
  const duplicate = state.sitePhotos.some(item => item.file_name === photo.file_name && item.category === photo.category && item.taken_at === photo.taken_at);
  if (duplicate) { toast("相同文件名、拍摄时间和类别的照片已登记", true); return; }
  state.sitePhotos.push(photo);
  form.elements.photo_name.value = "";
  renderSitePhotos();
}

function evidenceFromForm() {
  const form = $("#evidence-form");
  return { captured_by: form.elements.captured_by.value, latitude: Number(form.elements.latitude.value), longitude: Number(form.elements.longitude.value), observations: form.elements.observations.value, measure_checks: $$("#measure-checks input").map(input => ({ code: input.value, confirmed: input.checked, note: "" })), photo_records: state.sitePhotos };
}

async function saveEvidenceDraft(showToast = true) {
  const evidence = evidenceFromForm();
  state.view = await api(`/api/applications/${encodeURIComponent(state.view.application.id)}/site-evidence/draft`, { method: "PUT", body: JSON.stringify({ meta: meta("site-draft", evidence.captured_by || "现场核验人员"), evidence }) });
  await loadApplications();
  renderAll();
  if (showToast) toast("现场证据进度已暂存");
}

async function submitEvidence(event) {
  event.preventDefault();
  try { await saveEvidenceDraft(true); } catch (error) { toast(error.message, true); }
}

async function confirmEvidence() {
  try {
    await saveEvidenceDraft(false);
    await runAction("site-evidence/confirm", { meta: meta("site-confirm", state.view.application.evidence_draft?.captured_by || "现场核验人员") }, "现场证据已冻结，申请进入专家复核", "review");
  } catch (error) { toast(error.message, true); }
}

async function submitReview(event) {
  event.preventDefault();
  if (!event.currentTarget.reportValidity()) return;
  const form = event.currentTarget;
  const outcome = form.elements.outcome.value;
  const matrix = $$(".matrix-item").map(item => ({ id: item.dataset.id, conclusion: item.querySelector('[name="matrix_conclusion"]').value, expert_note: item.querySelector('[name="matrix_note"]').value, evidence_references: [item.dataset.source] }));
  const review = { reviewer: form.elements.reviewer.value, outcome, comments: form.elements.comments.value, rectification_items: [], matrix };
  await runAction("review", { meta: meta("review", review.reviewer), review }, outcome === "approved" ? "申请已批准归档" : "整改要求已一次性下达", outcome === "approved" ? "archive" : "review");
}

async function submitRectification(event) {
  event.preventDefault();
  if (!event.currentTarget.reportValidity()) return;
  const responses = $$(".rectification-item").map(item => ({
    item_id: item.dataset.itemId,
    explanation: item.querySelector('[name="explanation"]').value,
    replacement_materials: [],
    materials: item.querySelector('[name="material_name"]').value.trim() ? [{ name: item.querySelector('[name="material_name"]').value, category: item.querySelector('[name="material_category"]').value, version_note: item.querySelector('[name="material_version"]').value, content_digest: item.querySelector('[name="material_digest"]').value }] : []
  }));
  await runAction("rectifications", { meta: meta("rectify", "方案编制人员"), responses }, "补正已提交，请重新执行规则核查", "assessment");
}

function resubmit() {
  runAction("resubmit", { meta: meta("resubmit", "方案编制人员") }, "整改版本已提交专家复核", "review");
}

async function verifyArchive() {
  try {
    await api(`/api/applications/${encodeURIComponent(state.view.application.id)}/archive-integrity`, { method: "POST", body: JSON.stringify({ request_id: requestID("integrity"), actor: "归档审查人员" }) });
    await selectApplication(state.view.application.id);
    state.activeTab = "archive";
    renderAll();
    toast("归档完整性核验回执已生成");
  } catch (error) { toast(error.message, true); }
}

function bindEvents() {
  $("#new-button").addEventListener("click", beginCreate);
  $("#search").addEventListener("input", event => { state.search = event.target.value; renderList(); });
  $$(".tab").forEach(tab => tab.addEventListener("click", () => { state.activeTab = tab.dataset.tab; renderTabs(); }));
  $("#draft-form").addEventListener("submit", saveDraft);
  $("#assess-button").addEventListener("click", runAssessment);
  $("#assess-from-plan").addEventListener("click", runAssessment);
  $("#submit-button").addEventListener("click", submitApplication);
  $("#resubmit-button").addEventListener("click", resubmit);
  $("#evidence-form").addEventListener("submit", submitEvidence);
  $("#add-photo").addEventListener("click", addSitePhoto);
  $("#confirm-evidence").addEventListener("click", confirmEvidence);
  $("#site-photo-list").addEventListener("click", event => { const button = event.target.closest(".remove-photo"); if (button) removeSitePhoto(Number(button.dataset.index)); });
  $("#warning-findings").addEventListener("click", event => { const button = event.target.closest(".save-warning"); if (button) saveWarning(button); });
  $("#review-form").addEventListener("submit", submitReview);
  $("#rectification-form").addEventListener("submit", submitRectification);
}

async function start() {
  bindEvents();
  try {
    await api("/api/health");
    $("#connection").textContent = "服务正常";
    $("#connection").classList.add("ok");
    await loadApplications(true);
  } catch (error) {
    $("#connection").textContent = "服务不可用";
    toast(error.message, true);
  }
}

start();
