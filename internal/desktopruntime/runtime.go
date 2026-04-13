package desktopruntime

import (
	"os"
	"strings"

	"github.com/henry-insomniac/token-manager-tools/internal/platform"
)

const StartHiddenArg = "--start-hidden"

type Manager struct {
	executablePath string
	pathOptions    platform.PathOptions
}

type Status struct {
	Mode                  string `json:"mode"`
	HideWindowOnClose     bool   `json:"hideWindowOnClose"`
	AutoStartEnabled      bool   `json:"autoStartEnabled"`
	AutoStartKind         string `json:"autoStartKind,omitempty"`
	AutoStartTarget       string `json:"autoStartTarget,omitempty"`
	CanConfigureAutoStart bool   `json:"canConfigureAutoStart"`
	AutoStartMessage      string `json:"autoStartMessage,omitempty"`
}

func NewManager(executablePath string) *Manager {
	return &Manager{
		executablePath: strings.TrimSpace(executablePath),
	}
}

func (manager *Manager) WithPathOptions(options platform.PathOptions) *Manager {
	manager.pathOptions = options
	return manager
}

func (manager *Manager) Status() (Status, error) {
	status := Status{
		Mode:              "desktop",
		HideWindowOnClose: true,
	}
	status.CanConfigureAutoStart = manager.canConfigureAutoStart(&status.AutoStartMessage)
	autoStart, err := platform.GetAutoStartStatus(platform.AutoStartOptions{
		PathOptions: manager.pathOptions,
	})
	if err != nil {
		return status, err
	}
	status.AutoStartEnabled = autoStart.Enabled
	status.AutoStartKind = autoStart.Kind
	status.AutoStartTarget = autoStart.Target
	return status, nil
}

func (manager *Manager) SetAutoStart(enabled bool) (Status, error) {
	if enabled {
		if err := platform.ValidatePersistentExecutable(manager.executablePath); err != nil {
			status, statusErr := manager.Status()
			if statusErr == nil {
				status.AutoStartMessage = err.Error()
				status.CanConfigureAutoStart = false
				return status, err
			}
			return Status{}, err
		}
		if _, err := platform.EnsureAutoStart(platform.AutoStartOptions{
			PathOptions:    manager.pathOptions,
			ExecutablePath: manager.executablePath,
			Args:           []string{StartHiddenArg},
		}); err != nil {
			return Status{}, err
		}
	} else {
		if err := platform.DisableAutoStart(platform.AutoStartOptions{
			PathOptions: manager.pathOptions,
		}); err != nil {
			return Status{}, err
		}
	}
	return manager.Status()
}

func (manager *Manager) canConfigureAutoStart(message *string) bool {
	if err := platform.ValidatePersistentExecutable(manager.executablePath); err != nil {
		if message != nil {
			*message = err.Error()
		}
		return false
	}
	if message != nil {
		*message = ""
	}
	return true
}

func ExecutablePath() (string, error) {
	return os.Executable()
}
