package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/henry-insomniac/token-manager-tools/internal/accountpool"
)

func TestProfilesAPICreatesAndListsProfile(t *testing.T) {
	handler := NewHandler(newTestPool(t))

	createReq := httptest.NewRequest(http.MethodPost, "/api/profiles", bytes.NewBufferString(`{"name":"acct-api"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createResp := httptest.NewRecorder()
	handler.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusCreated {
		t.Fatalf("unexpected create status %d: %s", createResp.Code, createResp.Body.String())
	}

	listResp := httptest.NewRecorder()
	handler.ServeHTTP(listResp, httptest.NewRequest(http.MethodGet, "/api/profiles", nil))
	if listResp.Code != http.StatusOK {
		t.Fatalf("unexpected list status %d: %s", listResp.Code, listResp.Body.String())
	}
	var body struct {
		Profiles []accountpool.ProfileSnapshot `json:"profiles"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&body); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !hasProfile(body.Profiles, "acct-api") {
		t.Fatalf("created profile missing from list: %#v", body.Profiles)
	}
}

func TestStartLoginAPIHidesVerifier(t *testing.T) {
	pool := newTestPool(t)
	handler := NewHandler(pool)
	if _, err := pool.CreateProfile("acct-login"); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/profiles/acct-login/login/start", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", resp.Code, resp.Body.String())
	}
	raw := resp.Body.String()
	if strings.Contains(raw, "verifier") {
		t.Fatalf("login response leaked verifier: %s", raw)
	}
	var body struct {
		ProfileName string `json:"profileName"`
		AuthURL     string `json:"authUrl"`
		RedirectURL string `json:"redirectUrl"`
	}
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if body.ProfileName != "acct-login" || !strings.Contains(body.AuthURL, "code_challenge=") || !strings.Contains(body.RedirectURL, "/auth/callback") {
		t.Fatalf("unexpected login response: %#v", body)
	}
}

func TestCallbackRejectsUnknownState(t *testing.T) {
	handler := NewHandler(newTestPool(t))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/auth/callback?code=abc&state=missing", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("callback page should render actionable HTML, got %d", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "登录流程已失效") {
		t.Fatalf("unexpected callback body: %s", resp.Body.String())
	}
}

func TestManualLoginCompleteAPI(t *testing.T) {
	accessToken := accountpoolTestToken(t, "acct-manual-id", "manual@example.com")
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		writeJSONResponse(t, w, map[string]any{
			"access_token":  accessToken,
			"refresh_token": "refresh-manual",
			"id_token":      "id-manual",
			"expires_in":    3600,
		})
	}))
	defer tokenServer.Close()

	pool := newTestPoolWithConfig(t, accountpool.Config{OAuthTokenURL: tokenServer.URL})
	if _, err := pool.CreateProfile("acct-manual"); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	handler := NewHandler(pool)

	startResp := httptest.NewRecorder()
	handler.ServeHTTP(startResp, httptest.NewRequest(http.MethodPost, "/api/profiles/acct-manual/login/start", nil))
	if startResp.Code != http.StatusOK {
		t.Fatalf("unexpected start status %d: %s", startResp.Code, startResp.Body.String())
	}
	var startBody struct {
		AuthURL string `json:"authUrl"`
	}
	if err := json.NewDecoder(startResp.Body).Decode(&startBody); err != nil {
		t.Fatalf("Decode start: %v", err)
	}
	parsedURL, err := url.Parse(startBody.AuthURL)
	if err != nil {
		t.Fatalf("Parse auth url: %v", err)
	}
	state := parsedURL.Query().Get("state")
	if state == "" {
		t.Fatalf("missing state in auth url: %s", startBody.AuthURL)
	}

	completeReq := httptest.NewRequest(http.MethodPost, "/api/profiles/acct-manual/login/complete", bytes.NewBufferString(`{"input":"http://127.0.0.1:1455/auth/callback?code=manual-code&state=`+state+`"}`))
	completeReq.Header.Set("Content-Type", "application/json")
	completeResp := httptest.NewRecorder()
	handler.ServeHTTP(completeResp, completeReq)
	if completeResp.Code != http.StatusOK {
		t.Fatalf("unexpected complete status %d: %s", completeResp.Code, completeResp.Body.String())
	}

	profiles, err := pool.ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	for _, profile := range profiles {
		if profile.Name == "acct-manual" {
			if profile.AccountEmail != "manual@example.com" || !profile.HasCredential {
				t.Fatalf("manual login did not persist credentials: %#v", profile)
			}
			return
		}
	}
	t.Fatalf("manual profile missing from list")
}

