package platform

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPathsForSupportedSystems(t *testing.T) {
	t.Run("darwin", func(t *testing.T) {
		paths, err := DefaultPaths(PathOptions{GOOS: "darwin", HomeDir: "/Users/test"})
		if err != nil {
			t.Fatalf("DefaultPaths: %v", err)
		}
		want := filepath.Join("/Users/test", "Library", "Application Support", AppName)
		if paths.ManagerState != want {
			t.Fatalf("unexpected manager state: %s", paths.ManagerState)
		}
	})

	t.Run("windows", func(t *testing.T) {
		paths, err := DefaultPaths(PathOptions{
			GOOS:    "windows",
			HomeDir: `C:\Users\test`,
			Env: map[string]string{
				"APPDATA": `C:\Users\test\AppData\Roaming`,
			},
		})
		if err != nil {
			t.Fatalf("DefaultPaths: %v", err)
		}
		if !strings.Contains(paths.ManagerState, "Roaming") || !strings.Contains(paths.ManagerState, AppName) {
			t.Fatalf("unexpected windows manager state: %s", paths.ManagerState)
		}
	})

	t.Run("linux", func(t *testing.T) {
		paths, err := DefaultPaths(PathOptions{
			GOOS:    "linux",
			HomeDir: "/home/test",
			Env: map[string]string{
				"XDG_STATE_HOME": "/state/test",
			},
		})
		if err != nil {
			t.Fatalf("DefaultPaths: %v", err)
		}
		want := filepath.Join("/state/test", "token-manager-tools")
		if paths.ManagerState != want {
			t.Fatalf("unexpected linux manager state: %s", paths.ManagerState)
		}
	})
}
