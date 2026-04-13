package platform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAutoStartTargetForSupportedSystems(t *testing.T) {
	t.Run("darwin", func(t *testing.T) {
		target, err := autoStartTarget(PathOptions{GOOS: "darwin", HomeDir: "/Users/test"})
		if err != nil {
			t.Fatalf("autoStartTarget: %v", err)
		}
		if target.Kind != "LaunchAgent" {
			t.Fatalf("unexpected kind: %s", target.Kind)
		}
		want := filepath.Join("/Users/test", "Library", "LaunchAgents", autoStartLaunchAgentLabel+".plist")
		if target.Target != want {
			t.Fatalf("unexpected target: %s", target.Target)
		}
	})

	t.Run("windows", func(t *testing.T) {
		target, err := autoStartTarget(PathOptions{
			GOOS:    "windows",
			HomeDir: `C:\Users\test`,
			Env: map[string]string{
				"APPDATA": `C:\Users\test\AppData\Roaming`,
			},
		})
		if err != nil {
			t.Fatalf("autoStartTarget: %v", err)
		}
		if target.Kind != "启动文件夹" {
			t.Fatalf("unexpected kind: %s", target.Kind)
		}
		if !strings.Contains(target.Target, "Startup") || !strings.Contains(target.Target, autoStartWindowsFileName) {
			t.Fatalf("unexpected target: %s", target.Target)
		}
	})

	t.Run("linux", func(t *testing.T) {
		target, err := autoStartTarget(PathOptions{
			GOOS:    "linux",
			HomeDir: "/home/test",
			Env: map[string]string{
				"XDG_CONFIG_HOME": "/config/test",
			},
		})
		if err != nil {
			t.Fatalf("autoStartTarget: %v", err)
		}
		if target.Kind != "XDG Autostart" {
			t.Fatalf("unexpected kind: %s", target.Kind)
		}
		want := filepath.Join("/config/test", "autostart", autoStartLinuxFileName)
		if target.Target != want {
			t.Fatalf("unexpected target: %s", target.Target)
		}
	})
}

func TestAutoStartContent(t *testing.T) {
	t.Run("darwin", func(t *testing.T) {
		content, err := autoStartContent("darwin", "/Applications/Token Manager Tools/token-manager", []string{"start", "127.0.0.1:1455", "--no-open"})
		if err != nil {
			t.Fatalf("autoStartContent: %v", err)
		}
		if !strings.Contains(content, "<string>/Applications/Token Manager Tools/token-manager</string>") {
			t.Fatalf("missing executable path: %s", content)
		}
		if !strings.Contains(content, "<string>--no-open</string>") {
			t.Fatalf("missing no-open arg: %s", content)
		}
	})

	t.Run("windows", func(t *testing.T) {
		content, err := autoStartContent("windows", `C:\Program Files\Token Manager Tools\token-manager.exe`, []string{"start", "127.0.0.1:1455", "--no-open"})
		if err != nil {
			t.Fatalf("autoStartContent: %v", err)
		}
		if !strings.Contains(content, `start "" /min "C:\Program Files\Token Manager Tools\token-manager.exe" start 127.0.0.1:1455 --no-open`) {
			t.Fatalf("unexpected windows content: %s", content)
		}
	})

	t.Run("linux", func(t *testing.T) {
		content, err := autoStartContent("linux", "/opt/token manager/token-manager", []string{"start", "127.0.0.1:1455", "--no-open"})
		if err != nil {
			t.Fatalf("autoStartContent: %v", err)
		}
		if !strings.Contains(content, `Exec=sh -lc "exec '/opt/token manager/token-manager' 'start' '127.0.0.1:1455' '--no-open'"`) {
			t.Fatalf("unexpected linux content: %s", content)
		}
	})
}

func TestEnsureAndDisableAutoStart(t *testing.T) {
	tempDir := t.TempDir()
	status, err := EnsureAutoStart(AutoStartOptions{
		PathOptions: PathOptions{
			GOOS:    "linux",
			HomeDir: tempDir,
			Env: map[string]string{
				"XDG_CONFIG_HOME": filepath.Join(tempDir, ".config"),
			},
		},
		ExecutablePath: filepath.Join(tempDir, "token-manager"),
		Args:           []string{"start", "127.0.0.1:1455", "--no-open"},
	})
	if err != nil {
		t.Fatalf("EnsureAutoStart: %v", err)
	}
	if !status.Enabled {
		t.Fatalf("expected autostart to be enabled")
	}
	if _, err := os.Stat(status.Target); err != nil {
		t.Fatalf("expected autostart file: %v", err)
	}

	current, err := GetAutoStartStatus(AutoStartOptions{
		PathOptions: PathOptions{
			GOOS:    "linux",
			HomeDir: tempDir,
			Env: map[string]string{
				"XDG_CONFIG_HOME": filepath.Join(tempDir, ".config"),
			},
		},
	})
	if err != nil {
		t.Fatalf("GetAutoStartStatus: %v", err)
	}
	if !current.Enabled {
		t.Fatalf("expected autostart status to be enabled")
	}

	if err := DisableAutoStart(AutoStartOptions{
		PathOptions: PathOptions{
			GOOS:    "linux",
			HomeDir: tempDir,
			Env: map[string]string{
				"XDG_CONFIG_HOME": filepath.Join(tempDir, ".config"),
			},
		},
	}); err != nil {
		t.Fatalf("DisableAutoStart: %v", err)
	}
	current, err = GetAutoStartStatus(AutoStartOptions{
		PathOptions: PathOptions{
			GOOS:    "linux",
			HomeDir: tempDir,
			Env: map[string]string{
				"XDG_CONFIG_HOME": filepath.Join(tempDir, ".config"),
			},
		},
	})
	if err != nil {
		t.Fatalf("GetAutoStartStatus after disable: %v", err)
	}
	if current.Enabled {
		t.Fatalf("expected autostart status to be disabled")
	}
}
