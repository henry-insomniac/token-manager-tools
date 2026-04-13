package accountpool

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/henry-insomniac/token-manager-tools/internal/platform"
)

const (
	defaultOAuthClientID     = "app_EMoamEEZ73f0CkXaXp7hrann"
	defaultOAuthAuthorizeURL = "https://auth.openai.com/oauth/authorize"
	defaultOAuthTokenURL     = "https://auth.openai.com/oauth/token"
	defaultOAuthScope        = "openid profile email offline_access api.connectors.read api.connectors.invoke"
	defaultOAuthRedirectURL  = "http://localhost:1455/auth/callback"
	defaultUsageURL          = "https://chatgpt.com/backend-api/wham/usage"
	openAIAuthClaim          = "https://api.openai.com/auth"
	openAIProfileClaim       = "https://api.openai.com/profile"
)

var profileNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

func New(config Config) (*AccountPool, error) {
	paths, err := platform.DefaultPaths(platform.PathOptions{HomeDir: config.HomeDir})
	if err != nil {
		return nil, err
	}
	homeDir := firstNonEmpty(config.HomeDir, paths.HomeDir)
	openClawHome := firstNonEmpty(config.OpenClawHome, os.Getenv("OPENCLAW_HOME_DIR"), paths.OpenClawHome)
	codexHome := firstNonEmpty(config.CodexHome, os.Getenv("OPENCLAW_CODEX_HOME_DIR"), paths.CodexHome)
	managerDir := firstNonEmpty(config.ManagerDir, os.Getenv("OPENCLAW_MANAGER_DIR"), paths.ManagerState)
	defaultOpenDir := firstNonEmpty(config.DefaultOpenDir, filepath.Join(openClawHome, ".openclaw"))
	defaultCodex := firstNonEmpty(config.DefaultCodex, filepath.Join(codexHome, ".codex"))
	clock := config.Clock
	if clock == nil {
		clock = func() int64 { return time.Now().UnixNano() }
	}

	pool := &AccountPool{
		homeDir:        homeDir,
		openClawHome:   openClawHome,
		codexHome:      codexHome,
		managerDir:     managerDir,
		statePath:      filepath.Join(managerDir, "state.json"),
		settingsPath:   filepath.Join(managerDir, "settings.json"),
		defaultOpenDir: defaultOpenDir,
		defaultCodex:   defaultCodex,
		authorizeURL:   firstNonEmpty(config.OAuthAuthorizeURL, defaultOAuthAuthorizeURL),
		tokenURL:       firstNonEmpty(config.OAuthTokenURL, os.Getenv("TOKEN_MANAGER_OAUTH_TOKEN_URL"), defaultOAuthTokenURL),
		redirectURL:    firstNonEmpty(config.OAuthRedirectURL, os.Getenv("TOKEN_MANAGER_OAUTH_REDIRECT_URL"), defaultOAuthRedirectURL),
		usageURL:       firstNonEmpty(config.UsageURL, os.Getenv("TOKEN_MANAGER_USAGE_URL"), defaultUsageURL),
		clock:          clock,
	}
	if config.HTTPClient != nil {
		pool.httpClient = config.HTTPClient
		pool.httpClientFixed = true
	} else {
		pool.httpClient = pool.newHTTPClient("")
	}
	return pool, ensureDir(managerDir)
}

func (pool *AccountPool) ListProfiles() ([]ProfileSnapshot, error) {
	state, err := pool.loadState()
	if err != nil {
		return nil, err
	}
	discovered, err := pool.discoverStateDirs()
	if err != nil {
		return nil, err
	}
	if len(discovered) == 0 {
		discovered[DefaultProfileName] = pool.defaultOpenDir
	}

	names := make([]string, 0, len(discovered))
	for name := range discovered {
		names = append(names, name)
	}
	sortProfileNames(names)

	profiles := make([]ProfileSnapshot, 0, len(names))
	for _, name := range names {
		snapshot, err := pool.profileSnapshot(name, discovered[name], state)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, snapshot)
	}
	return profiles, nil
}

func (pool *AccountPool) CreateProfile(rawName string) (ProfileSnapshot, error) {
	name, err := normalizeProfileName(rawName, false)
	if err != nil {
		return ProfileSnapshot{}, err
	}
	if err := pool.ensureProfileScaffold(name); err != nil {
		return ProfileSnapshot{}, err
	}
	state, err := pool.loadState()
	if err != nil {
		return ProfileSnapshot{}, err
	}
	stateDir, err := pool.resolveStateDir(name)
	if err != nil {
		return ProfileSnapshot{}, err
	}
	return pool.profileSnapshot(name, stateDir, state)
}

func (pool *AccountPool) ActivateProfile(rawName string) error {
	name, err := normalizeProfileName(rawName, true)
	if err != nil {
		return err
	}
	if err := pool.syncDefaultMirror(name); err != nil {
		return err
	}
	state, err := pool.loadState()
	if err != nil {
		return err
	}
	state.ActiveProfileName = ptr(name)
	return pool.saveState(state)
}

