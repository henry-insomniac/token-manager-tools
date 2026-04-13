package desktopapp

import (
	"fmt"
	"strings"

	"github.com/henry-insomniac/token-manager-tools/internal/accountpool"
)

const (
	titlebarActionRefreshUsage  = "titlebar_refresh_usage"
	titlebarActionRunAutoSwitch = "titlebar_run_auto_switch"
	titlebarActionShowQuota     = "titlebar_show_quota"
)

func titlebarSummary(profiles []accountpool.ProfileSnapshot) string {
	active := currentManagedProfile(profiles)
	if active == nil {
		for _, profile := range profiles {
			if profile.IsActive {
				return "当前：系统默认资料"
			}
		}
		return "当前：未切到托管槽位"
	}

	summary := profileUsageSummary(*active)
	if strings.TrimSpace(summary) == "" {
		return fmt.Sprintf("当前：%s", active.Name)
	}
	return fmt.Sprintf("当前：%s · %s", active.Name, summary)
}

func currentManagedProfile(profiles []accountpool.ProfileSnapshot) *accountpool.ProfileSnapshot {
	for _, profile := range profiles {
		if profile.IsActive && !profile.IsDefault {
			profileCopy := profile
			return &profileCopy
		}
	}
	return nil
}
