package accountpool

import (
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompleteLoginPersistsTokensAndCodexAuth(t *testing.T) {
	accessToken := fakeAccessToken(t, "acct-login-id", "login@example.com")
	var seenForm url.Values
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		seenForm = r.PostForm
		writeJSONResponse(t, w, map[string]any{
			"access_token":  accessToken,
			"refresh_token": "refresh-token",
			"id_token":      "id-token",
			"expires_in":    3600,
		})
	}))
	defer tokenServer.Close()

	pool := newTestPoolWithConfig(t, Config{
		OAuthTokenURL:    tokenServer.URL,
		OAuthRedirectURL: "http://localhost:1455/auth/callback",
	})
	flow, err := pool.StartLogin("acct-login")
	if err != nil {
		t.Fatalf("StartLogin: %v", err)
	}
	if !strings.Contains(flow.AuthURL, "code_challenge=") || !strings.Contains(flow.AuthURL, "state="+flow.State) {
		t.Fatalf("auth url missing PKCE/state: %s", flow.AuthURL)
	}

	tokens, err := pool.CompleteLogin("acct-login", "auth-code", flow.Verifier)
	if err != nil {
		t.Fatalf("CompleteLogin: %v", err)
	}
	if tokens.AccountID != "acct-login-id" || tokens.Email != "login@example.com" {
		t.Fatalf("unexpected tokens: %#v", tokens)
	}
	if got := seenForm.Get("grant_type"); got != "authorization_code" {
		t.Fatalf("unexpected grant type: %s", got)
	}
	if got := seenForm.Get("code_verifier"); got != flow.Verifier {
		t.Fatalf("unexpected verifier: %s", got)
	}

	store, err := pool.loadAuthStore("acct-login")
	if err != nil {
		t.Fatalf("loadAuthStore: %v", err)
	}
	if got := store.LastGood["openai-codex"]; got != "openai-codex:default" {
		t.Fatalf("unexpected lastGood: %s", got)
	}
	credential := store.Profiles["openai-codex:default"]
	if credential["accountId"] != "acct-login-id" {
		t.Fatalf("credential missing account id: %#v", credential)
	}

	codexAuthPath := filepath.Join(pool.codexHomeFor("acct-login"), "auth.json")
	assertPathExists(t, codexAuthPath)
	profiles, err := pool.ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	var found ProfileSnapshot
	for _, profile := range profiles {
		if profile.Name == "acct-login" {
			found = profile
			break
		}
	}
	if found.AccountEmail != "login@example.com" || !found.HasCredential || !found.HasCodexAuth {
		t.Fatalf("profile snapshot did not include login state: %#v", found)
	}
}

func TestSetOAuthRedirectURLUpdatesLoginFlow(t *testing.T) {
	pool := newTestPoolWithConfig(t, Config{
		OAuthRedirectURL: "http://localhost:1455/auth/callback",
	})
	pool.SetOAuthRedirectURL("http://localhost:18765/auth/callback")

	flow, err := pool.StartLogin("acct-redirect")
	if err != nil {
		t.Fatalf("StartLogin: %v", err)
	}
	if flow.RedirectURL != "http://localhost:18765/auth/callback" {
		t.Fatalf("unexpected redirect url: %s", flow.RedirectURL)
	}
	parsed, err := url.Parse(flow.AuthURL)
	if err != nil {
		t.Fatalf("Parse auth url: %v", err)
	}
	if got := parsed.Query().Get("redirect_uri"); got != "http://localhost:18765/auth/callback" {
		t.Fatalf("unexpected redirect_uri in auth url: %s", got)
	}
}

