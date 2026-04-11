package accountpool

import (
	"net/http"
	"sync"
)

const DefaultProfileName = "default"

type Config struct {
	HomeDir      string
	OpenClawHome string
	CodexHome    string
	ManagerDir   string

	DefaultOpenDir string
	DefaultCodex   string

	OAuthAuthorizeURL string
	OAuthTokenURL     string
	OAuthRedirectURL  string
	UsageURL          string
	HTTPClient        *http.Client
	Clock             func() int64
}

type AccountPool struct {
	homeDir         string
	openClawHome    string
	codexHome       string
	managerDir      string
	statePath       string
	settingsPath    string
	defaultOpenDir  string
	defaultCodex    string
	authorizeURL    string
	tokenURL        string
	redirectURL     string
	usageURL        string
	httpClient      *http.Client
	httpClientFixed bool
	clock           func() int64
	settingsMu      sync.Mutex
	autoSwitchMu    sync.Mutex
}

type ProfileSnapshot struct {
	Name          string               `json:"name"`
	IsDefault     bool                 `json:"isDefault"`
	IsActive      bool                 `json:"isActive"`
	StateDir      string               `json:"stateDir"`
	CodexHome     string               `json:"codexHome"`
	ConfigPath    string               `json:"configPath"`
	AuthStorePath string               `json:"authStorePath"`
	CodexAuthPath string               `json:"codexAuthPath"`
	HasConfig     bool                 `json:"hasConfig"`
	HasAuthStore  bool                 `json:"hasAuthStore"`
	HasCodexAuth  bool                 `json:"hasCodexAuth"`
	HasCredential bool                 `json:"hasCredential"`
	AccountID     string               `json:"accountId,omitempty"`
	AccountEmail  string               `json:"accountEmail,omitempty"`
	Status        string               `json:"status"`
	StatusReason  string               `json:"statusReason"`
	CachedProbe   *CachedProbeSnapshot `json:"cachedProbe,omitempty"`
}

type RemoveResult struct {
	ProfileName       string  `json:"profileName"`
	Message           string  `json:"message"`
	ArchivedStateDir  *string `json:"archivedStateDir,omitempty"`
	ArchivedCodexHome *string `json:"archivedCodexHome,omitempty"`
}

type State struct {
	ActiveProfileName *string `json:"activeProfileName,omitempty"`
}

type AuthStore struct {
	Version    int                       `json:"version"`
	Profiles   map[string]map[string]any `json:"profiles"`
	LastGood   map[string]string         `json:"lastGood"`
	UsageStats map[string]map[string]any `json:"usageStats"`
}

type OAuthTokens struct {
	Access    string
	Refresh   string
	IDToken   string
	Expires   int64
	AccountID string
	Email     string
}

type LoginFlow struct {
	ProfileName string
	AuthURL     string
	Verifier    string
	State       string
	RedirectURL string
}

type UsageWindow struct {
	Label       string  `json:"label"`
	UsedPercent int     `json:"usedPercent"`
	LeftPercent int     `json:"leftPercent"`
	ResetAt     *string `json:"resetAt,omitempty"`
	ResetInMs   *int    `json:"resetInMs,omitempty"`
}

type UsageSnapshot struct {
	Plan     *string      `json:"plan,omitempty"`
	FiveHour *UsageWindow `json:"fiveHour,omitempty"`
	Week     *UsageWindow `json:"week,omitempty"`
}

type CachedProbeSnapshot struct {
	Status      string        `json:"status"`
	Reason      string        `json:"reason"`
	Usage       UsageSnapshot `json:"usage"`
	LastProbeAt *string       `json:"lastProbeAt,omitempty"`
}

type ProbeResult struct {
	ProfileName  string        `json:"profileName"`
	Status       string        `json:"status"`
	Reason       string        `json:"reason"`
	Usage        UsageSnapshot `json:"usage"`
	AccountID    string        `json:"accountId,omitempty"`
	AccountEmail string        `json:"accountEmail,omitempty"`
}

type AutoSwitchEvent struct {
	At      string  `json:"at"`
	Type    string  `json:"type"`
	Message string  `json:"message"`
	From    *string `json:"from,omitempty"`
	To      *string `json:"to,omitempty"`
	Reason  *string `json:"reason,omitempty"`
}

type AutoSwitchStatus struct {
	Enabled                  bool              `json:"enabled"`
	PollIntervalMinSeconds   int               `json:"pollIntervalMinSeconds"`
	PollIntervalMaxSeconds   int               `json:"pollIntervalMaxSeconds"`
	MinSwitchIntervalSeconds int               `json:"minSwitchIntervalSeconds"`
	LastCheckedAt            *string           `json:"lastCheckedAt,omitempty"`
	LastSwitchedAt           *string           `json:"lastSwitchedAt,omitempty"`
	LastMessage              string            `json:"lastMessage,omitempty"`
	LastFrom                 *string           `json:"lastFrom,omitempty"`
	LastTo                   *string           `json:"lastTo,omitempty"`
	Events                   []AutoSwitchEvent `json:"events,omitempty"`
}

type AutoSwitchRunResult struct {
	Switched bool             `json:"switched"`
	Status   AutoSwitchStatus `json:"status"`
}
