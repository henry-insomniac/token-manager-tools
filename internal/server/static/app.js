const state = {
  profiles: [],
  selectedName: null,
  busy: new Set(),
  autoRefreshStarted: false,
  autoSwitch: {
    enabled: false,
    events: [],
  },
};

const els = {
  list: document.querySelector("#profile-list"),
  detail: document.querySelector("#detail-panel"),
  toast: document.querySelector("#toast"),
  form: document.querySelector("#create-form"),
  input: document.querySelector("#profile-name"),
  refresh: document.querySelector("#refresh-button"),
  total: document.querySelector("#summary-total"),
  checked: document.querySelector("#summary-checked"),
  fiveHourRisk: document.querySelector("#summary-fivehour-risk"),
  weekRisk: document.querySelector("#summary-week-risk"),
  autoSwitchBadge: document.querySelector("#auto-switch-badge"),
  autoSwitchToggle: document.querySelector("#auto-switch-toggle"),
  autoSwitchRun: document.querySelector("#auto-switch-run"),
  autoSwitchMeta: document.querySelector("#auto-switch-meta"),
  autoSwitchEvents: document.querySelector("#auto-switch-events"),
};

els.form.addEventListener("submit", async (event) => {
  event.preventDefault();
  const name = els.input.value.trim();
  if (!name) {
    showToast("先输入槽位名");
    return;
  }
  await runTask("create", async () => {
    await api("/api/profiles", { method: "POST", body: { name } });
    els.input.value = "";
    showToast(`已创建 ${name}`);
    await loadDashboard();
  });
});

els.refresh.addEventListener("click", () => refreshAllUsage());
els.autoSwitchToggle.addEventListener("click", () => toggleAutoSwitch());
els.autoSwitchRun.addEventListener("click", () => runAutoSwitchCheck());

bootstrap();

async function bootstrap() {
  await loadDashboard();
  window.setInterval(() => {
    if (document.hidden) {
      return;
    }
    silentRefreshDashboard();
  }, 15000);
}

async function loadDashboard() {
  await Promise.all([loadProfiles(), loadAutoSwitchStatus()]);
}

async function loadProfiles() {
  await runTask("load", async () => {
    const data = await api("/api/profiles");
    applyProfiles(data.profiles ?? []);
    render();
    maybeAutoRefreshUsage();
  });
}

function render() {
  renderSummary();
  renderAutoSwitch();
  renderList();
  renderDetail();
}

function renderSummary() {
  const managed = state.profiles.filter((profile) => !profile.isDefault);
  els.total.textContent = managed.length;
  els.checked.textContent = managed.filter((profile) => profile.cachedProbe).length;
  els.fiveHourRisk.textContent = managed.filter((profile) => isFiveHourRisk(profile)).length;
  els.weekRisk.textContent = managed.filter((profile) => isWeekRisk(profile)).length;
}

function renderList() {
  const managed = state.profiles.filter((profile) => !profile.isDefault);
  if (managed.length === 0) {
    els.list.innerHTML = `
      <div class="empty-state">
        <p class="empty-title">还没有账号槽位</p>
        <p class="muted">先创建一个槽位，再登录 Codex/OpenAI 账号。</p>
      </div>
    `;
    return;
  }
  els.list.replaceChildren(...managed.map(renderProfileRow));
}

function renderProfileRow(profile) {
  const row = document.createElement("article");
  row.className = `profile-row ${profile.name === state.selectedName ? "is-selected" : ""}`;
  row.tabIndex = 0;
  row.setAttribute("role", "button");
  row.addEventListener("click", () => {
    state.selectedName = profile.name;
    render();
  });
  row.addEventListener("keydown", (event) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      state.selectedName = profile.name;
      render();
    }
  });

  const main = document.createElement("div");
  main.className = "profile-main";
  main.innerHTML = `
    <p class="profile-name">
      <span>${escapeHTML(profile.name)}</span>
      <span class="badge ${badgeClass(profile)}">${escapeHTML(statusLabel(profile))}</span>
      ${profile.isActive ? '<span class="badge ok">当前</span>' : ""}
    </p>
    <p class="profile-meta">${escapeHTML(profile.accountEmail || profile.accountId || profile.statusReason || "未登录")}</p>
    ${renderListQuota(profile)}
  `;

  const actions = document.createElement("div");
  actions.className = "profile-actions";
  actions.addEventListener("click", (event) => event.stopPropagation());
  actions.append(
    actionButton("登录", () => startLogin(profile.name)),
    actionButton("检查", () => probeProfile(profile.name)),
    actionButton("切换", () => activateProfile(profile.name), profile.isActive),
    actionButton("移除槽位", () => removeProfile(profile.name), false, "danger"),
  );

  row.append(main, actions);
  return row;
}

