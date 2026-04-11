package platform

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const AppName = "Token Manager Tools"

type PathOptions struct {
	GOOS    string
	HomeDir string
	Env     map[string]string
}

type Paths struct {
	HomeDir        string
	OpenClawHome   string
	CodexHome      string
	ManagerState   string
	DefaultOpenDir string
	DefaultCodex   string
}

func DefaultPaths(options PathOptions) (Paths, error) {
	homeDir := strings.TrimSpace(options.HomeDir)
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return Paths{}, err
		}
	}
	if homeDir == "" {
		return Paths{}, errors.New("home directory is required")
	}

	goos := strings.TrimSpace(options.GOOS)
	if goos == "" {
		goos = runtimeGOOS()
	}
	env := options.Env
	if env == nil {
		env = readProcessEnv()
	}

	managerState := defaultManagerStateDir(goos, homeDir, env)
	return Paths{
		HomeDir:        homeDir,
		OpenClawHome:   homeDir,
		CodexHome:      homeDir,
		ManagerState:   managerState,
		DefaultOpenDir: filepath.Join(homeDir, ".openclaw"),
		DefaultCodex:   filepath.Join(homeDir, ".codex"),
	}, nil
}

func defaultManagerStateDir(goos, homeDir string, env map[string]string) string {
	switch goos {
	case "darwin":
		return filepath.Join(homeDir, "Library", "Application Support", AppName)
	case "windows":
		if appData := strings.TrimSpace(env["APPDATA"]); appData != "" {
			return filepath.Join(appData, AppName)
		}
		return filepath.Join(homeDir, "AppData", "Roaming", AppName)
	default:
		if stateHome := strings.TrimSpace(env["XDG_STATE_HOME"]); stateHome != "" {
			return filepath.Join(stateHome, "token-manager-tools")
		}
		return filepath.Join(homeDir, ".local", "state", "token-manager-tools")
	}
}

func readProcessEnv() map[string]string {
	values := map[string]string{}
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			values[key] = value
		}
	}
	return values
}
