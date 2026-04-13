package desktopapp

import (
	"github.com/henry-insomniac/token-manager-tools/internal/accountpool"
	"github.com/henry-insomniac/token-manager-tools/internal/appservice"
	"github.com/henry-insomniac/token-manager-tools/internal/desktopruntime"
)

type Bindings struct {
	service        *appservice.Service
	runtimeManager *desktopruntime.Manager
	app            *App
}

func NewBindings(service *appservice.Service, runtimeManager *desktopruntime.Manager, app *App) *Bindings {
	return &Bindings{
		service:        service,
		runtimeManager: runtimeManager,
		app:            app,
	}
}

func (bindings *Bindings) ListProfiles() ([]accountpool.ProfileSnapshot, error) {
	return bindings.service.ListProfiles()
}

func (bindings *Bindings) CreateProfile(name string) (accountpool.ProfileSnapshot, error) {
	profile, err := bindings.service.CreateProfile(name)
	if err == nil {
		bindings.notifyChanged()
	}
	return profile, err
}

func (bindings *Bindings) StartLogin(name string) (appservice.LoginStartResult, error) {
	if bindings.app != nil {
		if err := bindings.app.EnsureLoginReady(); err != nil {
			return appservice.LoginStartResult{}, err
		}
	}
	return bindings.service.StartLogin(name)
}

func (bindings *Bindings) CompleteManualLogin(profileName, input string) (appservice.LoginCompleteResult, error) {
	result, err := bindings.service.CompleteManualLogin(profileName, input)
	if err == nil {
		bindings.notifyChanged()
	}
	return result, err
}

func (bindings *Bindings) ProbeProfile(name string) (accountpool.ProbeResult, error) {
	result, err := bindings.service.ProbeProfile(name)
	if err == nil {
		bindings.notifyChanged()
	}
	return result, err
}

func (bindings *Bindings) ActivateProfile(name string) (map[string]string, error) {
	if err := bindings.service.ActivateProfile(name); err != nil {
		return nil, err
	}
	bindings.notifyChanged()
	return map[string]string{"message": "已切换默认运行槽位"}, nil
}

func (bindings *Bindings) RemoveProfile(name string) (accountpool.RemoveResult, error) {
	result, err := bindings.service.RemoveProfile(name)
	if err == nil {
		bindings.notifyChanged()
	}
	return result, err
}

func (bindings *Bindings) RefreshUsage() (appservice.UsageRefreshResult, error) {
	result, err := bindings.service.RefreshUsage()
	if err == nil {
		bindings.notifyChanged()
	}
	return result, err
}

func (bindings *Bindings) GetAutoSwitchStatus() (accountpool.AutoSwitchStatus, error) {
	return bindings.service.AutoSwitchStatus()
}

func (bindings *Bindings) SetAutoSwitchEnabled(enabled bool) (accountpool.AutoSwitchRunResult, error) {
	result, err := bindings.service.SetAutoSwitchEnabled(enabled)
	if err == nil {
		bindings.notifyChanged()
	}
	return result, err
}

func (bindings *Bindings) RunAutoSwitchCheck() (accountpool.AutoSwitchRunResult, error) {
	result, err := bindings.service.RunAutoSwitchNow()
	if err == nil {
		bindings.notifyChanged()
	}
	return result, err
}

func (bindings *Bindings) GetDesktopStatus() (desktopruntime.Status, error) {
	return bindings.runtimeManager.Status()
}

func (bindings *Bindings) SetDesktopAutoStart(enabled bool) (desktopruntime.Status, error) {
	status, err := bindings.runtimeManager.SetAutoStart(enabled)
	if err == nil {
		return status, nil
	}
	status.AutoStartMessage = err.Error()
	return status, nil
}

func (bindings *Bindings) notifyChanged() {
	if bindings.app == nil {
		return
	}
	bindings.app.RefreshDesktopMenus()
}
