package desktopruntime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/henry-insomniac/token-manager-tools/internal/platform"
)

func TestStatusReportsAutoStartAvailability(t *testing.T) {
	root := t.TempDir()
	manager := NewManager("/Applications/Token Manager Tools/token-manager-desktop").WithPathOptions(platform.PathOptions{
		GOOS:    "linux",
		HomeDir: root,
		Env: map[string]string{
			"XDG_CONFIG_HOME": filepath.Join(root, ".config"),
		},
	})

	status, err := manager.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Mode != "desktop" || !status.HideWindowOnClose {
		t.Fatalf("unexpected status: %#v", status)
	}
	if !status.CanConfigureAutoStart {
		t.Fatalf("expected autostart to be configurable: %#v", status)
	}
	if status.AutoStartEnabled {
		t.Fatalf("expected autostart to be disabled by default: %#v", status)
	}
}

func TestSetAutoStartWritesStartHiddenEntry(t *testing.T) {
	root := t.TempDir()
	executable := "/opt/token-manager-tools/token-manager-desktop"
	manager := NewManager(executable).WithPathOptions(platform.PathOptions{
		GOOS:    "linux",
		HomeDir: root,
		Env: map[string]string{
			"XDG_CONFIG_HOME": filepath.Join(root, ".config"),
		},
	})

	status, err := manager.SetAutoStart(true)
	if err != nil {
		t.Fatalf("SetAutoStart enable: %v", err)
	}
	if !status.AutoStartEnabled {
		t.Fatalf("expected enabled status: %#v", status)
	}

	buffer, err := os.ReadFile(status.AutoStartTarget)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(buffer)
	if !strings.Contains(content, StartHiddenArg) {
		t.Fatalf("expected start-hidden arg in autostart content: %s", content)
	}

	status, err = manager.SetAutoStart(false)
	if err != nil {
		t.Fatalf("SetAutoStart disable: %v", err)
	}
	if status.AutoStartEnabled {
		t.Fatalf("expected disabled status: %#v", status)
	}
}

func TestSetAutoStartRejectsTempExecutable(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(t.TempDir(), "go-build", "token-manager-desktop")
	manager := NewManager(executable).WithPathOptions(platform.PathOptions{
		GOOS:    "linux",
		HomeDir: root,
		Env: map[string]string{
			"XDG_CONFIG_HOME": filepath.Join(root, ".config"),
		},
	})

	status, err := manager.SetAutoStart(true)
	if err == nil {
		t.Fatalf("expected temp executable to be rejected")
	}
	if status.CanConfigureAutoStart {
		t.Fatalf("expected canConfigureAutoStart=false: %#v", status)
	}
	if !strings.Contains(status.AutoStartMessage, "临时可执行文件") {
		t.Fatalf("unexpected status message: %#v", status)
	}
}
