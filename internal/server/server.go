package server

import (
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strings"

	"github.com/henry-insomniac/token-manager-tools/internal/appservice"
	"github.com/henry-insomniac/token-manager-tools/internal/logincallback"
)

//go:embed static/*
var staticFiles embed.FS

type LocalServer struct {
	service *appservice.Service
}

func StaticAssets() fs.FS {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return staticFiles
	}
	return sub
}

func NewHandler(service *appservice.Service) http.Handler {
	server := &LocalServer{
		service: service,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleIndex)
	mux.HandleFunc("/app.js", server.handleStatic("app.js", "application/javascript; charset=utf-8"))
	mux.HandleFunc("/desktop-transport.js", server.handleStatic("desktop-transport.js", "application/javascript; charset=utf-8"))
	mux.HandleFunc("/transport.js", server.handleStatic("transport.js", "application/javascript; charset=utf-8"))
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
		profiles, err := server.service.ListProfiles()
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
		profile, err := server.service.CreateProfile(input.Name)
		if err != nil {
			writeAPIError(w, err, http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusCreated, profile)
	default:
		writeAPIError(w, unsupportedMethodError(r.Method), http.StatusMethodNotAllowed)
	}
}

func (server *LocalServer) handleAutoSwitch(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		status, err := server.service.AutoSwitchStatus()
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
		result, err := server.service.SetAutoSwitchEnabled(input.Enabled)
		if err != nil {
			writeAPIError(w, err, http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, result)
	default:
		writeAPIError(w, unsupportedMethodError(r.Method), http.StatusMethodNotAllowed)
	}
}

func (server *LocalServer) handleAutoSwitchRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, unsupportedMethodError(r.Method), http.StatusMethodNotAllowed)
		return
	}
	result, err := server.service.RunAutoSwitchNow()
	if err != nil {
		writeAPIError(w, err, http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (server *LocalServer) handleProfileAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, unsupportedMethodError(r.Method), http.StatusMethodNotAllowed)
		return
	}

	trimmed := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/profiles/"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 {
		writeAPIError(w, errMissingProfileAction, http.StatusBadRequest)
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
		if err := server.service.ActivateProfile(name); err != nil {
			writeAPIError(w, err, http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"message": "已切换默认运行槽位"})
	case "probe":
		result, err := server.service.ProbeProfile(name)
		if err != nil {
			writeAPIError(w, err, http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, result)
	case "remove":
		result, err := server.service.RemoveProfile(name)
		if err != nil {
			writeAPIError(w, err, http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, result)
	case "login/start":
		flow, err := server.service.StartLogin(name)
		if err != nil {
			writeAPIError(w, err, http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, flow)
	case "login/complete":
		var input struct {
			Input string `json:"input"`
		}
		if err := decodeJSONBody(r, &input); err != nil {
			writeAPIError(w, err, http.StatusBadRequest)
			return
		}
		result, err := server.service.CompleteManualLogin(name, input.Input)
		if err != nil {
			writeAPIError(w, err, http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, result)
	default:
		writeAPIError(w, unknownProfileActionError(action), http.StatusNotFound)
	}
}

func (server *LocalServer) handleUsageRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIError(w, unsupportedMethodError(r.Method), http.StatusMethodNotAllowed)
		return
	}
	result, err := server.service.RefreshUsage()
	if err != nil {
		writeAPIError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (server *LocalServer) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	result, err := server.service.CompleteOAuthCallback(
		r.URL.Query().Get("state"),
		r.URL.Query().Get("code"),
		r.URL.Query().Get("error"),
	)
	if err != nil {
		logincallback.WriteHTML(w, logincallback.PageData{
			Title:         "登录失败",
			Body:          err.Error(),
			ProfileName:   result.ProfileName,
			Status:        "error",
			StorageKey:    logincallback.StorageKey,
			RedirectURL:   "/",
			RedirectDelay: 1400,
		})
		return
	}
	logincallback.WriteHTML(w, logincallback.PageData{
		Title:         "登录成功",
		Body:          result.Message,
		ProfileName:   result.ProfileName,
		Status:        "success",
		StorageKey:    logincallback.StorageKey,
		RedirectURL:   "/",
		RedirectDelay: 900,
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

var errMissingProfileAction = unsupportedActionError("缺少账号槽位动作")

type unsupportedActionError string

func (err unsupportedActionError) Error() string { return string(err) }

func unsupportedMethodError(method string) error {
	return unsupportedActionError("不支持的请求方法: " + method)
}

func unknownProfileActionError(action string) error {
	return unsupportedActionError("未知账号槽位动作: " + action)
}
