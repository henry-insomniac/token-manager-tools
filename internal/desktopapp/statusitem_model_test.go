package desktopapp

import (
	"testing"

	"github.com/henry-insomniac/token-manager-tools/internal/accountpool"
)

func TestBuildStatusItemMenuItems(t *testing.T) {
	profiles := []accountpool.ProfileSnapshot{
		{Name: accountpool.DefaultProfileName, IsDefault: true},
		{
			Name:          "acct-a",
			HasCredential: true,
			IsActive:      true,
			CachedProbe: &accountpool.CachedProbeSnapshot{
				Usage: accountpool.UsageSnapshot{
					FiveHour: &accountpool.UsageWindow{LeftPercent: 75},
					Week:     &accountpool.UsageWindow{LeftPercent: 40},
				},
			},
		},
	}

	items := buildStatusItemMenuItems(profiles, nil)
	if len(items) < 6 {
		t.Fatalf("expected status item menu entries, got %d", len(items))
	}
	if items[0].Action != statusActionShowWindow {
		t.Fatalf("unexpected first action: %#v", items[0])
	}

	var quotaMenu statusItemMenuItem
	foundQuota := false
	for _, item := range items {
		if item.Title == "查看额度" {
			quotaMenu = item
			foundQuota = true
			break
		}
	}
	if !foundQuota {
		t.Fatalf("missing quota submenu: %#v", items)
	}
	if len(quotaMenu.Children) != 1 {
		t.Fatalf("unexpected quota submenu items: %#v", quotaMenu.Children)
	}
	if quotaMenu.Children[0].Action == "" {
		t.Fatalf("quota submenu should focus a profile: %#v", quotaMenu.Children[0])
	}
}

func TestParseStatusAction(t *testing.T) {
	tests := []struct {
		name        string
		action      string
		wantKind    string
		wantProfile string
	}{
		{name: "show", action: statusActionShowWindow, wantKind: statusActionShowWindow},
		{name: "refresh", action: statusActionRefreshUsage, wantKind: statusActionRefreshUsage},
		{name: "focus", action: statusActionFocusPrefix + "acct-d", wantKind: statusActionFocusPrefix, wantProfile: "acct-d"},
		{name: "probe", action: statusActionProbePrefix + "acct-d", wantKind: statusActionProbePrefix, wantProfile: "acct-d"},
		{name: "invalid", action: "unknown", wantKind: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotKind, gotProfile := parseStatusAction(tt.action)
			if gotKind != tt.wantKind || gotProfile != tt.wantProfile {
				t.Fatalf("parseStatusAction(%q) = (%q, %q), want (%q, %q)", tt.action, gotKind, gotProfile, tt.wantKind, tt.wantProfile)
			}
		})
	}
}
