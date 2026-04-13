const state = {
  profiles: [],
  selectedName: null,
  busy: new Set(),
  autoRefreshStarted: false,
  login: null,
  desktop: null,
  autoSwitch: {
    enabled: false,
    events: [],
  },
};

const LOGIN_EVENT_KEY = "token-manager-last-login";
const LOGIN_RUNTIME_EVENT = "token-manager-login-result";
const DESKTOP_ACTION_RUNTIME_EVENT = "token-manager-desktop-action";
const FOCUS_PROFILE_RUNTIME_EVENT = "token-manager-focus-profile";
const transport = window.tokenManagerTransport;
const runtime = window.runtime;

const els = {
  list: document.querySelector("#profile-list"),
  detail: document.querySelector("#detail-panel"),
  toast: document.querySelector("#toast"),
  form: document.querySelector("#create-form"),
  input: document.querySelector("#profile-name"),
  refresh: document.querySelector("#refresh-button"),
  desktopQuickbar: document.querySelector("#desktop-quickbar"),
  desktopQuickbarActive: document.querySelector("#desktop-quickbar-active"),
  desktopQuickbarMeta: document.querySelector("#desktop-quickbar-meta"),
  desktopQuickbarProfiles: document.querySelector("#desktop-quickbar-profiles"),
  desktopQuickbarRefresh: document.querySelector("#desktop-quickbar-refresh"),
  desktopQuickbarRun: document.querySelector("#desktop-quickbar-run"),
  total: document.querySelector("#summary-total"),
  checked: document.querySelector("#summary-checked"),
  fiveHourRisk: document.querySelector("#summary-fivehour-risk"),
  weekRisk: document.querySelector("#summary-week-risk"),
  autoSwitchBadge: document.querySelector("#auto-switch-badge"),
  autoSwitchToggle: document.querySelector("#auto-switch-toggle"),
  autoSwitchRun: document.querySelector("#auto-switch-run"),
  autoSwitchMeta: document.querySelector("#auto-switch-meta"),
  autoSwitchEvents: document.querySelector("#auto-switch-events"),
  desktopPanel: document.querySelector("#desktop-panel"),
  desktopPanelSummary: document.querySelector("#desktop-panel-summary"),
  desktopPanelMeta: document.querySelector("#desktop-panel-meta"),
  desktopAutoStartBadge: document.querySelector("#desktop-autostart-badge"),
  desktopAutoStartToggle: document.querySelector("#desktop-autostart-toggle"),
  desktopHideWindow: document.querySelector("#desktop-hide-window"),
  desktopQuitApp: document.querySelector("#desktop-quit-app"),
  loginSheet: document.querySelector("#login-sheet"),
  loginSheetBackdrop: document.querySelector("#login-sheet-backdrop"),
  loginSheetClose: document.querySelector("#login-sheet-close"),
  loginSheetTitle: document.querySelector("#login-sheet-title"),
  loginSheetSummary: document.querySelector("#login-sheet-summary"),
  loginSheetMeta: document.querySelector("#login-sheet-meta"),
  loginManualInput: document.querySelector("#login-manual-input"),
  loginOpenExternal: document.querySelector("#login-open-external"),
  loginOpenCurrent: document.querySelector("#login-open-current"),
  loginCopyLink: document.querySelector("#login-copy-link"),
  loginSubmitManual: document.querySelector("#login-submit-manual"),
  loginClearManual: document.querySelector("#login-clear-manual"),
};

els.form.addEventListener("submit", async (event) => {
  event.preventDefault();
  const name = els.input.value.trim();
  if (!name) {
    showToast("先输入槽位名");
    return;
  }
  await runTask("create", async () => {
    await transport.createProfile(name);
    els.input.value = "";
    showToast(`已创建 ${name}`);
    await loadDashboard();
  });
});