func TestAutoSwitchAPIGetAndPatch(t *testing.T) {
	handler := NewHandler(newTestPool(t))

	getResp := httptest.NewRecorder()
	handler.ServeHTTP(getResp, httptest.NewRequest(http.MethodGet, "/api/auto-switch", nil))
	if getResp.Code != http.StatusOK {
		t.Fatalf("unexpected get status %d: %s", getResp.Code, getResp.Body.String())
	}
	var initial accountpool.AutoSwitchStatus
	if err := json.NewDecoder(getResp.Body).Decode(&initial); err != nil {
		t.Fatalf("Decode initial: %v", err)
	}
	if initial.Enabled {
		t.Fatalf("auto switch should be disabled by default")
	}

	patchReq := httptest.NewRequest(http.MethodPatch, "/api/auto-switch", bytes.NewBufferString(`{"enabled":true}`))
	patchReq.Header.Set("Content-Type", "application/json")
	patchResp := httptest.NewRecorder()
	handler.ServeHTTP(patchResp, patchReq)
	if patchResp.Code != http.StatusOK {
		t.Fatalf("unexpected patch status %d: %s", patchResp.Code, patchResp.Body.String())
	}
	var patched accountpool.AutoSwitchRunResult
	if err := json.NewDecoder(patchResp.Body).Decode(&patched); err != nil {
		t.Fatalf("Decode patched: %v", err)
	}
	if !patched.Status.Enabled {
		t.Fatalf("expected auto switch to become enabled: %#v", patched)
	}
}

func TestAutoSwitchRunAPI(t *testing.T) {
	handler := NewHandler(newTestPool(t))
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/auto-switch/run", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected run status %d: %s", resp.Code, resp.Body.String())
	}
}

