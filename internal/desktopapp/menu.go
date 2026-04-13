package desktopapp

import (
	"fmt"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/henry-insomniac/token-manager-tools/internal/accountpool"
	"github.com/henry-insomniac/token-manager-tools/internal/appservice"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var menuUpdateMu sync.Mutex

func (app *App) ApplicationMenu() *menu.Menu {
	if goruntime.GOOS != "darwin" {
		return nil
	}

	root := menu.NewMenu()
	root.Append(menu.AppMenu())
	root.Append(menu.EditMenu())
	app.buildQuickActionsMenu(root.AddSubmenu("快捷操作"))
	root.Append(menu.WindowMenu())
	return root
}

func (app *App) RefreshApplicationMenu() {
	if app.ctx == nil || goruntime.GOOS != "darwin" {
		return
	}
	menuUpdateMu.Lock()
	defer menuUpdateMu.Unlock()
	runtime.MenuSetApplicationMenu(app.ctx, app.ApplicationMenu())
}

func (app *App) RefreshDesktopMenus() {
	app.RefreshApplicationMenu()
	app.RefreshStatusItem()
	app.RefreshTitlebarAccessory()
}

func (app *App) buildQuickActionsMenu(target *menu.Menu) {
	target.AddText("显示主窗口", nil, func(*menu.CallbackData) {
		app.ShowWindow()
	})
	target.AddText("刷新全部额度", nil, func(*menu.CallbackData) {
		go app.refreshUsageFromMenu()
	})
	target.AddText("立即执行自动切换检查", nil, func(*menu.CallbackData) {
		go app.runAutoSwitchCheckFromMenu()
	})
	target.AddSeparator()

	profiles, err := app.service.ListProfiles()
	if err != nil {
		target.AddText("读取槽位失败", nil, nil).Disable()
		target.AddText(err.Error(), nil, nil).Disable()
		target.AddSeparator()
		target.AddText("退出客户端", nil, func(*menu.CallbackData) {
			if app.ctx != nil {
				runtime.Quit(app.ctx)
			}
		})
		return
	}

	target.AddText("当前运行槽位："+currentSlotMenuLabel(profiles), nil, nil).Disable()

	quotaMenu := target.AddSubmenu("查看额度")
	for _, profile := range managedProfiles(profiles) {
		name := profile.Name
		quotaMenu.AddText(profileQuotaMenuLabel(profile), nil, func(*menu.CallbackData) {
			app.FocusProfile(name)
		})
	}
	if len(quotaMenu.Items) == 0 {
		quotaMenu.AddText("还没有托管槽位", nil, nil).Disable()
	}

	checkMenu := target.AddSubmenu("快速检查")
	for _, profile := range managedProfiles(profiles) {
		if !profile.HasCredential {
			continue
		}
		name := profile.Name
		checkMenu.AddText(profile.Name, nil, func(*menu.CallbackData) {
			go app.probeProfileFromMenu(name)
		})
	}
	if len(checkMenu.Items) == 0 {
		checkMenu.AddText("还没有已登录槽位", nil, nil).Disable()
	}

	target.AddSeparator()
	target.AddText("退出客户端", nil, func(*menu.CallbackData) {
		if app.ctx != nil {
			runtime.Quit(app.ctx)
		}
	})
}

func (app *App) refreshUsageFromMenu() {
	result, err := app.service.RefreshUsage()
	if err != nil {
		app.emitDesktopAction("error", "刷新失败", err.Error(), "")
		return
	}
	app.RefreshDesktopMenus()
	body := usageRefreshMessage(result)
	app.emitDesktopAction("success", "额度已刷新", body, "")
}

func (app *App) runAutoSwitchCheckFromMenu() {
	result, err := app.service.RunAutoSwitchNow()
	if err != nil {
		app.emitDesktopAction("error", "自动切换检查失败", err.Error(), "")
		return
	}
	app.RefreshDesktopMenus()
	body := strings.TrimSpace(result.Status.LastMessage)
	if body == "" {
		body = "已完成自动切换检查。"
	}
	app.emitDesktopAction("success", "自动切换检查已完成", body, "")
}

func (app *App) probeProfileFromMenu(name string) {
	result, err := app.service.ProbeProfile(name)
	if err != nil {
		app.emitDesktopAction("error", "检查失败", err.Error(), name)
		return
	}
	app.RefreshDesktopMenus()
	body := singleProfileProbeMessage(result)
	app.emitDesktopAction("success", "额度已更新", body, name)
}

func (app *App) emitDesktopAction(status, title, body, profileName string) {
	if app.ctx == nil {
		return
	}
	payload := map[string]string{
		"status": status,
		"title":  title,
		"body":   body,
		"at":     time.Now().Format(time.RFC3339Nano),
	}
	if strings.TrimSpace(profileName) != "" {
		payload["profileName"] = profileName
	}
	runtime.EventsEmit(app.ctx, DesktopActionEvent, payload)
}

func managedProfiles(profiles []accountpool.ProfileSnapshot) []accountpool.ProfileSnapshot {
	result := make([]accountpool.ProfileSnapshot, 0, len(profiles))
	for _, profile := range profiles {
		if profile.IsDefault {
			continue
		}
		result = append(result, profile)
	}
	return result
}

func currentSlotMenuLabel(profiles []accountpool.ProfileSnapshot) string {
	for _, profile := range profiles {
		if profile.IsActive && !profile.IsDefault {
			return profile.Name
		}
	}
	for _, profile := range profiles {
		if profile.IsActive {
			return "系统默认资料"
		}
	}
	return "未切到托管槽位"
}

func profileQuotaMenuLabel(profile accountpool.ProfileSnapshot) string {
	parts := []string{profile.Name}
	if profile.IsActive {
		parts = append(parts, "当前")
	}
	parts = append(parts, profileUsageSummary(profile))
	return strings.Join(parts, " · ")
}

func profileUsageSummary(profile accountpool.ProfileSnapshot) string {
	if !profile.HasCredential {
		return "未登录"
	}
	if profile.CachedProbe == nil {
		return "未检查"
	}

	usageParts := make([]string, 0, 2)
	if profile.CachedProbe.Usage.FiveHour != nil {
		usageParts = append(usageParts, fmt.Sprintf("5h %d%%", profile.CachedProbe.Usage.FiveHour.LeftPercent))
	}
	if profile.CachedProbe.Usage.Week != nil {
		usageParts = append(usageParts, fmt.Sprintf("周 %d%%", profile.CachedProbe.Usage.Week.LeftPercent))
	}
	if len(usageParts) > 0 {
		return strings.Join(usageParts, " · ")
	}
	reason := strings.TrimSpace(profile.CachedProbe.Reason)
	if reason != "" {
		return reason
	}
	switch profile.CachedProbe.Status {
	case "ok":
		return "可用"
	case "warn":
		return "偏紧"
	case "danger":
		return "危险"
	default:
		return "已登录"
	}
}

func usageRefreshMessage(result appservice.UsageRefreshResult) string {
	refreshed := len(result.Refreshed)
	failed := len(result.Failed)
	switch {
	case refreshed == 0 && failed == 0:
		return "没有可刷新的已登录槽位。"
	case refreshed > 0 && failed == 0:
		return fmt.Sprintf("已刷新 %d 个槽位。", refreshed)
	case refreshed == 0:
		return fmt.Sprintf("没有刷新成功，%d 个槽位失败。", failed)
	default:
		return fmt.Sprintf("已刷新 %d 个槽位，%d 个失败。", refreshed, failed)
	}
}

func singleProfileProbeMessage(result accountpool.ProbeResult) string {
	label := result.AccountEmail
	if strings.TrimSpace(label) == "" {
		label = result.ProfileName
	}

	usageParts := make([]string, 0, 2)
	if result.Usage.FiveHour != nil {
		usageParts = append(usageParts, fmt.Sprintf("5h %d%%", result.Usage.FiveHour.LeftPercent))
	}
	if result.Usage.Week != nil {
		usageParts = append(usageParts, fmt.Sprintf("周 %d%%", result.Usage.Week.LeftPercent))
	}
	if len(usageParts) > 0 {
		return fmt.Sprintf("%s：%s", label, strings.Join(usageParts, " · "))
	}
	if strings.TrimSpace(result.Reason) != "" {
		return fmt.Sprintf("%s：%s", label, result.Reason)
	}
	return fmt.Sprintf("%s 已更新额度状态。", label)
}