els.refresh.addEventListener("click", () => refreshAllUsage());
els.desktopQuickbarRefresh.addEventListener("click", () => refreshAllUsage());
els.autoSwitchToggle.addEventListener("click", () => toggleAutoSwitch());
els.autoSwitchRun.addEventListener("click", () => runAutoSwitchCheck());
els.desktopQuickbarRun.addEventListener("click", () => runAutoSwitchCheck());
els.desktopAutoStartToggle.addEventListener("click", () => toggleDesktopAutoStart());
els.desktopHideWindow.addEventListener("click", () => hideDesktopWindow());
els.desktopQuitApp.addEventListener("click", () => quitDesktopApp());
els.loginSheetBackdrop.addEventListener("click", () => closeLoginSheet());
els.loginSheetClose.addEventListener("click", () => closeLoginSheet());
els.loginOpenExternal.addEventListener("click", () => openLoginInNewTab());
els.loginOpenCurrent.addEventListener("click", () => openLoginInCurrentTab());
els.loginCopyLink.addEventListener("click", () => copyLoginLink());
els.loginSubmitManual.addEventListener("click", () => submitManualLogin());
els.loginClearManual.addEventListener("click", () => {
  els.loginManualInput.value = "";
  els.loginManualInput.focus();
});
window.addEventListener("storage", (event) => {
  if (event.key === LOGIN_EVENT_KEY && event.newValue) {
    void consumeLoginResult(event.newValue);
  }
});

bootstrap();

async function bootstrap() {
  applyDesktopChrome();
  bindDesktopRuntimeEvents();
  await Promise.all([restoreLoginResult(), loadDashboard(), loadDesktopStatus()]);
  window.setInterval(() => {
    if (document.hidden) {
      return;
    }
    silentRefreshDashboard();
  }, 15000);
}

function applyDesktopChrome() {
  document.body.classList.toggle("desktop-mode", isDesktopMode());
  if (!isDesktopMode()) {
    return;
  }
  document.title = "Token Manager Tools";
  const eyebrow = document.querySelector(".masthead .eyebrow");
  const title = document.querySelector(".masthead h1");
  const lede = document.querySelector(".masthead .lede");
  if (eyebrow) {
    eyebrow.textContent = "macOS 客户端";
  }
  if (title) {
    title.textContent = "账号池";
  }
  if (lede) {
    lede.textContent = "本机管理 Codex / OpenAI 槽位。登录、切换和自动兜底都留在这一个窗口里。";
  }
}

async function loadDashboard() {
  await Promise.all([loadProfiles(), loadAutoSwitchStatus()]);
}

async function loadDesktopStatus() {
  if (!isDesktopMode()) {
    renderDesktopPanel();
    return;
  }
  await runTask("desktop:status", async () => {
    state.desktop = await transport.getDesktopStatus();
    renderDesktopPanel();
  });
}

async function loadProfiles() {
  await runTask("load", async () => {
    const data = await transport.listProfiles();
    applyProfiles(data.profiles ?? []);
    render();
    maybeAutoRefreshUsage();
  });
}

function render() {
  renderDesktopQuickbar();
  renderSummary();
  renderAutoSwitch();
  renderDesktopPanel();
  renderList();
  renderDetail();
}

