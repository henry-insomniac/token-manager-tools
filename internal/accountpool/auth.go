package accountpool

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func defaultAuthStore() AuthStore {
	return AuthStore{
		Version:    1,
		Profiles:   map[string]map[string]any{},
		LastGood:   map[string]string{},
		UsageStats: map[string]map[string]any{},
	}
}

func (pool *AccountPool) loadAuthStore(name string) (AuthStore, error) {
	stateDir, err := pool.resolveStateDir(name)
	if err != nil {
		return AuthStore{}, err
	}
	store := readJSONFile(pool.authStorePath(name, stateDir), defaultAuthStore())
	normalizeAuthStore(&store)
	return store, nil
}

func (pool *AccountPool) saveAuthStore(name string, store AuthStore) error {
	normalizeAuthStore(&store)
	stateDir, err := pool.resolveStateDir(name)
	if err != nil {
		return err
	}
	return writeJSONFile(pool.authStorePath(name, stateDir), store)
}

func (pool *AccountPool) PersistTokens(profileName string, tokens OAuthTokens) error {
	name, err := normalizeProfileName(profileName, false)
	if err != nil {
		return err
	}
	if err := pool.ensureProfileScaffold(name); err != nil {
		return err
	}
	store, err := pool.loadAuthStore(name)
	if err != nil {
		return err
	}
	upsertCodexCredential(&store, "openai-codex:default", tokens)
	if err := pool.saveAuthStore(name, store); err != nil {
		return err
	}
	if strings.TrimSpace(tokens.IDToken) != "" {
		if err := pool.saveCodexAuth(name, tokens); err != nil {
			return err
		}
	}
	return nil
}

func (pool *AccountPool) syncDefaultIfActive(profileName string) error {
	state, err := pool.loadState()
	if err != nil {
		return err
	}
	if state.ActiveProfileName == nil || *state.ActiveProfileName != profileName {
		return nil
	}
	return pool.syncDefaultMirror(profileName)
}

func (pool *AccountPool) tokensForProfile(profileName string) (OAuthTokens, error) {
	store, err := pool.loadAuthStore(profileName)
	if err != nil {
		return OAuthTokens{}, err
	}
	profileID := pickCodexProfileID(store)
	if profileID == nil {
		return OAuthTokens{}, errors.New("未找到 Codex 认证")
	}
	credential := store.Profiles[*profileID]
	tokens := OAuthTokens{
		Access:    anyString(credential["access"]),
		Refresh:   anyString(credential["refresh"]),
		IDToken:   anyString(credential["id_token"]),
		AccountID: anyString(credential["accountId"]),
	}
	if expires, ok := anyInt64(credential["expires"]); ok {
		tokens.Expires = expires
	}
	if tokens.AccountID == "" {
		tokens.AccountID = extractAccountID(tokens.Access)
	}
	tokens.Email = extractEmail(tokens.Access)
	if strings.TrimSpace(tokens.Access) == "" {
		return OAuthTokens{}, errors.New("Codex 认证缺少 access token")
	}
	return tokens, nil
}

func upsertCodexCredential(store *AuthStore, profileID string, tokens OAuthTokens) {
	normalizeAuthStore(store)
	store.Profiles[profileID] = map[string]any{
		"type":      "oauth",
		"provider":  "openai-codex",
		"access":    tokens.Access,
		"refresh":   tokens.Refresh,
		"expires":   tokens.Expires,
		"accountId": tokens.AccountID,
	}
	if strings.TrimSpace(tokens.IDToken) != "" {
		store.Profiles[profileID]["id_token"] = tokens.IDToken
	}
	store.LastGood["openai-codex"] = profileID
	stats := cloneAnyMap(store.UsageStats[profileID])
	if stats == nil {
		stats = map[string]any{}
	}
	stats["errorCount"] = 0
	stats["lastUsed"] = time.Now().UnixMilli()
	store.UsageStats[profileID] = stats
}

