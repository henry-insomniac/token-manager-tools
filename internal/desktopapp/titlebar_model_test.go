package desktopapp

import (
	"testing"

	"github.com/henry-insomniac/token-manager-tools/internal/accountpool"
)

func TestTitlebarSummary(t *testing.T) {
	tests := []struct {
		name     string
		profiles []accountpool.ProfileSnapshot
		want     string
	}{
		{
			name: "managed active profile",
			profiles: []accountpool.ProfileSnapshot{
				{Name: accountpool.DefaultProfileName, IsDefault: true},
				{
					Name:          "acct-d",
					IsActive:      true,
					HasCredential: true,
					CachedProbe: &accountpool.CachedProbeSnapshot{
						Usage: accountpool.UsageSnapshot{
							FiveHour: &accountpool.UsageWindow{LeftPercent: 75},
							Week:     &accountpool.UsageWindow{LeftPercent: 40},
						},
					},
				},
			},
			want: "当前：acct-d · 5h 75% · 周 40%",
		},
		{
			name: "default only",
			profiles: []accountpool.ProfileSnapshot{
				{Name: accountpool.DefaultProfileName, IsDefault: true, IsActive: true},
			},
			want: "当前：系统默认资料",
		},
		{
			name: "none active",
			profiles: []accountpool.ProfileSnapshot{
				{Name: accountpool.DefaultProfileName, IsDefault: true},
			},
			want: "当前：未切到托管槽位",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := titlebarSummary(tt.profiles); got != tt.want {
				t.Fatalf("titlebarSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}