func TestProbeProfileFetchesUsage(t *testing.T) {
	reset := time.Date(2026, 4, 11, 11, 0, 0, 0, time.UTC).Unix()
	accessToken := fakeAccessToken(t, "acct-probe-id", "probe@example.com")
	var accountHeader string
	usageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+accessToken {
			t.Fatalf("unexpected auth header: %s", got)
		}
		accountHeader = r.Header.Get("ChatGPT-Account-Id")
		writeJSONResponse(t, w, map[string]any{
			"plan_type": "plus",
			"rate_limit": map[string]any{
				"primary_window": map[string]any{
					"used_percent": 25,
					"reset_at":     reset,
				},
				"secondary_window": map[string]any{
					"used_percent": 60,
					"reset_at":     reset,
				},
			},
		})
	}))
	defer usageServer.Close()

	pool := newTestPoolWithConfig(t, Config{UsageURL: usageServer.URL})
	if _, err := pool.CreateProfile("acct-probe"); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	if err := pool.PersistTokens("acct-probe", OAuthTokens{
		Access:    accessToken,
		Refresh:   "refresh",
		IDToken:   "id-token",
		Expires:   1893456000000,
		AccountID: "acct-probe-id",
		Email:     "probe@example.com",
	}); err != nil {
		t.Fatalf("PersistTokens: %v", err)
	}

	result, err := pool.ProbeProfile("acct-probe")
	if err != nil {
		t.Fatalf("ProbeProfile: %v", err)
	}
	if accountHeader != "acct-probe-id" {
		t.Fatalf("usage request missing account header: %s", accountHeader)
	}
	if result.Status != "healthy" || result.Reason != "额度可用" {
		t.Fatalf("unexpected probe status: %#v", result)
	}
	if result.Usage.FiveHour == nil || result.Usage.FiveHour.LeftPercent != 75 {
		t.Fatalf("unexpected five hour usage: %#v", result.Usage.FiveHour)
	}
	if result.Usage.Week == nil || result.Usage.Week.LeftPercent != 40 {
		t.Fatalf("unexpected week usage: %#v", result.Usage.Week)
	}

	profiles, err := pool.ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	var found ProfileSnapshot
	for _, profile := range profiles {
		if profile.Name == "acct-probe" {
			found = profile
			break
		}
	}
	if found.CachedProbe == nil || found.CachedProbe.Usage.FiveHour == nil || found.CachedProbe.Usage.FiveHour.LeftPercent != 75 {
		t.Fatalf("cached usage missing from profile snapshot: %#v", found)
	}
}

func TestProbeProfileRefreshesExpiredAccessToken(t *testing.T) {
	oldAccessToken := fakeAccessToken(t, "acct-old-id", "old@example.com")
	newAccessToken := fakeAccessToken(t, "acct-new-id", "new@example.com")
	reset := time.Date(2026, 4, 11, 11, 0, 0, 0, time.UTC).Unix()

	usageRequests := 0
	usageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		usageRequests++
		switch got := r.Header.Get("Authorization"); got {
		case "Bearer " + oldAccessToken:
			http.Error(w, "expired", http.StatusUnauthorized)
		case "Bearer " + newAccessToken:
			writeJSONResponse(t, w, map[string]any{
				"rate_limit": map[string]any{
					"primary_window": map[string]any{
						"used_percent": 10,
						"reset_at":     reset,
					},
				},
			})
		default:
			t.Fatalf("unexpected usage auth header: %s", got)
		}
	}))
	defer usageServer.Close()

	refreshRequests := 0
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshRequests++
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm: %v", err)
		}
		if got := r.PostForm.Get("grant_type"); got != "refresh_token" {
			t.Fatalf("unexpected grant type: %s", got)
		}
		if got := r.PostForm.Get("refresh_token"); got != "refresh-old" {
			t.Fatalf("unexpected refresh token: %s", got)
		}
		writeJSONResponse(t, w, map[string]any{
			"access_token": newAccessToken,
			"expires_in":   7200,
		})
	}))
	defer tokenServer.Close()

	pool := newTestPoolWithConfig(t, Config{
		OAuthTokenURL: tokenServer.URL,
		UsageURL:      usageServer.URL,
	})
	if _, err := pool.CreateProfile("acct-refresh"); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	if err := pool.PersistTokens("acct-refresh", OAuthTokens{
		Access:    oldAccessToken,
		Refresh:   "refresh-old",
		IDToken:   "id-old",
		Expires:   1770000000000,
		AccountID: "acct-old-id",
		Email:     "old@example.com",
	}); err != nil {
		t.Fatalf("PersistTokens: %v", err)
	}

	result, err := pool.ProbeProfile("acct-refresh")
	if err != nil {
		t.Fatalf("ProbeProfile: %v", err)
	}
	if usageRequests != 2 || refreshRequests != 1 {
		t.Fatalf("expected one refresh and retry, usage=%d refresh=%d", usageRequests, refreshRequests)
	}
	if result.AccountID != "acct-new-id" || result.AccountEmail != "new@example.com" {
		t.Fatalf("unexpected refreshed identity: %#v", result)
	}
	if result.Usage.FiveHour == nil || result.Usage.FiveHour.LeftPercent != 90 {
		t.Fatalf("unexpected refreshed usage: %#v", result.Usage.FiveHour)
	}

	tokens, err := pool.tokensForProfile("acct-refresh")
	if err != nil {
		t.Fatalf("tokensForProfile: %v", err)
	}
	if tokens.Access != newAccessToken || tokens.Refresh != "refresh-old" || tokens.IDToken != "id-old" {
		t.Fatalf("refreshed tokens not persisted correctly: %#v", tokens)
	}
}

