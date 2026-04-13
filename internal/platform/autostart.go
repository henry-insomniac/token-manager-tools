package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	autoStartLaunchAgentLabel = "com.henryinsomniac.token-manager-tools"
	autoStartWindowsFileName  = "Token Manager Tools.cmd"
	autoStartLinuxFileName    = "token-manager-tools.desktop"
)

type AutoStartOptions struct {
	PathOptions
	ExecutablePath string
	Args           []string
}

type AutoStartStatus struct {
	Enabled bool
	Kind    string
	Target  string
}

func EnsureAutoStart(options AutoStartOptions) (AutoStartStatus, error) {
	target, err := autoStartTarget(options.PathOptions)
	if err != nil {
		return AutoStartStatus{}, err
	}
	content, err := autoStartContent(target.GOOS, strings.TrimSpace(options.ExecutablePath), options.Args)
	if err != nil {
		return AutoStartStatus{}, err
	}
	if err := os.MkdirAll(filepath.Dir(target.Target), 0o755); err != nil {
		return AutoStartStatus{}, err
	}
	if err := os.WriteFile(target.Target, []byte(content), 0o644); err != nil {
		return AutoStartStatus{}, err
	}
	return AutoStartStatus{
		Enabled: true,
		Kind:    target.Kind,
		Target:  target.Target,
	}, nil
}

func DisableAutoStart(options AutoStartOptions) error {
	target, err := autoStartTarget(options.PathOptions)
	if err != nil {
		return err
	}
	if err := os.Remove(target.Target); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func GetAutoStartStatus(options AutoStartOptions) (AutoStartStatus, error) {
	target, err := autoStartTarget(options.PathOptions)
	if err != nil {
		return AutoStartStatus{}, err
	}
	_, err = os.Stat(target.Target)
	if err == nil {
		return AutoStartStatus{
			Enabled: true,
			Kind:    target.Kind,
			Target:  target.Target,
		}, nil
	}
	if !os.IsNotExist(err) {
		return AutoStartStatus{}, err
	}
	return AutoStartStatus{
		Enabled: false,
		Kind:    target.Kind,
		Target:  target.Target,
	}, nil
}

type autoStartTargetInfo struct {
	GOOS   string
	Kind   string
	Target string
}

func autoStartTarget(options PathOptions) (autoStartTargetInfo, error) {
	goos := strings.TrimSpace(options.GOOS)
	if goos == "" {
		goos = runtimeGOOS()
	}
	homeDir := strings.TrimSpace(options.HomeDir)
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return autoStartTargetInfo{}, err
		}
	}
	env := options.Env
	if env == nil {
		env = readProcessEnv()
	}

	switch goos {
	case "darwin":
		return autoStartTargetInfo{
			GOOS:   goos,
			Kind:   "LaunchAgent",
			Target: filepath.Join(homeDir, "Library", "LaunchAgents", autoStartLaunchAgentLabel+".plist"),
		}, nil
	case "windows":
		appData := strings.TrimSpace(env["APPDATA"])
		if appData == "" {
			appData = filepath.Join(homeDir, "AppData", "Roaming")
		}
		return autoStartTargetInfo{
			GOOS: goos,
			Kind: "启动文件夹",
			Target: filepath.Join(
				appData,
				"Microsoft",
				"Windows",
				"Start Menu",
				"Programs",
				"Startup",
				autoStartWindowsFileName,
			),
		}, nil
	default:
		configHome := strings.TrimSpace(env["XDG_CONFIG_HOME"])
		if configHome == "" {
			configHome = filepath.Join(homeDir, ".config")
		}
		return autoStartTargetInfo{
			GOOS:   goos,
			Kind:   "XDG Autostart",
			Target: filepath.Join(configHome, "autostart", autoStartLinuxFileName),
		}, nil
	}
}

func autoStartContent(goos, executable string, args []string) (string, error) {
	if strings.TrimSpace(executable) == "" {
		return "", fmt.Errorf("executable path is required")
	}
	commandLine := shellJoin(append([]string{executable}, args...))
	switch goos {
	case "darwin":
		entries := make([]string, 0, len(args)+1)
		entries = append(entries, fmt.Sprintf("    <string>%s</string>", xmlEscape(executable)))
		for _, arg := range args {
			entries = append(entries, fmt.Sprintf("    <string>%s</string>", xmlEscape(arg)))
		}
		return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%s</string>
  <key>ProgramArguments</key>
  <array>
%s
  </array>
  <key>RunAtLoad</key>
  <true/>
</dict>
</plist>
`, autoStartLaunchAgentLabel, strings.Join(entries, "\n")), nil
	case "windows":
		return fmt.Sprintf("@echo off\r\nstart \"\" /min \"%s\" %s\r\n", windowsEscape(executable), windowsArgs(args)), nil
	default:
		return fmt.Sprintf(`[Desktop Entry]
Type=Application
Version=1.0
Name=Token Manager Tools
Comment=启动本机账号池服务
Exec=sh -lc "%s"
Terminal=false
X-GNOME-Autostart-enabled=true
`, desktopEscape("exec "+commandLine)), nil
	}
}

func shellJoin(parts []string) string {
	quoted := make([]string, 0, len(parts))
	for _, part := range parts {
		quoted = append(quoted, shellQuote(part))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func desktopEscape(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
	)
	return replacer.Replace(value)
}

func xmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}

func windowsEscape(value string) string {
	return strings.ReplaceAll(value, `"`, `""`)
}

func windowsArgs(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		if strings.ContainsAny(arg, " \t\"") {
			quoted = append(quoted, `"`+windowsEscape(arg)+`"`)
			continue
		}
		quoted = append(quoted, arg)
	}
	return strings.Join(quoted, " ")
}