func TestAutoSwitchEnableAPISwitchesActiveProfile(t *testing.T) {
	accessA := accountpoolTestToken(t, "acct-a-id", "a@example.com")
	accessB := accountpoolTestToken(t, "acct-b-id", "b@example.com")
	usageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch got := r.Header.Get("Authorization"); got {
		case "Bearer " + accessA:
			writeJSONResponse(t, w, map[string]any{
				"rate_limit": map[string]any{
					"primary_window": map[string]any{
						"used_percent": 100,
					},
				},
			})
		case "Bearer " + accessB:
			writeJSONResponse(t, w, map[string]any{
				"rate_limit": map[string]any{
					"primary_window": map[string]any{
						"used_percent": 10,
					},
					"secondary_window": map[string]any{
						"used_percent": 20,
					},
				},
			})
		default:
			t.Fatalf("unexpected authorization: %s", got)
		}
	}))
	defer usageServer.Close()

	pool := newTestPoolWithConfig(t, accountpool.Config{UsageURL: usageServer.URL})
	if _, err := pool.CreateProfile("acct-a"); err != nil {
		t.Fatalf("CreateProfile acct-a: %v", err)
	}
	if _, err := pool.CreateProfile("acct-b"); err != nil {
		t.Fatalf("CreateProfile acct-b: %v", err)
	}
	if err := pool.PersistTokens("acct-a", accountpool.OAuthTokens{
		Access:    accessA,
		Refresh:   "refresh-a",
		IDToken:   "id-a",
		Expires:   1893456000000,
		AccountID: "acct-a-id",
		Email:     "a@example.com",
	}); err != nil {
		t.Fatalf("PersistTokens acct-a: %v", err)
	}
	if err := pool.PersistTokens("acct-b", accountpool.OAuthTokens{
		Access:    accessB,
		Refresh:   "refresh-b",
		IDToken:   "id-b",
		Expires:   1893456000000,
		AccountID: "acct-b-id",
		Email:     "b@example.com",
	}); err != nil {
		t.Fatalf("PersistTokens acct-b: %v", err)
	}
	if err := pool.ActivateProfile("acct-a"); err != nil {
		t.Fatalf("ActivateProfile acct-a: %v", err)
	}

	handler := NewHandler(pool)
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/auto-switch", bytes.NewBufferString(`{"enabled":true}`))
	patchReq.Header.Set("Content-Type", "application/json")
	patchResp := httptest.NewRecorder()
	handler.ServeHTTP(patchResp, patchReq)
	if patchResp.Code != http.StatusOK {
		t.Fatalf("unexpected patch status %d: %s", patchResp.Code, patchResp.Body.String())
	}
	var result accountpool.AutoSwitchRunResult
	if err := json.NewDecoder(patchResp.Body).Decode(&result); err != nil {
		t.Fatalf("Decode patch: %v", err)
	}
	if !result.Switched || result.Status.LastTo == nil || *result.Status.LastTo != "acct-b" {
		t.Fatalf("expected immediate auto switch to acct-b: %#v", result)
	}

	listResp := httptest.NewRecorder()
	handler.ServeHTTP(listResp, httptest.NewRequest(http.MethodGet, "/api/profiles", nil))
	if listResp.Code != http.StatusOK {
		t.Fatalf("unexpected list status %d: %s", listResp.Code, listResp.Body.String())
	}
	var body struct {
		Profiles []accountpool.ProfileSnapshot `json:"profiles"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&body); err != nil {
		t.Fatalf("Decode list: %v", err)
	}
	for _, profile := range body.Profiles {
		if profile.Name == "acct-b" && !profile.IsActive {
			t.Fatalf("expected acct-b to be active after auto switch")
		}
	}
}

func newTestPool(t *testing.T) *accountpool.AccountPool {
	t.Helper()
	return newTestPoolWithConfig(t, accountpool.Config{})
}

func newTestPoolWithConfig(t *testing.T, config accountpool.Config) *accountpool.AccountPool {
	t.Helper()
	root := t.TempDir()
	config.HomeDir = firstNonEmptyString(config.HomeDir, root)
	config.OpenClawHome = firstNonEmptyString(config.OpenClawHome, root)
	config.CodexHome = firstNonEmptyString(config.CodexHome, root)
	config.ManagerDir = firstNonEmptyString(config.ManagerDir, filepath.Join(root, ".manager"))
	config.OAuthAuthorizeURL = firstNonEmptyString(config.OAuthAuthorizeURL, "https://example.test/authorize")
	config.OAuthRedirectURL = firstNonEmptyString(config.OAuthRedirectURL, "http://127.0.0.1:1455/auth/callback")
	if config.Clock == nil {
		config.Clock = func() int64 {
			return time.Date(2026, 4, 11, 10, 0, 0, 0, time.UTC).UnixNano()
		}
	}
	pool, err := accountpool.New(config)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return pool
}

func accountpoolTestToken(t *testing.T, accountID, email string) string {
	t.Helper()
	payload := map[string]any{
		"https://api.openai.com/auth": map[string]any{
			"chatgpt_account_id": accountID,
		},
		"https://api.openai.com/profile": map[string]any{
			"email": email,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal token: %v", err)
	}
	return "header." + base64.RawURLEncoding.EncodeToString(body) + ".sig"
}

func writeJSONResponse(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("Encode: %v", err)
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func hasProfile(profiles []accountpool.ProfileSnapshot, name string) bool {
	for _, profile := range profiles {
		if profile.Name == name {
			return true
		}
	}
	return false
}
