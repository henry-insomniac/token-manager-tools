package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/henry-insomniac/token-manager-tools/internal/accountpool"
)

//go:embed static/*
var staticFiles embed.FS

type LocalServer struct {
	pool *accountpool.AccountPool

	pendingMu sync.Mutex
	pending   map[string]pendingLoginFlow
	loopOnce  sync.Once
}

type pendingLoginFlow struct {
	flow      accountpool.LoginFlow
	createdAt time.Time
}

const loginFlowTTL = 10 * time.Minute
const loginEventStorageKey = "token-manager-last-login"

func NewHandler(pool *accountpool.AccountPool) http.Handler {
	server := &LocalServer{
		pool:    pool,
		pending: map[string]pendingLoginFlow{},
	}
	server.startAutoSwitchLoop()
	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleIndex)
	mux.HandleFunc("/app.js", server.handleStatic("app.js", "application/javascript; charset=utf-8"))
	mux.HandleFunc("/styles.css", server.handleStatic("styles.css", "text/css; charset=utf-8"))
	mux.HandleFunc("/api/profiles", server.handleProfiles)
	mux.HandleFunc("/api/auto-switch", server.handleAutoSwitch)
	mux.HandleFunc("/api/auto-switch/run", server.handleAutoSwitchRun)
	mux.HandleFunc("/api/usage/refresh", server.handleUsageRefresh)
	mux.HandleFunc("/api/profiles/", server.handleProfileAction)
	mux.HandleFunc("/auth/callback", server.handleOAuthCallback)
	return mux
}

func (server *LocalServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	server.writeStatic(w, "index.html", "text/html; charset=utf-8")
}

func (server *LocalServer) handleStatic(name, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		server.writeStatic(w, name, contentType)
	}
}

func (server *LocalServer) writeStatic(w http.ResponseWriter, name, contentType string) {
	buffer, err := staticFiles.ReadFile("static/" + name)
	if err != nil {
		http.Error(w, "static asset missing", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(buffer)
}

func (server *LocalServer) handleProfiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		profiles, err := server.pool.ListProfiles()
		if err != nil {
			writeAPIError(w, err, http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"profiles": profiles})
	case http.MethodPost:
		var input struct {
			Name string `json:"name"`
		}
		if err := decodeJSONBody(r, &input); err != nil {
			writeAPIError(w, err, http.StatusBadRequest)
			return
		}
		profile, err := server.pool.CreateProfile(input.Name)
		if err != nil {
			writeAPIError(w, err, http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusCreated, profile)
	default:
		writeAPIError(w, fmt.Errorf("不支持的请求方法: %s", r.Method), http.StatusMethodNotAllowed)
	}
}

func (server *LocalServer) handleAutoSwitch(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		status, err := server.pool.AutoSwitchStatus()
		if err != nil {
			writeAPIError(w, err, http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, status)
	case http.MethodPatch:
		var input struct {
			Enabled bool `json:"enabled"`
		}
		if err := decodeJSONBody(r, &input); err != nil {
			writeAPIError(w, err, http.StatusBadRequest)
			return
		}
		result, err := server.pool.SetAutoSwitchEnabled(input.Enabled)
		if err != nil {
			writeAPIError(w, err, http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, result)
	default:
		writeAPIError(w, fmt.Errorf("不支持的请求方法: %s", r.Method), http.StatusMethodNotAllowed)
	}
}

func (server *LocalServer) handleAutoSwitchRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, fmt.Errorf("不支持的请求方法: %s", r.Method), http.StatusMethodNotAllowed)
		return
	}
	result, err := server.pool.RunAutoSwitchNow()
	if err != nil {
		writeAPIError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (server *LocalServer) handleProfileAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, fmt.Errorf("不支持的请求方法: %s", r.Method), http.StatusMethodNotAllowed)
		return
	}

	trimmed := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/profiles/"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 {
		writeAPIError(w, fmt.Errorf("缺少账号槽位动作"), http.StatusBadRequest)
		return
	}
	name, err := url.PathUnescape(parts[0])
	if err != nil {
		writeAPIError(w, err, http.StatusBadRequest)
		return
	}
	action := strings.Join(parts[1:], "/")

	switch action {
	case "activate":
		if err := server.pool.ActivateProfile(name); err != nil {
			writeAPIError(w, err, http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"message": "已切换默认运行槽位"})
	case "probe":
		result, err := server.pool.ProbeProfile(name)
		if err != nil {
			writeAPIError(w, err, http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, result)
	case "remove":
		result, err := server.pool.RemoveProfile(name)
		if err != nil {
			writeAPIError(w, err, http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, result)
	case "login/start":
		flow, err := server.pool.StartLogin(name)
		if err != nil {
			writeAPIError(w, err, http.StatusBadRequest)
			return
		}
		server.pendingMu.Lock()
		server.cleanupExpiredLoginFlowsLocked(time.Now())
		server.pending[flow.State] = pendingLoginFlow{
			flow:      flow,
			createdAt: time.Now(),
		}
		server.pendingMu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{
			"profileName": flow.ProfileName,
			"authUrl":     flow.AuthURL,
			"redirectUrl": flow.RedirectURL,
		})
	case "login/complete":
		var input struct {
			Input string `json:"input"`
		}
		if err := decodeJSONBody(r, &input); err != nil {
			writeAPIError(w, err, http.StatusBadRequest)
			return
		}
		tokens, err := server.completeManualLogin(name, input.Input)
		if err != nil {
			writeAPIError(w, err, http.StatusBadRequest)
			return
		}
		label := tokens.Email
		if strings.TrimSpace(label) == "" {
			label = name
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"profileName":  name,
			"accountEmail": tokens.Email,
			"message":      fmt.Sprintf("%s 已写入本机账号池。", label),
		})
	default:
		writeAPIError(w, fmt.Errorf("未知账号槽位动作: %s", action), http.StatusNotFound)
	}
}