function renderDetail() {
  const profile = state.profiles.find((item) => item.name === state.selectedName);
  if (!profile) {
    els.detail.innerHTML = `
      <p class="empty-title">选择一个槽位</p>
      <p class="muted">左侧只保留关键信息。路径、认证文件和错误细节会显示在这里。</p>
    `;
    return;
  }
  els.detail.innerHTML = `
    <h2>${escapeHTML(profile.name)}</h2>
    <p class="muted">${escapeHTML(profile.statusReason || "未提供状态原因")}</p>
    ${renderDetailUsage(profile)}
    <div class="detail-grid">
      ${detailItem("账号", profile.accountEmail || profile.accountId || "未登录")}
      ${detailItem("套餐", profile.cachedProbe?.usage?.plan || "未提供")}
      ${detailItem("上次检查", formatDateTime(profile.cachedProbe?.lastProbeAt) || "未检查")}
      ${detailItem("状态目录", profile.stateDir)}
      ${detailItem("Codex 目录", profile.codexHome)}
      ${detailItem("OpenClaw 配置", profile.configPath)}
      ${detailItem("认证池", profile.authStorePath)}
      ${detailItem("Codex 认证", profile.codexAuthPath)}
    </div>
  `;
}

function renderAutoSwitch() {
  const status = state.autoSwitch || { enabled: false, events: [] };
  els.autoSwitchBadge.textContent = status.enabled ? "开启" : "关闭";
  els.autoSwitchBadge.className = `badge ${status.enabled ? "ok" : ""}`;
  els.autoSwitchToggle.textContent = status.enabled ? "关闭" : "开启";
  els.autoSwitchMeta.innerHTML = `
    ${detailItem("最近检查", formatDateTime(status.lastCheckedAt) || "未检查")}
    ${detailItem("最近切换", formatDateTime(status.lastSwitchedAt) || "未切换")}
    ${detailItem("状态", compactAutoSwitchMessage(status.lastMessage || "自动切换未开启"))}
  `;
  if (!status.events?.length) {
    els.autoSwitchEvents.innerHTML = `<p class="muted auto-switch-empty">暂无记录</p>`;
    return;
  }
  els.autoSwitchEvents.innerHTML = status.events
    .map((event) => {
      const parts = [compactAutoSwitchMessage(event.message)];
      if (event.reason) {
        parts.push(`原因：${event.reason}`);
      }
      return `
        <article class="auto-switch-event">
          <header>
            <strong>${escapeHTML(formatDateTime(event.at) || event.at)}</strong>
            <span>${escapeHTML(autoSwitchEventLabel(event.type))}</span>
          </header>
          <p>${escapeHTML(parts.join(" · "))}</p>
        </article>
      `;
    })
    .join("");
}

function renderListQuota(profile) {
  if (!profile.cachedProbe) {
    return `
      <div class="quota-row">
        <div class="quota-chip">
          <header><strong>额度</strong><span>未检查</span></header>
          <div class="quota-bar"><i style="width: 0%"></i></div>
          <p class="quota-note">点“检查”或“刷新额度”后显示。</p>
        </div>
      </div>
    `;
  }
  return `
    <div class="quota-row">
      ${quotaChip("5 小时", profile.cachedProbe.usage?.fiveHour)}
      ${quotaChip("本周", profile.cachedProbe.usage?.week)}
    </div>
  `;
}

function renderDetailUsage(profile) {
  if (!profile.cachedProbe) {
    return `
      <div class="detail-usage">
        <div class="detail-usage-card">
          <header><strong>额度概览</strong><span>未检查</span></header>
          <p class="muted">点击“检查”或顶部“刷新额度”后，会显示 5 小时和本周余量。</p>
        </div>
      </div>
    `;
  }
  return `
    <div class="detail-usage">
      <div class="detail-usage-card">
        <header><strong>5 小时额度</strong><span>${usagePercentLabel(profile.cachedProbe.usage?.fiveHour)}</span></header>
        ${usageBar(profile.cachedProbe.usage?.fiveHour)}
        <p class="quota-note">${quotaResetLabel(profile.cachedProbe.usage?.fiveHour)}</p>
      </div>
      <div class="detail-usage-card">
        <header><strong>本周额度</strong><span>${usagePercentLabel(profile.cachedProbe.usage?.week)}</span></header>
        ${usageBar(profile.cachedProbe.usage?.week)}
        <p class="quota-note">${quotaResetLabel(profile.cachedProbe.usage?.week)}</p>
      </div>
    </div>
  `;
}