function renderDesktopQuickbar() {
  if (!isDesktopMode()) {
    els.desktopQuickbar.hidden = true;
    return;
  }
  const managed = state.profiles.filter((profile) => !profile.isDefault);
  const active = managed.find((profile) => profile.isActive);
  els.desktopQuickbar.hidden = false;
  els.desktopQuickbarActive.textContent = active
    ? `当前运行槽位：${active.name}`
    : "当前运行槽位：系统默认资料";
  els.desktopQuickbarMeta.textContent = managed.length > 0
    ? "右上角菜单栏入口如果被系统或第三方工具折叠，直接在这里看各槽位余量。"
    : "先创建并登录槽位，这里会直接显示每个槽位的额度概览。";
  if (managed.length === 0) {
    els.desktopQuickbarProfiles.innerHTML = `<p class="muted desktop-quickbar-empty">还没有托管槽位。</p>`;
    return;
  }
  els.desktopQuickbarProfiles.replaceChildren(...managed.map(renderDesktopQuickbarProfile));
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

function renderDesktopQuickbarProfile(profile) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = `desktop-quickbar-profile ${profile.name === state.selectedName ? "is-selected" : ""}`;
  button.addEventListener("click", () => {
    state.selectedName = profile.name;
    render();
  });
  button.innerHTML = `
    <span class="desktop-quickbar-profile-name">
      <strong>${escapeHTML(profile.name)}</strong>
      ${profile.isActive ? '<span class="badge ok">当前</span>' : ""}
      <span class="badge ${badgeClass(profile)}">${escapeHTML(statusLabel(profile))}</span>
    </span>
    <span class="desktop-quickbar-profile-quota">${escapeHTML(desktopQuickbarQuotaText(profile))}</span>
  `;
  return button;
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

function renderDesktopPanel() {
  if (!isDesktopMode()) {
    els.desktopPanel.hidden = true;
    return;
  }
  const status = state.desktop || {
    hideWindowOnClose: true,
    autoStartEnabled: false,
    autoStartKind: "",
    autoStartTarget: "",
    canConfigureAutoStart: false,
    autoStartMessage: "正在读取桌面状态。",
  };
  els.desktopPanel.hidden = false;
  els.desktopAutoStartBadge.textContent = status.autoStartEnabled ? "已启用" : "未启用";
  els.desktopAutoStartBadge.className = `badge ${status.autoStartEnabled ? "ok" : ""}`;
  els.desktopAutoStartToggle.textContent = status.autoStartEnabled ? "关闭开机启动" : "开启开机启动";
  els.desktopAutoStartToggle.disabled = !status.canConfigureAutoStart;
  els.desktopPanelSummary.textContent = status.hideWindowOnClose
    ? "关闭窗口会隐藏到后台；再次打开应用会回到当前客户端。"
    : "当前窗口可以隐藏到后台，账号检查和自动切换会继续运行。";
  const autoStartState = status.autoStartEnabled
    ? `已启用${status.autoStartKind ? `（${status.autoStartKind}）` : ""}`
    : `未启用${status.autoStartKind ? `（${status.autoStartKind}）` : ""}`;
  els.desktopPanelMeta.innerHTML = `
    ${detailItem("开机启动", autoStartState)}
    ${detailItem("关闭按钮", status.hideWindowOnClose ? "隐藏窗口，不会直接退出" : "直接退出")}
    ${detailItem("提示", status.autoStartMessage || (status.autoStartEnabled ? "开机启动时会先在后台拉起客户端；需要时再打开窗口。" : "可把窗口隐藏到后台；需要完全退出时点“退出客户端”。"))}
  `;
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

function desktopQuickbarQuotaText(profile) {
  if (!profile.hasCredential) return "未登录";
  if (!profile.cachedProbe) return "未检查";
  const parts = [];
  const fiveHour = profile.cachedProbe.usage?.fiveHour;
  const week = profile.cachedProbe.usage?.week;
  if (typeof fiveHour?.leftPercent === "number") {
    parts.push(`5h ${fiveHour.leftPercent}%`);
  }
  if (typeof week?.leftPercent === "number") {
    parts.push(`周 ${week.leftPercent}%`);
  }
  if (parts.length > 0) {
    return parts.join(" · ");
  }
  return profile.cachedProbe.reason || "已登录";
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
  await runTask(`login:${name}`, async () => {
    const data = await transport.startLogin(name);
    openLoginSheet(data);
    const opened = openURLInNewTab(data.authUrl);
    showToast(opened ? "已打开登录页。完成后会自动回到账号池；如果失败，可复制链接或粘贴回调地址。" : "浏览器拦截了新窗口。请点“新窗口打开”或“当前页打开”。");
  });
}

function openLoginSheet(flow) {
  state.login = {
    profileName: flow.profileName,
    authUrl: flow.authUrl,
    redirectUrl: flow.redirectUrl,
  };
  els.loginSheet.hidden = false;
  document.body.classList.add("has-sheet");
  els.loginSheetTitle.textContent = `登录 ${flow.profileName}`;
  els.loginSheetSummary.textContent = `优先用系统浏览器完成 OpenAI 登录。完成后会自动写入本机槽位 ${flow.profileName}。`;
  els.loginSheetMeta.textContent = `回调地址：${flow.redirectUrl}。如果页面没有自动写回，把最终跳转地址或 code 粘贴到上面的输入框。`;
  els.loginManualInput.placeholder = `${flow.redirectUrl}?code=...&state=...`;
  if (!els.loginManualInput.value.trim()) {
    els.loginManualInput.value = "";
  }
}

function closeLoginSheet() {
  state.login = null;
  els.loginSheet.hidden = true;
  document.body.classList.remove("has-sheet");
}

function openURLInNewTab(url) {
  if (runtime && typeof runtime.BrowserOpenURL === "function") {
    runtime.BrowserOpenURL(url);
    return true;
  }
  const popup = window.open(url, "_blank", "noopener");
  return Boolean(popup);
}

function openLoginInNewTab() {
  if (!state.login?.authUrl) {
    return;
  }
  const opened = openURLInNewTab(state.login.authUrl);
  showToast(opened ? "已再次打开登录页。" : "浏览器拦截了新窗口，请改用“当前页打开”或“复制链接”。");
}

function openLoginInCurrentTab() {
  if (!state.login?.authUrl) {
    return;
  }
  if (runtime && typeof runtime.BrowserOpenURL === "function") {
    runtime.BrowserOpenURL(state.login.authUrl);
    showToast("桌面客户端会使用系统浏览器打开登录页。");
    return;
  }
  window.location.assign(state.login.authUrl);
}

async function copyLoginLink() {
  if (!state.login?.authUrl) {
    return;
  }
  try {
    await navigator.clipboard.writeText(state.login.authUrl);
    showToast("已复制登录链接。");
  } catch {
    const input = document.createElement("textarea");
    input.value = state.login.authUrl;
    input.setAttribute("readonly", "readonly");
    input.style.position = "fixed";
    input.style.opacity = "0";
    document.body.append(input);
    input.select();
    document.execCommand("copy");
    input.remove();
    showToast("已复制登录链接。");
  }
}

async function submitManualLogin() {
  const profileName = state.login?.profileName;
  const input = els.loginManualInput.value.trim();
  if (!profileName) {
    showToast("先点“登录”再粘贴回调地址。");
    return;
  }
  if (!input) {
    showToast("先粘贴回调地址或 code。");
    return;
  }
  await runTask(`login-complete:${profileName}`, async () => {
    const result = await transport.completeManualLogin(profileName, input);
    els.loginManualInput.value = "";
    closeLoginSheet();
    showToast(result.message || `${profileName} 已写入本机账号池。`);
    await loadDashboard();
  });
}

async function probeProfile(name) {
  await runTask(`probe:${name}`, async () => {
    const result = await transport.probeProfile(name);
    showToast(`${name}: ${result.reason}`);
    await loadDashboard();
  });
}

async function refreshAllUsage() {
  await runTask("refresh-usage", async () => {
    const result = await transport.refreshUsage();
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
    state.autoSwitch = await transport.getAutoSwitchStatus();
    renderAutoSwitch();
  });
}

async function toggleAutoSwitch() {
  await runTask("auto-switch:toggle", async () => {
    const result = await transport.setAutoSwitchEnabled(!state.autoSwitch?.enabled);
    state.autoSwitch = result.status;
    renderAutoSwitch();
    await loadProfiles();
    showToast(state.autoSwitch.enabled ? "自动切换已开启" : "自动切换已关闭");
  });
}

async function runAutoSwitchCheck() {
  await runTask("auto-switch:run", async () => {
    const result = await transport.runAutoSwitchCheck();
    state.autoSwitch = result.status;
    renderAutoSwitch();
    await loadProfiles();
    showToast(result.switched ? state.autoSwitch.lastMessage || "已自动切换账号" : state.autoSwitch.lastMessage || "已完成自动切换检查");
  });
}

async function toggleDesktopAutoStart() {
  if (!isDesktopMode()) {
    return;
  }
  await runTask("desktop:autostart", async () => {
    const previousEnabled = Boolean(state.desktop?.autoStartEnabled);
    state.desktop = await transport.setDesktopAutoStart(!state.desktop?.autoStartEnabled);
    renderDesktopPanel();
    if (state.desktop.autoStartEnabled !== previousEnabled) {
      showToast(state.desktop.autoStartEnabled ? "已开启开机启动。" : "已关闭开机启动。");
      return;
    }
    showToast(state.desktop.autoStartMessage || "开机启动状态未变化。");
  });
}

function hideDesktopWindow() {
  if (!runtime || typeof runtime.Hide !== "function") {
    return;
  }
  showToast("客户端已隐藏到后台。点 Dock 图标或重新打开应用可以回来。");
  runtime.Hide();
}

function quitDesktopApp() {
  if (!runtime || typeof runtime.Quit !== "function") {
    return;
  }
  runtime.Quit();
}

async function activateProfile(name) {
  await runTask(`activate:${name}`, async () => {
    await transport.activateProfile(name);
    showToast(`已切换到 ${name}`);
    await loadDashboard();
  });
}

async function removeProfile(name) {
  if (!confirm(`移除本机槽位 ${name}？本地资料会归档，远端账号不会被删除。`)) {
    return;
  }
  await runTask(`remove:${name}`, async () => {
    const result = await transport.removeProfile(name);
    showToast(result.message || `已移除 ${name}`);
    await loadDashboard();
  });
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
      transport.listProfiles(),
      transport.getAutoSwitchStatus(),
    ]);
    applyProfiles(profiles.profiles ?? []);
    state.autoSwitch = autoSwitch;
    render();
  } catch {
    // Ignore silent polling errors; manual actions still surface errors.
  }
}

