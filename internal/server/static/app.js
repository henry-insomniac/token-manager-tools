const state = {
  profiles: [],
  selectedName: null,
  busy: new Set(),
};

const els = {
  list: document.querySelector("#profile-list"),
  detail: document.querySelector("#detail-panel"),
  toast: document.querySelector("#toast"),
  form: document.querySelector("#create-form"),
  input: document.querySelector("#profile-name"),
  refresh: document.querySelector("#refresh-button"),
  total: document.querySelector("#summary-total"),
  healthy: document.querySelector("#summary-healthy"),
  attention: document.querySelector("#summary-attention"),
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
    await loadProfiles();
  });
});

els.refresh.addEventListener("click", () => loadProfiles());

loadProfiles();

async function loadProfiles() {
  await runTask("load", async () => {
    const data = await api("/api/profiles");
    state.profiles = data.profiles ?? [];
    if (!state.selectedName && state.profiles.length > 0) {
      state.selectedName = state.profiles.find((profile) => !profile.isDefault)?.name ?? state.profiles[0].name;
    }
    if (!state.profiles.some((profile) => profile.name === state.selectedName)) {
      state.selectedName = state.profiles[0]?.name ?? null;
    }
    render();
  });
}

function render() {
  renderSummary();
  renderList();
  renderDetail();
}

function renderSummary() {
  const managed = state.profiles.filter((profile) => !profile.isDefault);
  els.total.textContent = managed.length;
  els.healthy.textContent = managed.filter((profile) => profile.status === "healthy").length;
  els.attention.textContent = managed.filter((profile) => profile.status !== "healthy").length;
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
  const row = document.createElement("button");
  row.type = "button";
  row.className = `profile-row ${profile.name === state.selectedName ? "is-selected" : ""}`;
  row.addEventListener("click", () => {
    state.selectedName = profile.name;
    render();
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
    <div class="detail-grid">
      ${detailItem("账号", profile.accountEmail || profile.accountId || "未登录")}
      ${detailItem("状态目录", profile.stateDir)}
      ${detailItem("Codex 目录", profile.codexHome)}
      ${detailItem("OpenClaw 配置", profile.configPath)}
      ${detailItem("认证池", profile.authStorePath)}
      ${detailItem("Codex 认证", profile.codexAuthPath)}
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
  button.className = variant;
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
    await loadProfiles();
  });
}

async function activateProfile(name) {
  await runTask(`activate:${name}`, async () => {
    await api(`/api/profiles/${encodeURIComponent(name)}/activate`, { method: "POST" });
    showToast(`已切换到 ${name}`);
    await loadProfiles();
  });
}

async function removeProfile(name) {
  if (!confirm(`移除本机槽位 ${name}？本地资料会归档，远端账号不会被删除。`)) {
    return;
  }
  await runTask(`remove:${name}`, async () => {
    const result = await api(`/api/profiles/${encodeURIComponent(name)}/remove`, { method: "POST" });
    showToast(result.message || `已移除 ${name}`);
    await loadProfiles();
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

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}
