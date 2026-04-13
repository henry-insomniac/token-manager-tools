package appservice

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/henry-insomniac/token-manager-tools/internal/accountpool"
)

type Service struct {
	pool *accountpool.AccountPool

	pendingMu sync.Mutex
	pending   map[string]pendingLoginFlow
	loopOnce  sync.Once
}

type LoginStartResult struct {
	ProfileName string `json:"profileName"`
	AuthURL     string `json:"authUrl"`
	RedirectURL string `json:"redirectUrl"`
}

type LoginCompleteResult struct {
	ProfileName  string `json:"profileName"`
	AccountEmail string `json:"accountEmail,omitempty"`
	Message      string `json:"message"`
}

type OAuthCallbackResult struct {
	ProfileName  string
	AccountEmail string
	Message      string
}

type UsageRefreshResult struct {
	Refreshed []string          `json:"refreshed"`
	Failed    map[string]string `json:"failed"`
}

type pendingLoginFlow struct {
	flow      accountpool.LoginFlow
	createdAt time.Time
}

const loginFlowTTL = 10 * time.Minute

func New(pool *accountpool.AccountPool) *Service {
	service := &Service{
		pool:    pool,
		pending: map[string]pendingLoginFlow{},
	}
	service.startAutoSwitchLoop()
	return service
}

func (service *Service) ListProfiles() ([]accountpool.ProfileSnapshot, error) {
	return service.pool.ListProfiles()
}

func (service *Service) CreateProfile(name string) (accountpool.ProfileSnapshot, error) {
	return service.pool.CreateProfile(name)
}

func (service *Service) ActivateProfile(name string) error {
	return service.pool.ActivateProfile(name)
}

func (service *Service) ProbeProfile(name string) (accountpool.ProbeResult, error) {
	return service.pool.ProbeProfile(name)
}

func (service *Service) RemoveProfile(name string) (accountpool.RemoveResult, error) {
	return service.pool.RemoveProfile(name)
}

func (service *Service) SetOAuthRedirectURL(raw string) {
	service.pool.SetOAuthRedirectURL(raw)
}

func (service *Service) OAuthRedirectURL() string {
	return service.pool.OAuthRedirectURL()
}

func (service *Service) AutoSwitchStatus() (accountpool.AutoSwitchStatus, error) {
	return service.pool.AutoSwitchStatus()
}

func (service *Service) SetAutoSwitchEnabled(enabled bool) (accountpool.AutoSwitchRunResult, error) {
	return service.pool.SetAutoSwitchEnabled(enabled)
}

func (service *Service) RunAutoSwitchNow() (accountpool.AutoSwitchRunResult, error) {
	return service.pool.RunAutoSwitchNow()
}

func (service *Service) StartLogin(name string) (LoginStartResult, error) {
	return service.startLoginAt(name, time.Now())
}

func (service *Service) startLoginAt(name string, now time.Time) (LoginStartResult, error) {
	flow, err := service.pool.StartLogin(name)
	if err != nil {
		return LoginStartResult{}, err
	}
	service.pendingMu.Lock()
	defer service.pendingMu.Unlock()
	service.cleanupExpiredLoginFlowsLocked(now)
	service.pending[flow.State] = pendingLoginFlow{
		flow:      flow,
		createdAt: now,
	}
	return LoginStartResult{
		ProfileName: flow.ProfileName,
		AuthURL:     flow.AuthURL,
		RedirectURL: flow.RedirectURL,
	}, nil
}

func (service *Service) CompleteManualLogin(profileName, rawInput string) (LoginCompleteResult, error) {
	return service.completeManualLoginAt(profileName, rawInput, time.Now())
}

func (service *Service) completeManualLoginAt(profileName, rawInput string, now time.Time) (LoginCompleteResult, error) {
	parsed, err := accountpool.ParseManualLoginInput(rawInput)
	if err != nil {
		return LoginCompleteResult{}, err
	}
	pending, err := service.pendingLoginFlowForProfile(profileName, parsed.State, now)
	if err != nil {
		return LoginCompleteResult{}, err
	}
	tokens, err := service.pool.CompleteLogin(pending.flow.ProfileName, parsed.Code, pending.flow.Verifier)
	if err != nil {
		return LoginCompleteResult{}, err
	}
	service.discardPendingLoginFlow(pending.flow.State)
	label := tokens.Email
	if strings.TrimSpace(label) == "" {
		label = pending.flow.ProfileName
	}
	return LoginCompleteResult{
		ProfileName:  pending.flow.ProfileName,
		AccountEmail: tokens.Email,
		Message:      fmt.Sprintf("%s 已写入本机账号池。", label),
	}, nil
}