func TestProbeProfileAutoDetectsLoopbackProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("http_proxy", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("https_proxy", "")

	accessToken := fakeAccessToken(t, "acct-proxy-id", "proxy@example.com")
	listener := listenOnLoopbackProxyCandidate(t)
	defer listener.Close()

	proxyHits := 0
	proxyServer := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits++
		if got := r.Header.Get("Authorization"); got != "Bearer "+accessToken {
			t.Fatalf("unexpected auth header via proxy: %s", got)
		}
		writeJSONResponse(t, w, map[string]any{
			"plan_type": "plus",
			"rate_limit": map[string]any{
				"primary_window": map[string]any{
					"used_percent": 5,
				},
			},
		})
	})}
	go proxyServer.Serve(listener)
	defer proxyServer.Close()

	pool := newTestPoolWithConfig(t, Config{
		UsageURL: "http://chatgpt.test/backend-api/wham/usage",
	})
	if _, err := pool.CreateProfile("acct-proxy"); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	if err := pool.PersistTokens("acct-proxy", OAuthTokens{
		Access:    accessToken,
		Refresh:   "refresh",
		IDToken:   "id-token",
		Expires:   1893456000000,
		AccountID: "acct-proxy-id",
		Email:     "proxy@example.com",
	}); err != nil {
		t.Fatalf("PersistTokens: %v", err)
	}

	result, err := pool.ProbeProfile("acct-proxy")
	if err != nil {
		t.Fatalf("ProbeProfile: %v", err)
	}
	if proxyHits != 1 {
		t.Fatalf("expected request to go through proxy, hits=%d", proxyHits)
	}
	if got := pool.cachedProxyURL(); got != "http://"+listener.Addr().String() {
		t.Fatalf("expected detected proxy to be cached, got %q", got)
	}
	if result.Usage.FiveHour == nil || result.Usage.FiveHour.LeftPercent != 95 {
		t.Fatalf("unexpected proxied usage: %#v", result.Usage.FiveHour)
	}
}