func (server *LocalServer) handleUsageRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, fmt.Errorf("不支持的请求方法: %s", r.Method), http.StatusMethodNotAllowed)
		return
	}
	profiles, err := server.pool.ListProfiles()
	if err != nil {
		writeAPIError(w, err, http.StatusInternalServerError)
		return
	}
	refreshed := make([]string, 0, len(profiles))
	failed := map[string]string{}
	for _, profile := range profiles {
		if profile.IsDefault || !profile.HasCredential {
			continue
		}
		if _, err := server.pool.ProbeProfile(profile.Name); err != nil {
			failed[profile.Name] = err.Error()
			continue
		}
		refreshed = append(refreshed, profile.Name)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"refreshed": refreshed,
		"failed":    failed,
	})
}

func (server *LocalServer) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	state := strings.TrimSpace(r.URL.Query().Get("state"))
	if state == "" {
		writeCallbackHTML(w, "登录失败", "缺少登录校验信息。请回到账号池页面重新登录。", "", "error")
		return
	}
	pending, err := server.pendingLoginFlowByState(state, time.Now())
	if err != nil {
		writeCallbackHTML(w, "登录失败", err.Error(), "", "error")
		return
	}
	flow := pending.flow
	if authErr := strings.TrimSpace(r.URL.Query().Get("error")); authErr != "" {
		server.discardPendingLoginFlow(state)
		writeCallbackHTML(w, "登录失败", authErr, flow.ProfileName, "error")
		return
	}
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		server.discardPendingLoginFlow(state)
		writeCallbackHTML(w, "登录失败", "回调缺少授权 code。请重新登录。", flow.ProfileName, "error")
		return
	}
	tokens, err := server.pool.CompleteLogin(flow.ProfileName, code, flow.Verifier)
	if err != nil {
		writeCallbackHTML(w, "登录失败", err.Error(), flow.ProfileName, "error")
		return
	}
	server.discardPendingLoginFlow(state)
	label := tokens.Email
	if strings.TrimSpace(label) == "" {
		label = flow.ProfileName
	}
	writeCallbackHTML(w, "登录成功", fmt.Sprintf("%s 已写入本机账号池。正在返回账号池。", label), flow.ProfileName, "success")
}

func (server *LocalServer) startAutoSwitchLoop() {
	server.loopOnce.Do(func() {
		go func() {
			timer := time.NewTimer(accountpool.NextAutoSwitchPollInterval())
			defer timer.Stop()
			for range timer.C {
				if _, err := server.pool.RunAutoSwitchNow(); err != nil {
					timer.Reset(accountpool.NextAutoSwitchPollInterval())
					continue
				}
				timer.Reset(accountpool.NextAutoSwitchPollInterval())
			}
		}()
	})
}

