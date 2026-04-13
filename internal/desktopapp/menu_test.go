package desktopapp

import (
	"testing"

	"github.com/henry-insomniac/token-manager-tools/internal/accountpool"
	"github.com/henry-insomniac/token-manager-tools/internal/appservice"
)

func TestProfileUsageSummary(t *testing.T) {
	tests := []struct {
		name    string
		profile accountpool.ProfileSnapshot
		want    string
	}{
		{
			name:    "not logged in",
			profile: accountpool.ProfileSnapshot{Name: "acct-a"},
			want:    "未登录",
		},
		{
			name: "not probed yet",
			profile: accountpool.ProfileSnapshot{
				Name:          "acct-a",
				HasCredential: true,
			},
			want: "未检查",
		},
		{
			name: "usage windows",
			profile: accountpool.ProfileSnapshot{
				Name:          "acct-a",
				HasCredential: true,
				CachedProbe: &accountpool.CachedProbeSnapshot{
					Usage: accountpool.UsageSnapshot{
						FiveHour: &accountpool.UsageWindow{LeftPercent: 75},
						Week:     &accountpool.UsageWindow{LeftPercent: 40},
					},
				},
			},
			want: "5h 75% · 周 40%",
		},
		{
			name: "reason fallback",
			profile: accountpool.ProfileSnapshot{
				Name:          "acct-a",
				HasCredential: true,
				CachedProbe: &accountpool.CachedProbeSnapshot{
					Status: "warn",
					Reason: "接近上限",
				},
			},
			want: "接近上限",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := profileUsageSummary(tt.profile); got != tt.want {
				t.Fatalf("profileUsageSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCurrentSlotMenuLabel(t *testing.T) {
	profiles := []accountpool.ProfileSnapshot{
		{Name: accountpool.DefaultProfileName, IsDefault: true},
		{Name: "acct-a"},
		{Name: "acct-b", IsActive: true},
	}
	if got := currentSlotMenuLabel(profiles); got != "acct-b" {
		t.Fatalf("currentSlotMenuLabel() = %q, want %q", got, "acct-b")
	}

	profiles = []accountpool.ProfileSnapshot{
		{Name: accountpool.DefaultProfileName, IsDefault: true, IsActive: true},
	}
	if got := currentSlotMenuLabel(profiles); got != "系统默认资料" {
		t.Fatalf("currentSlotMenuLabel() = %q, want %q", got, "系统默认资料")
	}
}

func TestUsageRefreshMessage(t *testing.T) {
	tests := []struct {
		name   string
		result map[string]int
		want   string
	}{
		{
			name: "none",
			want: "没有可刷新的已登录槽位。",
		},
		{
			name: "success only",
			result: map[string]int{
				"refreshed": 2,
			},
			want: "已刷新 2 个槽位。",
		},
		{
			name: "failed only",
			result: map[string]int{
				"failed": 1,
			},
			want: "没有刷新成功，1 个槽位失败。",
		},
		{
			name: "mixed",
			result: map[string]int{
				"refreshed": 2,
				"failed":    1,
			},
			want: "已刷新 2 个槽位，1 个失败。",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refreshed := make([]string, tt.result["refreshed"])
			failed := make(map[string]string, tt.result["failed"])
			for i := 0; i < tt.result["failed"]; i++ {
				failed["acct-"+string(rune('a'+i))] = "fail"
			}
			got := usageRefreshMessage(appservice.UsageRefreshResult{
				Refreshed: refreshed,
				Failed:    failed,
			})
			if got != tt.want {
				t.Fatalf("usageRefreshMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}
