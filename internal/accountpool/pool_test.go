package accountpool

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateProfileScaffoldsStandaloneDirs(t *testing.T) {
	pool := newTestPool(t)

	profile, err := pool.CreateProfile("acct-a")
	if err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}

	if profile.Name != "acct-a" {
		t.Fatalf("unexpected profile name: %s", profile.Name)
	}
	assertPathExists(t, profile.ConfigPath)
	assertPathExists(t, profile.AuthStorePath)
	assertPathExists(t, profile.CodexHome)

	profiles, err := pool.ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	if !hasProfile(profiles, "acct-a") {
		t.Fatalf("expected acct-a in profile list: %#v", profiles)
	}
}

func TestNewUsesEnvironmentRootOverrides(t *testing.T) {
	root := t.TempDir()
	openRoot := filepath.Join(root, "open")
	codexRoot := filepath.Join(root, "codex")
	managerRoot := filepath.Join(root, "manager")
	t.Setenv("OPENCLAW_HOME_DIR", openRoot)
	t.Setenv("OPENCLAW_CODEX_HOME_DIR", codexRoot)
	t.Setenv("OPENCLAW_MANAGER_DIR", managerRoot)

	pool, err := New(Config{HomeDir: root})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	profile, err := pool.CreateProfile("acct-env")
	if err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	if !strings.HasPrefix(profile.StateDir, openRoot) {
		t.Fatalf("expected state dir under OPENCLAW_HOME_DIR, got %s", profile.StateDir)
	}
	if !strings.HasPrefix(profile.CodexHome, codexRoot) {
		t.Fatalf("expected codex home under OPENCLAW_CODEX_HOME_DIR, got %s", profile.CodexHome)
	}
	if !strings.HasPrefix(pool.managerDir, managerRoot) {
		t.Fatalf("expected manager dir under OPENCLAW_MANAGER_DIR, got %s", pool.managerDir)
	}
}

func TestRemoveProfileArchivesAndDoesNotRediscover(t *testing.T) {
	pool := newTestPool(t)
	profile, err := pool.CreateProfile("acct-g")
	if err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	if err := writeJSONFile(filepath.Join(profile.CodexHome, "auth.json"), map[string]any{"token": "local-only"}); err != nil {
		t.Fatalf("write codex auth: %v", err)
	}

	result, err := pool.RemoveProfile("acct-g")
	if err != nil {
		t.Fatalf("RemoveProfile: %v", err)
	}
	if !strings.Contains(result.Message, "远端账号不会被删除") {
		t.Fatalf("unexpected message: %s", result.Message)
	}
	if result.ArchivedStateDir == nil || result.ArchivedCodexHome == nil {
		t.Fatalf("expected archived dirs, got %#v", result)
	}
	assertPathMissing(t, profile.StateDir)
	assertPathMissing(t, profile.CodexHome)
	assertPathExists(t, *result.ArchivedStateDir)
	assertPathExists(t, *result.ArchivedCodexHome)

	profiles, err := pool.ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	if hasProfile(profiles, "acct-g") {
		t.Fatalf("archived profile was rediscovered: %#v", profiles)
	}
}

func TestRemoveRejectsActiveProfile(t *testing.T) {
	pool := newTestPool(t)
	if _, err := pool.CreateProfile("acct-a"); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	if err := pool.ActivateProfile("acct-a"); err != nil {
		t.Fatalf("ActivateProfile: %v", err)
	}

	_, err := pool.RemoveProfile("acct-a")
	if err == nil {
		t.Fatalf("expected active profile removal to fail")
	}
	if !strings.Contains(err.Error(), "当前激活槽位不能移除") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestActivateSyncsDefaultAuthPoolAndRemoveCleansIt(t *testing.T) {
	pool := newTestPool(t)
	if _, err := pool.CreateProfile("acct-a"); err != nil {
		t.Fatalf("CreateProfile acct-a: %v", err)
	}
	if _, err := pool.CreateProfile("acct-b"); err != nil {
		t.Fatalf("CreateProfile acct-b: %v", err)
	}
	seedCodexCredential(t, pool, "acct-a", "acct-a-id")
	seedCodexCredential(t, pool, "acct-b", "acct-b-id")

	if err := pool.ActivateProfile("acct-a"); err != nil {
		t.Fatalf("ActivateProfile: %v", err)
	}
	defaultStore, err := pool.loadAuthStore(DefaultProfileName)
	if err != nil {
		t.Fatalf("load default auth store: %v", err)
	}
	if _, ok := defaultStore.Profiles["openai-codex:acct-a"]; !ok {
		t.Fatalf("expected acct-a in default auth pool")
	}
	if _, ok := defaultStore.Profiles["openai-codex:acct-b"]; !ok {
		t.Fatalf("expected acct-b in default auth pool")
	}
	if got := defaultStore.LastGood["openai-codex"]; got != "openai-codex:acct-a" {
		t.Fatalf("expected active profile first, got %q", got)
	}

	if _, err := pool.RemoveProfile("acct-b"); err != nil {
		t.Fatalf("RemoveProfile acct-b: %v", err)
	}
	defaultStore, err = pool.loadAuthStore(DefaultProfileName)
	if err != nil {
		t.Fatalf("load default auth store after remove: %v", err)
	}
	if _, ok := defaultStore.Profiles["openai-codex:acct-b"]; ok {
		t.Fatalf("expected acct-b to be removed from default auth pool")
	}
	if got := defaultStore.LastGood["openai-codex"]; got != "openai-codex:acct-a" {
		t.Fatalf("expected acct-a to remain active, got %q", got)
	}
}

func newTestPool(t *testing.T) *AccountPool {
	t.Helper()
	root := t.TempDir()
	pool, err := New(Config{
		HomeDir:      root,
		OpenClawHome: root,
		CodexHome:    root,
		ManagerDir:   filepath.Join(root, ".manager"),
		Clock: func() int64 {
			return time.Date(2026, 4, 11, 10, 0, 0, 0, time.UTC).UnixNano()
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return pool
}

func seedCodexCredential(t *testing.T, pool *AccountPool, profileName, accountID string) {
	t.Helper()
	stateDir, err := pool.resolveStateDir(profileName)
	if err != nil {
		t.Fatalf("resolveStateDir: %v", err)
	}
	store := defaultAuthStore()
	store.Profiles["openai-codex:default"] = map[string]any{
		"type":      "oauth",
		"provider":  "openai-codex",
		"access":    "access-" + profileName,
		"refresh":   "refresh-" + profileName,
		"expires":   int64(1893456000000),
		"accountId": accountID,
	}
	store.LastGood["openai-codex"] = "openai-codex:default"
	store.UsageStats["openai-codex:default"] = map[string]any{"lastUsed": int64(1770000000000)}
	if err := writeJSONFile(pool.authStorePath(profileName, stateDir), store); err != nil {
		t.Fatalf("write auth store: %v", err)
	}
}

func hasProfile(profiles []ProfileSnapshot, name string) bool {
	for _, profile := range profiles {
		if profile.Name == name {
			return true
		}
	}
	return false
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if !pathExists(path) {
		t.Fatalf("expected path to exist: %s", path)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if pathExists(path) {
		t.Fatalf("expected path to be missing: %s", path)
	}
}