function detailItem(label, value) {
  return `
    <div class="detail-item">
      <label>${escapeHTML(label)}</label>
      <code>${escapeHTML(value || "未提供")}</code>
    </div>
  `;
}

function actionButton(label, handler, disabled = false, variant = "") {
  const button = document.createElement("button");
  button.type = "button";
  button.textContent = label;
  button.className = `profile-action ${variant}`.trim();
  button.disabled = disabled;
  button.addEventListener("click", handler);
  return button;
}

async function startLogin(name) {
  const popup = window.open("about:blank", "_blank");
  await runTask(`login:${name}`, async () => {
    const data = await api(`/api/profiles/${encodeURIComponent(name)}/login/start`, { method: "POST" });
    if (popup) {
      popup.opener = null;
      popup.location.href = data.authUrl;
    } else {
      window.location.href = data.authUrl;
    }
    showToast("已打开登录页。登录完成后刷新账号池。");
  });
}

async function probeProfile(name) {
  await runTask(`probe:${name}`, async () => {
    const result = await api(`/api/profiles/${encodeURIComponent(name)}/probe`, { method: "POST" });
    showToast(`${name}: ${result.reason}`);
    await loadDashboard();
  });
}

async function refreshAllUsage() {
  await runTask("refresh-usage", async () => {
    const result = await api("/api/usage/refresh", { method: "POST" });
    const failedCount = Object.keys(result.failed || {}).length;
    showToast(`已刷新 ${result.refreshed?.length || 0} 个槽位${failedCount ? `，失败 ${failedCount} 个` : ""}`);
    await loadDashboard();
  });
}

function maybeAutoRefreshUsage() {
  if (state.autoRefreshStarted) {
    return;
  }
  const managed = state.profiles.filter((profile) => !profile.isDefault);
  const hasCredentialedProfile = managed.some((profile) => profile.hasCredential);
  const hasCachedProbe = managed.some((profile) => profile.cachedProbe);
  if (!hasCredentialedProfile || hasCachedProbe) {
    return;
  }
  state.autoRefreshStarted = true;
  refreshAllUsage();
}

async function loadAutoSwitchStatus() {
  await runTask("auto-switch:load", async () => {
    state.autoSwitch = await api("/api/auto-switch");
    renderAutoSwitch();
  });
}

async function toggleAutoSwitch() {
  await runTask("auto-switch:toggle", async () => {
    const result = await api("/api/auto-switch", {
      method: "PATCH",
      body: { enabled: !state.autoSwitch?.enabled },
    });
    state.autoSwitch = result.status;
    renderAutoSwitch();
    await loadProfiles();
    showToast(state.autoSwitch.enabled ? "自动切换已开启" : "自动切换已关闭");
  });
}

async function runAutoSwitchCheck() {
  await runTask("auto-switch:run", async () => {
    const result = await api("/api/auto-switch/run", { method: "POST" });
    state.autoSwitch = result.status;
    renderAutoSwitch();
    await loadProfiles();
    showToast(result.switched ? state.autoSwitch.lastMessage || "已自动切换账号" : state.autoSwitch.lastMessage || "已完成自动切换检查");
  });
}

async function activateProfile(name) {
  await runTask(`activate:${name}`, async () => {
    await api(`/api/profiles/${encodeURIComponent(name)}/activate`, { method: "POST" });
    showToast(`已切换到 ${name}`);
    await loadDashboard();
  });
}

async function removeProfile(name) {
  if (!confirm(`移除本机槽位 ${name}？本地资料会归档，远端账号不会被删除。`)) {
    return;
  }
  await runTask(`remove:${name}`, async () => {
    const result = await api(`/api/profiles/${encodeURIComponent(name)}/remove`, { method: "POST" });
    showToast(result.message || `已移除 ${name}`);
    await loadDashboard();
  });
}

async function api(path, options = {}) {
  const init = {
    method: options.method ?? "GET",
    headers: {},
  };
  if (options.body) {
    init.headers["Content-Type"] = "application/json";
    init.body = JSON.stringify(options.body);
  }
  const response = await fetch(path, init);
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(data.error || `请求失败: ${response.status}`);
  }
  return data;
}

async function runTask(key, task) {
  if (state.busy.has(key)) {
    return;
  }
  state.busy.add(key);
  document.body.classList.add("is-busy");
  try {
    await task();
  } catch (error) {
    showToast(error.message || String(error));
  } finally {
    state.busy.delete(key);
    if (state.busy.size === 0) {
      document.body.classList.remove("is-busy");
    }
  }
}