func (pool *AccountPool) loadCachedProbe(profileName string) (*CachedProbeSnapshot, error) {
	store, err := pool.loadAuthStore(profileName)
	if err != nil {
		return nil, err
	}
	profileID := pickCodexProfileID(store)
	if profileID == nil {
		return nil, nil
	}
	stats := store.UsageStats[*profileID]
	if len(stats) == 0 {
		return nil, nil
	}
	raw := stats["quotaCache"]
	if raw == nil {
		return nil, nil
	}
	buffer, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var cached CachedProbeSnapshot
	if err := json.Unmarshal(buffer, &cached); err != nil {
		return nil, err
	}
	if cached.Status == "" {
		return nil, nil
	}
	return &cached, nil
}

func (pool *AccountPool) saveCachedProbe(profileName string, result ProbeResult) error {
	store, err := pool.loadAuthStore(profileName)
	if err != nil {
		return err
	}
	profileID := pickCodexProfileID(store)
	if profileID == nil {
		return errors.New("未找到可写入额度缓存的 Codex 认证")
	}
	stats := cloneAnyMap(store.UsageStats[*profileID])
	if stats == nil {
		stats = map[string]any{}
	}
	stats["errorCount"] = 0
	stats["lastUsed"] = time.Now().UnixMilli()
	stats["quotaCache"] = map[string]any{
		"status":      result.Status,
		"reason":      result.Reason,
		"usage":       result.Usage,
		"lastProbeAt": pool.now().UTC().Format(time.RFC3339),
	}
	store.UsageStats[*profileID] = stats
	return pool.saveAuthStore(profileName, store)
}

func (pool *AccountPool) saveCodexAuth(profileName string, tokens OAuthTokens) error {
	codexHome := pool.codexHomeFor(profileName)
	file := struct {
		AuthMode     string  `json:"auth_mode"`
		OpenAIAPIKey *string `json:"OPENAI_API_KEY"`
		Tokens       struct {
			IDToken      string `json:"id_token"`
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			AccountID    string `json:"account_id"`
		} `json:"tokens"`
		LastRefresh string `json:"last_refresh"`
	}{AuthMode: "chatgpt"}
	file.Tokens.IDToken = tokens.IDToken
	file.Tokens.AccessToken = tokens.Access
	file.Tokens.RefreshToken = tokens.Refresh
	file.Tokens.AccountID = tokens.AccountID
	file.LastRefresh = pool.now().UTC().Format(time.RFC3339)
	return writeJSONFile(filepath.Join(codexHome, "auth.json"), file)
}

func normalizeAuthStore(store *AuthStore) {
	if store.Version == 0 {
		store.Version = 1
	}
	if store.Profiles == nil {
		store.Profiles = map[string]map[string]any{}
	}
	if store.LastGood == nil {
		store.LastGood = map[string]string{}
	}
	if store.UsageStats == nil {
		store.UsageStats = map[string]map[string]any{}
	}
}

func (pool *AccountPool) syncDefaultMirror(activeName string) error {
	if activeName == DefaultProfileName {
		return nil
	}
	if err := pool.ensureProfileScaffold(DefaultProfileName); err != nil {
		return err
	}
	if err := pool.refreshDefaultAuthPool(activeName); err != nil {
		return err
	}
	sourceStateDir, err := pool.resolveStateDir(activeName)
	if err != nil {
		return err
	}
	sourceConfig := filepath.Join(sourceStateDir, "openclaw.json")
	defaultConfig := filepath.Join(pool.defaultOpenDir, "openclaw.json")
	if pathExists(sourceConfig) {
		if err := copyFile(sourceConfig, defaultConfig); err != nil {
			return err
		}
	}
	sourceCodexAuth := filepath.Join(pool.codexHomeFor(activeName), "auth.json")
	defaultCodexAuth := filepath.Join(pool.defaultCodex, "auth.json")
	if pathExists(sourceCodexAuth) {
		if err := copyFile(sourceCodexAuth, defaultCodexAuth); err != nil {
			return err
		}
	}
	return nil
}

func (pool *AccountPool) refreshDefaultAuthPool(activeName string) error {
	if err := pool.ensureProfileScaffold(DefaultProfileName); err != nil {
		return err
	}
	targetStore, err := pool.loadAuthStore(DefaultProfileName)
	if err != nil {
		return err
	}
	nextStore, err := pool.buildDefaultCodexAuthPool(activeName, targetStore)
	if err != nil {
		return err
	}
	return pool.saveAuthStore(DefaultProfileName, nextStore)
}