func (pool *AccountPool) RemoveProfile(rawName string) (RemoveResult, error) {
	name, err := normalizeProfileName(rawName, false)
	if err != nil {
		return RemoveResult{}, err
	}
	state, err := pool.loadState()
	if err != nil {
		return RemoveResult{}, err
	}
	if state.ActiveProfileName != nil && *state.ActiveProfileName == name {
		return RemoveResult{}, errors.New("当前激活槽位不能移除，请先切到别的账号")
	}

	discovered, err := pool.discoverStateDirs()
	if err != nil {
		return RemoveResult{}, err
	}
	stateDir, found := discovered[name]
	codexHome := pool.codexHomeFor(name)
	if !found && !pathExists(codexHome) {
		return RemoveResult{}, errors.New("未找到这个账号槽位")
	}
	if !pathExists(stateDir) && !pathExists(codexHome) {
		return RemoveResult{}, errors.New("未找到这个账号槽位的本地目录")
	}

	archiveRoot, err := pool.prepareArchiveRoot(name)
	if err != nil {
		return RemoveResult{}, err
	}

	var archivedState *string
	if pathExists(stateDir) {
		target := filepath.Join(archiveRoot, filepath.Base(stateDir))
		if err := os.Rename(stateDir, target); err != nil {
			return RemoveResult{}, err
		}
		archivedState = ptr(target)
	}

	var archivedCodex *string
	if pathExists(codexHome) {
		target := filepath.Join(archiveRoot, filepath.Base(codexHome))
		if err := os.Rename(codexHome, target); err != nil {
			return RemoveResult{}, err
		}
		archivedCodex = ptr(target)
	}

	if state.ActiveProfileName != nil {
		if err := pool.syncDefaultMirror(*state.ActiveProfileName); err != nil {
			return RemoveResult{}, err
		}
	} else if err := pool.refreshDefaultAuthPool(""); err != nil {
		return RemoveResult{}, err
	}

	return RemoveResult{
		ProfileName:       name,
		Message:           fmt.Sprintf("已从本机移除 %s，本地资料已移到归档。远端账号不会被删除。", name),
		ArchivedStateDir:  archivedState,
		ArchivedCodexHome: archivedCodex,
	}, nil
}

func (pool *AccountPool) profileSnapshot(name, stateDir string, state State) (ProfileSnapshot, error) {
	authStorePath := pool.authStorePath(name, stateDir)
	codexHome := pool.codexHomeFor(name)
	codexAuthPath := filepath.Join(codexHome, "auth.json")
	hasCredential := false
	if pathExists(authStorePath) {
		store := readJSONFile(authStorePath, defaultAuthStore())
		if profileID := pickCodexProfileID(store); profileID != nil {
			hasCredential = true
		}
	}
	tokens, _ := pool.tokensForProfile(name)
	cachedProbe, _ := pool.loadCachedProbe(name)

	status := "reauth_required"
	reason := "未登录"
	if name == DefaultProfileName {
		status = "system"
		reason = "默认运行镜像"
	} else if hasCredential || pathExists(codexAuthPath) {
		status = "healthy"
		reason = "已登录"
	} else if pathExists(filepath.Join(stateDir, "openclaw.json")) || pathExists(authStorePath) {
		status = "reauth_required"
		reason = "未找到认证信息"
	}
	if cachedProbe != nil && name != DefaultProfileName && (hasCredential || pathExists(codexAuthPath)) {
		status = cachedProbe.Status
		reason = cachedProbe.Reason
	}

	active := state.ActiveProfileName != nil && *state.ActiveProfileName == name
	return ProfileSnapshot{
		Name:          name,
		IsDefault:     name == DefaultProfileName,
		IsActive:      active,
		StateDir:      stateDir,
		CodexHome:     codexHome,
		ConfigPath:    filepath.Join(stateDir, "openclaw.json"),
		AuthStorePath: authStorePath,
		CodexAuthPath: codexAuthPath,
		HasConfig:     pathExists(filepath.Join(stateDir, "openclaw.json")),
		HasAuthStore:  pathExists(authStorePath),
		HasCodexAuth:  pathExists(codexAuthPath),
		HasCredential: hasCredential,
		AccountID:     tokens.AccountID,
		AccountEmail:  tokens.Email,
		Status:        status,
		StatusReason:  reason,
		CachedProbe:   cachedProbe,
	}, nil
}

func normalizeProfileName(raw string, allowDefault bool) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", errors.New("账号槽位名称不能为空")
	}
	if name == DefaultProfileName && allowDefault {
		return name, nil
	}
	if name == DefaultProfileName {
		return "", errors.New("默认镜像不能作为普通账号槽位操作")
	}
	if !profileNamePattern.MatchString(name) || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return "", errors.New("账号槽位名称只能包含字母、数字、点、短横线和下划线")
	}
	return name, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func sortProfileNames(names []string) {
	sort.Slice(names, func(i, j int) bool {
		if names[i] == DefaultProfileName {
			return true
		}
		if names[j] == DefaultProfileName {
			return false
		}
		return names[i] < names[j]
	})
}

func ptr[T any](value T) *T {
	return &value
}