func (service *Service) CompleteOAuthCallback(state, code, authErr string) (OAuthCallbackResult, error) {
	return service.completeOAuthCallbackAt(state, code, authErr, time.Now())
}

func (service *Service) completeOAuthCallbackAt(state, code, authErr string, now time.Time) (OAuthCallbackResult, error) {
	state = strings.TrimSpace(state)
	if state == "" {
		return OAuthCallbackResult{}, errors.New("缺少登录校验信息。请回到账号池页面重新登录。")
	}
	pending, err := service.pendingLoginFlowByState(state, now)
	if err != nil {
		return OAuthCallbackResult{}, err
	}
	flow := pending.flow
	result := OAuthCallbackResult{ProfileName: flow.ProfileName}
	if authErr = strings.TrimSpace(authErr); authErr != "" {
		service.discardPendingLoginFlow(state)
		return result, errors.New(authErr)
	}
	code = strings.TrimSpace(code)
	if code == "" {
		service.discardPendingLoginFlow(state)
		return result, errors.New("回调缺少授权 code。请重新登录。")
	}
	tokens, err := service.pool.CompleteLogin(flow.ProfileName, code, flow.Verifier)
	if err != nil {
		return result, err
	}
	service.discardPendingLoginFlow(state)
	result.AccountEmail = tokens.Email
	label := tokens.Email
	if strings.TrimSpace(label) == "" {
		label = flow.ProfileName
	}
	result.Message = fmt.Sprintf("%s 已写入本机账号池。正在返回账号池。", label)
	return result, nil
}

func (service *Service) RefreshUsage() (UsageRefreshResult, error) {
	profiles, err := service.pool.ListProfiles()
	if err != nil {
		return UsageRefreshResult{}, err
	}
	result := UsageRefreshResult{
		Refreshed: make([]string, 0, len(profiles)),
		Failed:    map[string]string{},
	}
	for _, profile := range profiles {
		if profile.IsDefault || !profile.HasCredential {
			continue
		}
		if _, err := service.pool.ProbeProfile(profile.Name); err != nil {
			result.Failed[profile.Name] = err.Error()
			continue
		}
		result.Refreshed = append(result.Refreshed, profile.Name)
	}
	return result, nil
}

func (service *Service) startAutoSwitchLoop() {
	service.loopOnce.Do(func() {
		go func() {
			timer := time.NewTimer(accountpool.NextAutoSwitchPollInterval())
			defer timer.Stop()
			for range timer.C {
				if _, err := service.pool.RunAutoSwitchNow(); err != nil {
					timer.Reset(accountpool.NextAutoSwitchPollInterval())
					continue
				}
				timer.Reset(accountpool.NextAutoSwitchPollInterval())
			}
		}()
	})
}

func (service *Service) pendingLoginFlowByState(state string, now time.Time) (pendingLoginFlow, error) {
	service.pendingMu.Lock()
	defer service.pendingMu.Unlock()
	service.cleanupExpiredLoginFlowsLocked(now)
	pending, ok := service.pending[state]
	if !ok {
		return pendingLoginFlow{}, fmt.Errorf("登录流程已失效。请回到账号池页面重新登录。")
	}
	return pending, nil
}

func (service *Service) pendingLoginFlowForProfile(profileName, state string, now time.Time) (pendingLoginFlow, error) {
	service.pendingMu.Lock()
	defer service.pendingMu.Unlock()
	service.cleanupExpiredLoginFlowsLocked(now)
	if strings.TrimSpace(state) != "" {
		pending, ok := service.pending[state]
		if !ok {
			return pendingLoginFlow{}, fmt.Errorf("登录流程已失效。请先重新点“登录”。")
		}
		if pending.flow.ProfileName != profileName {
			return pendingLoginFlow{}, fmt.Errorf("回调槽位和当前槽位不一致。请重新开始登录。")
		}
		return pending, nil
	}

	var latest pendingLoginFlow
	found := false
	for _, pending := range service.pending {
		if pending.flow.ProfileName != profileName {
			continue
		}
		if !found || pending.createdAt.After(latest.createdAt) {
			latest = pending
			found = true
		}
	}
	if !found {
		return pendingLoginFlow{}, fmt.Errorf("没有找到未完成的登录流程。请先点“登录”。")
	}
	return latest, nil
}

func (service *Service) discardPendingLoginFlow(state string) {
	service.pendingMu.Lock()
	defer service.pendingMu.Unlock()
	delete(service.pending, state)
}

func (service *Service) cleanupExpiredLoginFlowsLocked(now time.Time) {
	for state, pending := range service.pending {
		if now.Sub(pending.createdAt) > loginFlowTTL {
			delete(service.pending, state)
		}
	}
}
