package accountpool

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const discoveryMaxDepth = 3

func (pool *AccountPool) discoverStateDirs() (map[string]string, error) {
	found := map[string]string{}
	if pathExists(pool.defaultOpenDir) {
		found[DefaultProfileName] = pool.defaultOpenDir
	}
	pool.walkForStateDirs(pool.openClawHome, 0, found)
	return found, nil
}

func (pool *AccountPool) walkForStateDirs(root string, depth int, found map[string]string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		fullPath := filepath.Join(root, name)
		if depth >= discoveryMaxDepth || pool.shouldSkipDiscoveryPath(fullPath, name) {
			continue
		}
		if profileName := profileNameFromStateDirName(name); profileName != "" {
			found[profileName] = preferDiscoveredStateDir(found[profileName], fullPath)
			continue
		}
		pool.walkForStateDirs(fullPath, depth+1, found)
	}
}

func (pool *AccountPool) shouldSkipDiscoveryPath(fullPath, entryName string) bool {
	switch entryName {
	case "Applications", "Desktop", "Documents", "Downloads", "Library", "Movies", "Music", "Pictures", "Public", "node_modules", ".cache", ".docker", ".git", ".local", ".npm", ".nvm", ".Trash":
		return true
	}
	cleanedPath := filepath.Clean(fullPath)
	cleanedManager := filepath.Clean(pool.managerDir)
	if cleanedPath == cleanedManager || strings.HasPrefix(cleanedPath, cleanedManager+string(os.PathSeparator)) {
		return true
	}
	return false
}

func profileNameFromStateDirName(entryName string) string {
	if entryName == ".openclaw" {
		return DefaultProfileName
	}
	if strings.HasPrefix(entryName, ".openclaw-") {
		suffix := strings.TrimPrefix(entryName, ".openclaw-")
		if suffix != "" {
			return suffix
		}
	}
	return ""
}

func preferDiscoveredStateDir(current, candidate string) string {
	if current == "" || len(candidate) < len(current) || candidate < current {
		return candidate
	}
	return current
}

func (pool *AccountPool) ensureProfileScaffold(name string) error {
	stateDir := pool.expectedStateDir(name)
	authStorePath := pool.authStorePath(name, stateDir)
	configPath := filepath.Join(stateDir, "openclaw.json")
	if err := ensureDir(filepath.Dir(authStorePath)); err != nil {
		return err
	}
	if err := ensureDir(pool.codexHomeFor(name)); err != nil {
		return err
	}
	if !pathExists(configPath) {
		if err := writeJSONFile(configPath, minimalOpenClawConfig()); err != nil {
			return err
		}
	}
	if !pathExists(authStorePath) {
		if err := writeJSONFile(authStorePath, defaultAuthStore()); err != nil {
			return err
		}
	}
	return nil
}

func (pool *AccountPool) resolveStateDir(name string) (string, error) {
	discovered, err := pool.discoverStateDirs()
	if err != nil {
		return "", err
	}
	if stateDir, ok := discovered[name]; ok {
		return stateDir, nil
	}
	return pool.expectedStateDir(name), nil
}

func (pool *AccountPool) expectedStateDir(name string) string {
	if name == DefaultProfileName {
		return pool.defaultOpenDir
	}
	return filepath.Join(pool.openClawHome, ".openclaw-"+name)
}

func (pool *AccountPool) codexHomeFor(name string) string {
	if name == DefaultProfileName {
		return pool.defaultCodex
	}
	return filepath.Join(pool.codexHome, ".codex-"+name)
}

func (pool *AccountPool) authStorePath(name, stateDir string) string {
	return filepath.Join(stateDir, "agents", "main", "agent", "auth-profiles.json")
}

func (pool *AccountPool) loadState() (State, error) {
	return readJSONFile(pool.statePath, State{}), nil
}

func (pool *AccountPool) saveState(state State) error {
	return writeJSONFile(pool.statePath, state)
}

func (pool *AccountPool) prepareArchiveRoot(name string) (string, error) {
	root := filepath.Join(pool.managerDir, "removed-profiles")
	if err := ensureDir(root); err != nil {
		return "", err
	}
	stamp := time.Unix(0, pool.clock()).UTC().Format("20060102T150405")
	target := filepath.Join(root, stamp+"-"+name)
	if pathExists(target) {
		target = filepath.Join(root, stamp+"-"+name+"-again")
	}
	if err := ensureDir(target); err != nil {
		return "", err
	}
	return target, nil
}

func pathExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func readJSONFile[T any](path string, fallback T) T {
	buffer, err := os.ReadFile(path)
	if err != nil {
		return fallback
	}
	var value T
	if err := json.Unmarshal(buffer, &value); err != nil {
		return fallback
	}
	return value
}

func writeJSONFile(path string, value any) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("path is required")
	}
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}
	buffer, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	buffer = append(buffer, '\n')
	return os.WriteFile(path, buffer, 0o600)
}

func minimalOpenClawConfig() map[string]any {
	return map[string]any{
		"meta": map[string]any{
			"managedBy": "token-manager-tools",
		},
		"auth": map[string]any{
			"profiles": map[string]any{},
		},
		"agents": map[string]any{
			"defaults": map[string]any{},
		},
	}
}

func cloneAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