function bindDesktopRuntimeEvents() {
  if (!runtime || typeof runtime.EventsOn !== "function") {
    return;
  }
  runtime.EventsOn(LOGIN_RUNTIME_EVENT, (payload) => {
    if (!payload) {
      return;
    }
    void consumeLoginResult(JSON.stringify(payload));
  });
  runtime.EventsOn(DESKTOP_ACTION_RUNTIME_EVENT, (payload) => {
    void consumeDesktopAction(payload);
  });
  runtime.EventsOn(FOCUS_PROFILE_RUNTIME_EVENT, (payload) => {
    void focusProfileFromDesktop(payload);
  });
}

function isDesktopMode() {
  return Boolean(runtime && typeof runtime.Quit === "function");
}

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

async function restoreLoginResult() {
  try {
    const raw = localStorage.getItem(LOGIN_EVENT_KEY);
    if (raw) {
      await consumeLoginResult(raw);
    }
  } catch {
    // Ignore storage access errors in restrictive browser modes.
  }
}

async function consumeLoginResult(raw) {
  let payload;
  try {
    payload = JSON.parse(raw);
  } catch {
    localStorage.removeItem(LOGIN_EVENT_KEY);
    return;
  }
  try {
    localStorage.removeItem(LOGIN_EVENT_KEY);
  } catch {
    // Ignore storage access errors in restrictive browser modes.
  }
  if (payload.status === "success" && payload.profileName && state.login?.profileName === payload.profileName) {
    closeLoginSheet();
  }
  showToast(payload.body || payload.title || "登录状态已更新。");
  await loadDashboard();
}

async function consumeDesktopAction(payload) {
  if (!payload) {
    return;
  }
  if (payload.profileName) {
    state.selectedName = payload.profileName;
  }
  showToast(payload.body || payload.title || "桌面操作已完成。");
  await loadDashboard();
}

async function focusProfileFromDesktop(payload) {
  const profileName = payload?.profileName;
  if (!profileName) {
    return;
  }
  state.selectedName = profileName;
  try {
    const data = await transport.listProfiles();
    applyProfiles(data.profiles ?? []);
  } catch {
    // Ignore and keep the previous state if the silent sync failed.
  }
  render();
}
