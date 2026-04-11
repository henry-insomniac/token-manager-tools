package accountpool

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type runtimeSettings struct {
	LastProxyURL string           `json:"lastProxyUrl,omitempty"`
	ProxyURL     string           `json:"proxyUrl,omitempty"`
	AutoSwitch   AutoSwitchStatus `json:"autoSwitch,omitempty"`
}

type proxyCandidate struct {
	URL    string
	Source string
}

func (pool *AccountPool) loadRuntimeSettings() runtimeSettings {
	pool.settingsMu.Lock()
	defer pool.settingsMu.Unlock()
	return pool.loadRuntimeSettingsLocked()
}

func (pool *AccountPool) loadRuntimeSettingsLocked() runtimeSettings {
	if strings.TrimSpace(pool.settingsPath) == "" {
		return runtimeSettings{}
	}
	settings := readJSONFile(pool.settingsPath, runtimeSettings{})
	settings.AutoSwitch = normalizeAutoSwitchStatus(settings.AutoSwitch)
	return settings
}

func (pool *AccountPool) saveRuntimeSettings(settings runtimeSettings) error {
	pool.settingsMu.Lock()
	defer pool.settingsMu.Unlock()
	return pool.saveRuntimeSettingsLocked(settings)
}

func (pool *AccountPool) saveRuntimeSettingsLocked(settings runtimeSettings) error {
	if strings.TrimSpace(pool.settingsPath) == "" {
		return nil
	}
	settings.AutoSwitch = normalizeAutoSwitchStatus(settings.AutoSwitch)
	return writeJSONFile(pool.settingsPath, settings)
}

func (pool *AccountPool) cachedProxyURL() string {
	settings := pool.loadRuntimeSettings()
	return firstNonEmpty(strings.TrimSpace(settings.LastProxyURL), strings.TrimSpace(settings.ProxyURL))
}

func (pool *AccountPool) rememberWorkingProxy(proxyURL string) {
	if strings.TrimSpace(pool.settingsPath) == "" {
		return
	}
	pool.settingsMu.Lock()
	defer pool.settingsMu.Unlock()
	settings := pool.loadRuntimeSettingsLocked()
	if strings.TrimSpace(settings.LastProxyURL) == strings.TrimSpace(proxyURL) {
		return
	}
	settings.LastProxyURL = strings.TrimSpace(proxyURL)
	_ = pool.saveRuntimeSettingsLocked(settings)
}

func (pool *AccountPool) newHTTPClient(proxyURL string) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	if strings.TrimSpace(proxyURL) != "" {
		if parsed, err := url.Parse(proxyURL); err == nil {
			transport.Proxy = http.ProxyURL(parsed)
		}
	}
	transport.DialContext = (&net.Dialer{
		Timeout:   4 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext
	transport.TLSHandshakeTimeout = 5 * time.Second
	transport.ResponseHeaderTimeout = 15 * time.Second
	transport.ExpectContinueTimeout = 1 * time.Second
	return &http.Client{Transport: transport}
}

func (pool *AccountPool) doRequest(req *http.Request) (*http.Response, error) {
	if pool.httpClientFixed {
		cloned, err := cloneRequest(req)
		if err != nil {
			return nil, err
		}
		return pool.httpClient.Do(cloned)
	}

	candidates := pool.proxyCandidates()
	var lastErr error
	for _, candidate := range candidates {
		cloned, err := cloneRequest(req)
		if err != nil {
			return nil, err
		}
		client := pool.newHTTPClient(candidate.URL)
		resp, err := client.Do(cloned)
		if err == nil {
			switch candidate.Source {
			case "env":
			case "direct":
				pool.rememberWorkingProxy("")
			default:
				pool.rememberWorkingProxy(candidate.URL)
			}
			return resp, nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = errors.New("network unavailable")
	}
	return nil, fmt.Errorf("无法连通 OpenAI。已自动尝试直连和常见本地代理端口；请开启代理软件的本地 HTTP/Mixed 端口或开启 TUN: %w", lastErr)
}

func (pool *AccountPool) proxyCandidates() []proxyCandidate {
	seen := map[string]bool{}
	list := make([]proxyCandidate, 0, 16)
	add := func(proxyURL, source string) {
		key := source + "|" + strings.TrimSpace(proxyURL)
		if seen[key] {
			return
		}
		seen[key] = true
		list = append(list, proxyCandidate{URL: strings.TrimSpace(proxyURL), Source: source})
	}

	if envProxy := currentProcessProxyURL(); envProxy != "" {
		add(envProxy, "env")
	}
	if cached := pool.cachedProxyURL(); cached != "" {
		add(cached, "cache")
	}
	add("", "direct")
	for _, proxyURL := range commonLoopbackProxyCandidates() {
		add(proxyURL, "auto")
	}
	return list
}

func currentProcessProxyURL() string {
	for _, key := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func commonLoopbackProxyCandidates() []string {
	ports := []string{
		"7890",
		"7897",
		"10809",
		"10808",
		"20171",
		"6152",
	}
	hosts := []string{"127.0.0.1", "localhost"}
	candidates := make([]string, 0, len(ports)*len(hosts))
	for _, host := range hosts {
		for _, port := range ports {
			candidates = append(candidates, "http://"+net.JoinHostPort(host, port))
		}
	}
	return candidates
}

func cloneRequest(req *http.Request) (*http.Request, error) {
	cloned := req.Clone(req.Context())
	if req.Body != nil && req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		cloned.Body = body
	}
	return cloned, nil
}
