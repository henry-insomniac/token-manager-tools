package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/henry-insomniac/token-manager-tools/internal/platform"
)

type backgroundServerState struct {
	PID       int    `json:"pid"`
	Addr      string `json:"addr"`
	URL       string `json:"url"`
	LogPath   string `json:"logPath"`
	StartedAt string `json:"startedAt"`
}

type backgroundPaths struct {
	ManagerDir string
	PIDPath    string
	LogPath    string
}

func runStart(args []string) error {
	addr, openBrowser, err := parseServeArgs(args)
	if err != nil {
		return err
	}
	paths, err := resolveBackgroundPaths()
	if err != nil {
		return err
	}
	existing, err := readBackgroundState(paths.PIDPath)
	if err != nil {
		return err
	}
	if existing != nil && platform.ProcessExists(existing.PID) {
		fmt.Printf("账号池服务已在后台运行: %s\n", existing.URL)
		return nil
	}
	if existing != nil {
		_ = os.Remove(paths.PIDPath)
	}

	executable, err := os.Executable()
	if err != nil {
		return err
	}
	serveArgs := []string{"serve", addr}
	if !openBrowser {
		serveArgs = append(serveArgs, "--no-open")
	}
	pid, err := platform.StartDetached(executable, serveArgs, paths.LogPath)
	if err != nil {
		return err
	}
	state := backgroundServerState{
		PID:       pid,
		Addr:      addr,
		URL:       "http://" + callbackHostForAddr(addr) + "/",
		LogPath:   paths.LogPath,
		StartedAt: time.Now().Format(time.RFC3339),
	}
	if err := writeBackgroundState(paths.PIDPath, state); err != nil {
		return err
	}
	if err := waitForServerReady(state.URL, 4*time.Second); err != nil {
		return fmt.Errorf("后台服务已启动但暂未响应，请稍后访问 %s；日志: %s", state.URL, paths.LogPath)
	}
	fmt.Printf("账号池服务已在后台启动: %s\n", state.URL)
	fmt.Printf("停止服务: token-manager stop\n")
	return nil
}

func runStop() error {
	paths, err := resolveBackgroundPaths()
	if err != nil {
		return err
	}
	state, err := readBackgroundState(paths.PIDPath)
	if err != nil {
		return err
	}
	if state == nil {
		fmt.Println("账号池服务未在后台运行")
		return nil
	}
	if !platform.ProcessExists(state.PID) {
		_ = os.Remove(paths.PIDPath)
		fmt.Println("账号池服务未在后台运行，已清理过期状态")
		return nil
	}
	if err := platform.StopProcess(state.PID); err != nil {
		return err
	}
	_ = os.Remove(paths.PIDPath)
	fmt.Println("账号池服务已停止")
	return nil
}

func runStatus() error {
	paths, err := resolveBackgroundPaths()
	if err != nil {
		return err
	}
	state, err := readBackgroundState(paths.PIDPath)
	if err != nil {
		return err
	}
	if state == nil || !platform.ProcessExists(state.PID) {
		if state != nil {
			_ = os.Remove(paths.PIDPath)
		}
		fmt.Println("账号池服务未在后台运行")
		return nil
	}
	fmt.Printf("账号池服务正在后台运行: %s\n", state.URL)
	fmt.Printf("PID: %d\n", state.PID)
	fmt.Printf("日志: %s\n", state.LogPath)
	return nil
}

func resolveBackgroundPaths() (backgroundPaths, error) {
	paths, err := platform.DefaultPaths(platform.PathOptions{})
	if err != nil {
		return backgroundPaths{}, err
	}
	managerDir := firstNonEmpty(os.Getenv("OPENCLAW_MANAGER_DIR"), paths.ManagerState)
	if strings.TrimSpace(managerDir) == "" {
		return backgroundPaths{}, errors.New("无法确定状态目录")
	}
	if err := os.MkdirAll(managerDir, 0o755); err != nil {
		return backgroundPaths{}, err
	}
	return backgroundPaths{
		ManagerDir: managerDir,
		PIDPath:    filepath.Join(managerDir, "server.pid.json"),
		LogPath:    filepath.Join(managerDir, "server.log"),
	}, nil
}

func readBackgroundState(path string) (*backgroundServerState, error) {
	buffer, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var state backgroundServerState
	if err := json.Unmarshal(buffer, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func writeBackgroundState(path string, state backgroundServerState) error {
	buffer, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	buffer = append(buffer, '\n')
	return os.WriteFile(path, buffer, 0o600)
}

func waitForServerReady(baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 350 * time.Millisecond}
	for time.Now().Before(deadline) {
		resp, err := client.Get(strings.TrimRight(baseURL, "/") + "/api/profiles")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				return nil
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	return errors.New("server not ready")
}