func decodeJSONBody(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeAPIError(w http.ResponseWriter, err error, status int) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

func (server *LocalServer) completeManualLogin(profileName, rawInput string) (accountpool.OAuthTokens, error) {
	parsed, err := accountpool.ParseManualLoginInput(rawInput)
	if err != nil {
		return accountpool.OAuthTokens{}, err
	}
	pending, err := server.pendingLoginFlowForProfile(profileName, parsed.State, time.Now())
	if err != nil {
		return accountpool.OAuthTokens{}, err
	}
	tokens, err := server.pool.CompleteLogin(pending.flow.ProfileName, parsed.Code, pending.flow.Verifier)
	if err != nil {
		return accountpool.OAuthTokens{}, err
	}
	server.discardPendingLoginFlow(pending.flow.State)
	return tokens, nil
}

func (server *LocalServer) pendingLoginFlowByState(state string, now time.Time) (pendingLoginFlow, error) {
	server.pendingMu.Lock()
	defer server.pendingMu.Unlock()
	server.cleanupExpiredLoginFlowsLocked(now)
	pending, ok := server.pending[state]
	if !ok {
		return pendingLoginFlow{}, fmt.Errorf("登录流程已失效。请回到账号池页面重新登录。")
	}
	return pending, nil
}

func (server *LocalServer) pendingLoginFlowForProfile(profileName, state string, now time.Time) (pendingLoginFlow, error) {
	server.pendingMu.Lock()
	defer server.pendingMu.Unlock()
	server.cleanupExpiredLoginFlowsLocked(now)
	if strings.TrimSpace(state) != "" {
		pending, ok := server.pending[state]
		if !ok {
			return pendingLoginFlow{}, fmt.Errorf("登录流程已失效。请先重新点“登录”。")
		}
		if pending.flow.ProfileName != profileName {
			return pendingLoginFlow{}, fmt.Errorf("回调槽位和当前槽位不一致。请重新开始登录。")
		}
		return pending, nil
	}

	var latest pendingLoginFlow
	found := false
	for _, pending := range server.pending {
		if pending.flow.ProfileName != profileName {
			continue
		}
		if !found || pending.createdAt.After(latest.createdAt) {
			latest = pending
			found = true
		}
	}
	if !found {
		return pendingLoginFlow{}, fmt.Errorf("没有找到未完成的登录流程。请先点“登录”。")
	}
	return latest, nil
}

func (server *LocalServer) discardPendingLoginFlow(state string) {
	server.pendingMu.Lock()
	defer server.pendingMu.Unlock()
	delete(server.pending, state)
}

func writeCallbackHTML(w http.ResponseWriter, title, body, profileName, status string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	payload, _ := json.Marshal(map[string]string{
		"status":      status,
		"title":       title,
		"body":        body,
		"profileName": profileName,
		"at":          time.Now().Format(time.RFC3339Nano),
	})
	payloadScript := strings.ReplaceAll(string(payload), "</", "<\\/")
	redirectDelay := 1400
	if status == "success" {
		redirectDelay = 900
	}
	_, _ = fmt.Fprintf(w, `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s</title>
  <style>
    body{margin:0;min-height:100vh;display:grid;place-items:center;background:#151916;color:#e7ece4;font-family:"Avenir Next","PingFang SC","Microsoft YaHei UI",sans-serif}
    main{width:min(520px,calc(100vw - 40px));border:1px solid #354136;background:#20261f;border-radius:28px;padding:34px}
    h1{margin:0 0 12px;font-size:28px}
    p{margin:0;color:#aeb8aa;line-height:1.7}
    a{display:inline-flex;margin-top:16px;color:#dceec7}
  </style>
</head>
<body>
<main>
  <h1>%s</h1>
  <p>%s</p>
  <a href="/">返回账号池</a>
</main>
<script>
  const payload = %s;
  const storageKey = %q;
  try {
    localStorage.setItem(storageKey, JSON.stringify(payload));
  } catch {}
  window.setTimeout(() => {
    try {
      window.close();
    } catch {}
    window.location.replace("/");
  }, %d);
</script>
</body>
</html>`, htmlEscape(title), htmlEscape(title), htmlEscape(body), payloadScript, loginEventStorageKey, redirectDelay)
}

func htmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(value)
}

func (server *LocalServer) cleanupExpiredLoginFlowsLocked(now time.Time) {
	for state, pending := range server.pending {
		if now.Sub(pending.createdAt) > loginFlowTTL {
			delete(server.pending, state)
		}
	}
}