func TestSetAutoSwitchEnabledSwitchesToHealthyCandidate(t *testing.T) {
	accessA := fakeAccessToken(t, "acct-a-id", "a@example.com")
	accessB := fakeAccessToken(t, "acct-b-id", "b@example.com")
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
						"used_percent": 8,
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

	pool := newTestPoolWithConfig(t, Config{UsageURL: usageServer.URL})
	if _, err := pool.CreateProfile("acct-a"); err != nil {
		t.Fatalf("CreateProfile acct-a: %v", err)
	}
	if _, err := pool.CreateProfile("acct-b"); err != nil {
		t.Fatalf("CreateProfile acct-b: %v", err)
	}
	if err := pool.PersistTokens("acct-a", OAuthTokens{
		Access:    accessA,
		Refresh:   "refresh-a",
		IDToken:   "id-a",
		Expires:   1893456000000,
		AccountID: "acct-a-id",
		Email:     "a@example.com",
	}); err != nil {
		t.Fatalf("PersistTokens acct-a: %v", err)
	}
	if err := pool.PersistTokens("acct-b", OAuthTokens{
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

	result, err := pool.SetAutoSwitchEnabled(true)
	if err != nil {
		t.Fatalf("SetAutoSwitchEnabled: %v", err)
	}
	if !result.Switched {
		t.Fatalf("expected auto switch to happen: %#v", result)
	}
	if result.Status.LastTo == nil || *result.Status.LastTo != "acct-b" {
		t.Fatalf("expected acct-b to become target: %#v", result.Status)
	}
	profiles, err := pool.ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	for _, profile := range profiles {
		if profile.Name == "acct-b" && !profile.IsActive {
			t.Fatalf("expected acct-b to be active after auto switch")
		}
	}
}

func TestRunAutoSwitchNowRespectsMinSwitchInterval(t *testing.T) {
	current := time.Date(2026, 4, 11, 10, 0, 0, 0, time.UTC)
	accessA := fakeAccessToken(t, "acct-a-id", "a@example.com")
	accessB := fakeAccessToken(t, "acct-b-id", "b@example.com")
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
						"used_percent": 4,
					},
					"secondary_window": map[string]any{
						"used_percent": 12,
					},
				},
			})
		default:
			t.Fatalf("unexpected authorization: %s", got)
		}
	}))
	defer usageServer.Close()

	pool := newTestPoolWithConfig(t, Config{
		UsageURL: usageServer.URL,
		Clock: func() int64 {
			return current.UnixNano()
		},
	})
	if _, err := pool.CreateProfile("acct-a"); err != nil {
		t.Fatalf("CreateProfile acct-a: %v", err)
	}
	if _, err := pool.CreateProfile("acct-b"); err != nil {
		t.Fatalf("CreateProfile acct-b: %v", err)
	}
	if err := pool.PersistTokens("acct-a", OAuthTokens{
		Access:    accessA,
		Refresh:   "refresh-a",
		IDToken:   "id-a",
		Expires:   1893456000000,
		AccountID: "acct-a-id",
		Email:     "a@example.com",
	}); err != nil {
		t.Fatalf("PersistTokens acct-a: %v", err)
	}
	if err := pool.PersistTokens("acct-b", OAuthTokens{
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
	settings := pool.loadRuntimeSettings()
	settings.AutoSwitch.Enabled = true
	settings.AutoSwitch.LastSwitchedAt = ptr(current.Add(-2 * time.Minute).Format(time.RFC3339))
	if err := pool.saveRuntimeSettings(settings); err != nil {
		t.Fatalf("saveRuntimeSettings: %v", err)
	}

	result, err := pool.RunAutoSwitchNow()
	if err != nil {
		t.Fatalf("RunAutoSwitchNow: %v", err)
	}
	if result.Switched {
		t.Fatalf("expected no switch during cooldown interval")
	}
	if got := result.Status.LastMessage; !strings.Contains(got, "距离上次自动切换过近") || !strings.Contains(got, "acct-b") {
		t.Fatalf("unexpected message: %s", got)
	}
}

func TestNextAutoSwitchPollIntervalUsesConfiguredRange(t *testing.T) {
	minDelay, maxDelay := AutoSwitchPollIntervalRange()
	seen := map[time.Duration]bool{}
	for range 64 {
		delay := NextAutoSwitchPollInterval()
		if delay < minDelay || delay > maxDelay {
			t.Fatalf("poll interval out of range: %s not in [%s, %s]", delay, minDelay, maxDelay)
		}
		seen[delay] = true
	}
	if len(seen) < 2 {
		t.Fatalf("expected randomized poll interval, got only one value: %#v", seen)
	}
}

func listenOnLoopbackProxyCandidate(t *testing.T) net.Listener {
	t.Helper()
	for _, candidate := range commonLoopbackProxyCandidates() {
		parsed, err := url.Parse(candidate)
		if err != nil || parsed.Host == "" {
			continue
		}
		if !strings.HasPrefix(parsed.Host, "127.0.0.1:") {
			continue
		}
		listener, err := net.Listen("tcp", parsed.Host)
		if err == nil {
			return listener
		}
	}
	t.Fatal("no free loopback proxy candidate port")
	return nil
}

func newTestPoolWithConfig(t *testing.T, config Config) *AccountPool {
	t.Helper()
	root := t.TempDir()
	config.HomeDir = firstNonEmpty(config.HomeDir, root)
	config.OpenClawHome = firstNonEmpty(config.OpenClawHome, root)
	config.CodexHome = firstNonEmpty(config.CodexHome, root)
	config.ManagerDir = firstNonEmpty(config.ManagerDir, filepath.Join(root, ".manager"))
	if config.Clock == nil {
		config.Clock = func() int64 {
			return time.Date(2026, 4, 11, 10, 0, 0, 0, time.UTC).UnixNano()
		}
	}
	pool, err := New(config)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return pool
}

func fakeAccessToken(t *testing.T, accountID, email string) string {
	t.Helper()
	payload := map[string]any{
		openAIAuthClaim: map[string]any{
			"chatgpt_account_id": accountID,
		},
		openAIProfileClaim: map[string]any{
			"email": email,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
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
