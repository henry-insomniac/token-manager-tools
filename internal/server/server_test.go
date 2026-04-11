package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	}
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if body.ProfileName != "acct-login" || !strings.Contains(body.AuthURL, "code_challenge=") {
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

func newTestPool(t *testing.T) *accountpool.AccountPool {
	t.Helper()
	root := t.TempDir()
	pool, err := accountpool.New(accountpool.Config{
		HomeDir:           root,
		OpenClawHome:      root,
		CodexHome:         root,
		ManagerDir:        filepath.Join(root, ".manager"),
		OAuthAuthorizeURL: "https://example.test/authorize",
		OAuthRedirectURL:  "http://127.0.0.1:1455/auth/callback",
		Clock: func() int64 {
			return time.Date(2026, 4, 11, 10, 0, 0, 0, time.UTC).UnixNano()
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return pool
}

func hasProfile(profiles []accountpool.ProfileSnapshot, name string) bool {
	for _, profile := range profiles {
		if profile.Name == name {
			return true
		}
	}
	return false
}
