package appservice

import (
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

func TestStartLoginAndCompleteOAuthCallback(t *testing.T) {
	accessToken := testToken(t, "acct-oauth-id", "oauth@example.com")
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		writeJSONResponse(t, w, map[string]any{
			"access_token":  accessToken,
			"refresh_token": "refresh-oauth",
			"id_token":      "id-oauth",
			"expires_in":    3600,
		})
	}))
	defer tokenServer.Close()

	service := newTestServiceWithConfig(t, accountpool.Config{OAuthTokenURL: tokenServer.URL})
	if _, err := service.CreateProfile("acct-oauth"); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}

	start, err := service.StartLogin("acct-oauth")
	if err != nil {
		t.Fatalf("StartLogin: %v", err)
	}
	parsed, err := url.Parse(start.AuthURL)
	if err != nil {
		t.Fatalf("Parse auth url: %v", err)
	}
	state := parsed.Query().Get("state")
	if state == "" {
		t.Fatalf("missing state in auth url")
	}

	result, err := service.CompleteOAuthCallback(state, "oauth-code", "")
	if err != nil {
		t.Fatalf("CompleteOAuthCallback: %v", err)
	}
	if result.ProfileName != "acct-oauth" {
		t.Fatalf("unexpected profile name: %#v", result)
	}
	if result.AccountEmail != "oauth@example.com" {
		t.Fatalf("unexpected account email: %#v", result)
	}
	if !strings.Contains(result.Message, "已写入本机账号池") {
		t.Fatalf("unexpected callback message: %s", result.Message)
	}
}

func TestCompleteManualLoginRequiresMatchingFlow(t *testing.T) {
	service := newTestService(t)
	if _, err := service.CreateProfile("acct-manual"); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	if _, err := service.StartLogin("acct-manual"); err != nil {
		t.Fatalf("StartLogin: %v", err)
	}
	_, err := service.CompleteManualLogin("acct-manual", "http://localhost:1455/auth/callback?code=manual-code&state=wrong")
	if err == nil {
		t.Fatalf("expected mismatched state to fail")
	}
	if !strings.Contains(err.Error(), "登录流程已失效") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRefreshUsageSkipsProfilesWithoutCredential(t *testing.T) {
	service := newTestService(t)
	if _, err := service.CreateProfile("acct-a"); err != nil {
		t.Fatalf("CreateProfile acct-a: %v", err)
	}
	if _, err := service.CreateProfile("acct-b"); err != nil {
		t.Fatalf("CreateProfile acct-b: %v", err)
	}

	result, err := service.RefreshUsage()
	if err != nil {
		t.Fatalf("RefreshUsage: %v", err)
	}
	if len(result.Refreshed) != 0 {
		t.Fatalf("expected no refreshed profiles: %#v", result)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("expected no failed profiles: %#v", result)
	}
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	return newTestServiceWithConfig(t, accountpool.Config{})
}

func newTestServiceWithConfig(t *testing.T, config accountpool.Config) *Service {
	t.Helper()
	root := t.TempDir()
	config.HomeDir = firstNonEmptyString(config.HomeDir, root)
	config.OpenClawHome = firstNonEmptyString(config.OpenClawHome, root)
	config.CodexHome = firstNonEmptyString(config.CodexHome, root)
	config.ManagerDir = firstNonEmptyString(config.ManagerDir, filepath.Join(root, ".manager"))
	config.OAuthAuthorizeURL = firstNonEmptyString(config.OAuthAuthorizeURL, "https://example.test/authorize")
	config.OAuthRedirectURL = firstNonEmptyString(config.OAuthRedirectURL, "http://localhost:1455/auth/callback")
	if config.Clock == nil {
		config.Clock = func() int64 {
			return time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC).UnixNano()
		}
	}
	pool, err := accountpool.New(config)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return New(pool)
}

func testToken(t *testing.T, accountID, email string) string {
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
