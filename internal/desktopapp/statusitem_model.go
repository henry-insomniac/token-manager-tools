package desktopapp

import (
	"net/url"
	"strings"

	"github.com/henry-insomniac/token-manager-tools/internal/accountpool"
)

const (
	statusItemTitle   = "TM额度"
	statusItemTooltip = "Token Manager Tools"

	statusActionShowWindow    = "show_window"
	statusActionRefreshUsage  = "refresh_usage"
	statusActionRunAutoSwitch = "run_auto_switch"
	statusActionQuit          = "quit"
	statusActionFocusPrefix   = "focus_profile:"
	statusActionProbePrefix   = "probe_profile:"
)

type statusItemMenuItem struct {
	Title     string               `json:"title,omitempty"`
	Action    string               `json:"action,omitempty"`
	Disabled  bool                 `json:"disabled,omitempty"`
	Separator bool                 `json:"separator,omitempty"`
	Children  []statusItemMenuItem `json:"children,omitempty"`
}

func buildStatusItemMenuItems(profiles []accountpool.ProfileSnapshot, loadErr error) []statusItemMenuItem {
	items := []statusItemMenuItem{
		{Title: "显示主窗口", Action: statusActionShowWindow},
		{Title: "刷新全部额度", Action: statusActionRefreshUsage},
	}

	if loadErr != nil {
		items = append(items,
			statusSeparatorItem(),
			statusDisabledItem("读取槽位失败"),
			statusDisabledItem(loadErr.Error()),
			statusSeparatorItem(),
			statusItemMenuItem{Title: "退出客户端", Action: statusActionQuit},
		)
		return items
	}

	items = append(items,
		statusItemMenuItem{Title: "立即执行自动切换检查", Action: statusActionRunAutoSwitch},
		statusSeparatorItem(),
		statusDisabledItem("当前运行槽位："+currentSlotMenuLabel(profiles)),
		buildStatusQuotaSubmenu(managedProfiles(profiles)),
		buildStatusProbeSubmenu(managedProfiles(profiles)),
		statusSeparatorItem(),
		statusItemMenuItem{Title: "退出客户端", Action: statusActionQuit},
	)
	return items
}

func buildStatusQuotaSubmenu(profiles []accountpool.ProfileSnapshot) statusItemMenuItem {
	children := make([]statusItemMenuItem, 0, len(profiles))
	for _, profile := range profiles {
		children = append(children, statusItemMenuItem{
			Title:  profileQuotaMenuLabel(profile),
			Action: statusActionFocusPrefix + url.QueryEscape(profile.Name),
		})
	}
	if len(children) == 0 {
		children = append(children, statusDisabledItem("还没有托管槽位"))
	}
	return statusItemMenuItem{
		Title:    "查看额度",
		Children: children,
	}
}

func buildStatusProbeSubmenu(profiles []accountpool.ProfileSnapshot) statusItemMenuItem {
	children := make([]statusItemMenuItem, 0, len(profiles))
	for _, profile := range profiles {
		if !profile.HasCredential {
			continue
		}
		children = append(children, statusItemMenuItem{
			Title:  profile.Name,
			Action: statusActionProbePrefix + url.QueryEscape(profile.Name),
		})
	}
	if len(children) == 0 {
		children = append(children, statusDisabledItem("还没有已登录槽位"))
	}
	return statusItemMenuItem{
		Title:    "快速检查",
		Children: children,
	}
}

func statusSeparatorItem() statusItemMenuItem {
	return statusItemMenuItem{Separator: true}
}

func statusDisabledItem(title string) statusItemMenuItem {
	return statusItemMenuItem{Title: title, Disabled: true}
}

func parseStatusAction(action string) (kind string, profileName string) {
	action = strings.TrimSpace(action)
	switch {
	case action == statusActionShowWindow:
		return statusActionShowWindow, ""
	case action == statusActionRefreshUsage:
		return statusActionRefreshUsage, ""
	case action == statusActionRunAutoSwitch:
		return statusActionRunAutoSwitch, ""
	case action == statusActionQuit:
		return statusActionQuit, ""
	case strings.HasPrefix(action, statusActionFocusPrefix):
		name, err := url.QueryUnescape(strings.TrimPrefix(action, statusActionFocusPrefix))
		if err != nil {
			return "", ""
		}
		return statusActionFocusPrefix, name
	case strings.HasPrefix(action, statusActionProbePrefix):
		name, err := url.QueryUnescape(strings.TrimPrefix(action, statusActionProbePrefix))
		if err != nil {
			return "", ""
		}
		return statusActionProbePrefix, name
	default:
		return "", ""
	}
}
