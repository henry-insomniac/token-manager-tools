(function bootstrapTransport(global) {
  async function request(path, options = {}) {
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

  const httpTransport = {
    listProfiles() {
      return request("/api/profiles");
    },
    createProfile(name) {
      return request("/api/profiles", { method: "POST", body: { name } });
    },
    startLogin(name) {
      return request(`/api/profiles/${encodeURIComponent(name)}/login/start`, { method: "POST" });
    },
    completeManualLogin(profileName, input) {
      return request(`/api/profiles/${encodeURIComponent(profileName)}/login/complete`, {
        method: "POST",
        body: { input },
      });
    },
    probeProfile(name) {
      return request(`/api/profiles/${encodeURIComponent(name)}/probe`, { method: "POST" });
    },
    activateProfile(name) {
      return request(`/api/profiles/${encodeURIComponent(name)}/activate`, { method: "POST" });
    },
    removeProfile(name) {
      return request(`/api/profiles/${encodeURIComponent(name)}/remove`, { method: "POST" });
    },
    refreshUsage() {
      return request("/api/usage/refresh", { method: "POST" });
    },
    getAutoSwitchStatus() {
      return request("/api/auto-switch");
    },
    setAutoSwitchEnabled(enabled) {
      return request("/api/auto-switch", { method: "PATCH", body: { enabled } });
    },
    runAutoSwitchCheck() {
      return request("/api/auto-switch/run", { method: "POST" });
    },
    getDesktopStatus() {
      return Promise.reject(new Error("当前不是桌面客户端。"));
    },
    setDesktopAutoStart() {
      return Promise.reject(new Error("当前不是桌面客户端。"));
    },
  };

  global.tokenManagerTransport = global.tokenManagerTransport || httpTransport;
})(window);