function showToast(message) {
  els.toast.textContent = message;
  els.toast.classList.add("is-visible");
  clearTimeout(showToast.timer);
  showToast.timer = setTimeout(() => els.toast.classList.remove("is-visible"), 3600);
}

function statusLabel(profile) {
  if (profile.status === "healthy") return "可用";
  if (profile.status === "reauth_required") return "需登录";
  if (profile.status === "cooldown") return "冷却";
  if (profile.status === "exhausted") return "耗尽";
  return profile.status || "未知";
}

function badgeClass(profile) {
  if (profile.status === "healthy") return "ok";
  if (profile.status === "reauth_required" || profile.status === "cooldown") return "warn";
  if (profile.status === "exhausted") return "danger";
  return "";
}

function quotaChip(label, window) {
  return `
    <div class="quota-chip">
      <header><strong>${escapeHTML(label)}</strong><span>${usagePercentLabel(window)}</span></header>
      ${usageBar(window)}
      <p class="quota-note">${quotaResetLabel(window)}</p>
    </div>
  `;
}

function usageBar(window) {
  const left = Math.max(0, Math.min(100, window?.leftPercent ?? 0));
  return `<div class="quota-bar ${barClass(left)}"><i style="width: ${left}%"></i></div>`;
}

function usagePercentLabel(window) {
  if (!window) return "未提供";
  return `剩余 ${window.leftPercent}%`;
}

function quotaResetLabel(window) {
  if (!window) return "未提供额度窗口";
  if (window.resetAt) {
    return `重置时间 ${formatDateTime(window.resetAt) || window.resetAt}`;
  }
  return "未提供重置时间";
}

function barClass(leftPercent) {
  if (leftPercent <= 0) return "is-danger";
  if (leftPercent <= 20) return "is-danger";
  if (leftPercent <= 40) return "is-warn";
  return "";
}

function isFiveHourRisk(profile) {
  const left = profile.cachedProbe?.usage?.fiveHour?.leftPercent;
  return typeof left === "number" && left <= 20;
}

function isWeekRisk(profile) {
  const left = profile.cachedProbe?.usage?.week?.leftPercent;
  return typeof left === "number" && left <= 20;
}

function formatDateTime(value) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function applyProfiles(profiles) {
  state.profiles = profiles;
  if (!state.selectedName && state.profiles.length > 0) {
    state.selectedName = state.profiles.find((profile) => !profile.isDefault)?.name ?? state.profiles[0].name;
  }
  if (!state.profiles.some((profile) => profile.name === state.selectedName)) {
    state.selectedName = state.profiles[0]?.name ?? null;
  }
}

function autoSwitchEventLabel(type) {
  if (type === "switch") return "自动切换";
  if (type === "enabled") return "已开启";
  if (type === "disabled") return "已关闭";
  return "状态";
}

function compactAutoSwitchMessage(message) {
  if (!message) return "未开启";
  if (message === "自动切换未开启") return "未开启";
  if (message === "自动切换已开启") return "已开启";
  if (message === "自动切换已关闭") return "已关闭";
  if (message === "没有可自动切换的可用账号") return "无可切账号";

  const healthyMatch = message.match(/^(.+?) 额度可用，无需切换$/);
  if (healthyMatch) return `${healthyMatch[1]} 可用`;

  const switchedMatch = message.match(/^已自动切换到 (.+)$/);
  if (switchedMatch) return `已切到 ${switchedMatch[1]}`;

  const foundMatch = message.match(/^已找到 (.+?)，但距离上次自动切换过近，暂不重复切换$/);
  if (foundMatch) return `已找到 ${foundMatch[1]}，暂不切`;

  const failedProbeMatch = message.match(/^(.+?) 检查失败，暂不自动切换$/);
  if (failedProbeMatch) return `${failedProbeMatch[1]} 检查失败`;

  const switchFailMatch = message.match(/^自动切换到 (.+?) 失败$/);
  if (switchFailMatch) return `切到 ${switchFailMatch[1]} 失败`;

  if (message === "读取账号池失败，暂不自动切换") return "读取失败";
  return message;
}

async function silentRefreshDashboard() {
  try {
    const [profiles, autoSwitch] = await Promise.all([
      api("/api/profiles"),
      api("/api/auto-switch"),
    ]);
    applyProfiles(profiles.profiles ?? []);
    state.autoSwitch = autoSwitch;
    render();
  } catch {
    // Ignore silent polling errors; manual actions still surface errors.
  }
}

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}