func (pool *AccountPool) buildDefaultCodexAuthPool(activeName string, targetStore AuthStore) (AuthStore, error) {
	nextStore := defaultAuthStore()
	nextStore.Version = targetStore.Version
	for _, profileID := range sortedKeys(targetStore.Profiles) {
		if authStoreProviderIDForProfile(targetStore, profileID) == "openai-codex" {
			continue
		}
		nextStore.Profiles[profileID] = cloneAnyMap(targetStore.Profiles[profileID])
	}
	for _, profileID := range sortedKeys(targetStore.UsageStats) {
		if authStoreProviderIDForProfile(targetStore, profileID) == "openai-codex" {
			continue
		}
		nextStore.UsageStats[profileID] = cloneAnyMap(targetStore.UsageStats[profileID])
	}
	for providerID, profileID := range targetStore.LastGood {
		if strings.TrimSpace(providerID) == "openai-codex" {
			continue
		}
		nextStore.LastGood[providerID] = profileID
	}

	discovered, err := pool.discoverStateDirs()
	if err != nil {
		return AuthStore{}, err
	}
	names := make([]string, 0, len(discovered))
	for name := range discovered {
		if name != DefaultProfileName {
			names = append(names, name)
		}
	}
	sortProfileNames(names)

	type poolEntry struct {
		name             string
		runtimeProfileID string
		credential       map[string]any
		usageStats       map[string]any
	}
	entries := make([]poolEntry, 0, len(names))
	for _, name := range names {
		sourceStore, err := pool.loadAuthStore(name)
		if err != nil {
			return AuthStore{}, err
		}
		profileID := pickCodexProfileID(sourceStore)
		if profileID == nil {
			continue
		}
		credential := sourceStore.Profiles[*profileID]
		runtimeProfileID := "openai-codex:" + name
		usageStats := cloneAnyMap(targetStore.UsageStats[runtimeProfileID])
		if usageStats == nil {
			usageStats = cloneAnyMap(sourceStore.UsageStats[*profileID])
		}
		entries = append(entries, poolEntry{
			name:             name,
			runtimeProfileID: runtimeProfileID,
			credential:       cloneAnyMap(credential),
			usageStats:       usageStats,
		})
	}

	for _, entry := range entries {
		nextStore.Profiles[entry.runtimeProfileID] = entry.credential
		if entry.usageStats != nil {
			nextStore.UsageStats[entry.runtimeProfileID] = entry.usageStats
		}
	}
	if len(entries) > 0 {
		nextStore.LastGood["openai-codex"] = entries[0].runtimeProfileID
		for _, entry := range entries {
			if entry.name == activeName {
				nextStore.LastGood["openai-codex"] = entry.runtimeProfileID
				break
			}
		}
	}
	return nextStore, nil
}

func pickCodexProfileID(store AuthStore) *string {
	normalizeAuthStore(&store)
	if preferred := strings.TrimSpace(store.LastGood["openai-codex"]); preferred != "" {
		if authStoreProviderIDForProfile(store, preferred) == "openai-codex" {
			return ptr(preferred)
		}
	}
	for _, key := range sortedKeys(store.Profiles) {
		if authStoreProviderIDForProfile(store, key) == "openai-codex" {
			return ptr(key)
		}
	}
	return nil
}

func authStoreProviderIDForProfile(store AuthStore, profileID string) string {
	if profile := store.Profiles[profileID]; profile != nil {
		if provider := strings.TrimSpace(anyString(profile["provider"])); provider != "" {
			return provider
		}
	}
	provider, _, _ := strings.Cut(strings.TrimSpace(profileID), ":")
	return strings.TrimSpace(provider)
}

func anyString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func anyInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case float64:
		return int64(typed), true
	default:
		return 0, false
	}
}

func (pool *AccountPool) now() time.Time {
	return time.Unix(0, pool.clock())
}

func copyFile(source, target string) error {
	buffer, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := ensureDir(filepath.Dir(target)); err != nil {
		return err
	}
	return os.WriteFile(target, buffer, 0o600)
}
