package accountpool

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"
)

const (
	autoSwitchPollIntervalMin   = 90 * time.Second
	autoSwitchPollIntervalMax   = 180 * time.Second
	autoSwitchMinSwitchInterval = 3 * time.Minute
	autoSwitchEventLimit        = 8
)

type autoSwitchCandidate struct {
	name       string
	healthy    bool
	reason     string
	fiveHour   int
	week       int
	probeError error
}

func (pool *AccountPool) AutoSwitchStatus() (AutoSwitchStatus, error) {
	settings := pool.loadRuntimeSettings()
	return normalizeAutoSwitchStatus(settings.AutoSwitch), nil
}

func (pool *AccountPool) SetAutoSwitchEnabled(enabled bool) (AutoSwitchRunResult, error) {
	pool.autoSwitchMu.Lock()
	defer pool.autoSwitchMu.Unlock()

	settings := pool.loadRuntimeSettings()
	status := normalizeAutoSwitchStatus(settings.AutoSwitch)
	status.Enabled = enabled
	now := pool.now().UTC().Format(time.RFC3339)
	if enabled {
		status.LastMessage = "自动切换已开启"
		status = appendAutoSwitchEvent(status, AutoSwitchEvent{
			At:      now,
			Type:    "enabled",
			Message: "已开启自动切换",
		})
		settings.AutoSwitch = status
		if err := pool.saveRuntimeSettings(settings); err != nil {
			return AutoSwitchRunResult{}, err
		}
		return pool.runAutoSwitchNowLocked()
	}
	status.LastMessage = "自动切换已关闭"
	status = appendAutoSwitchEvent(status, AutoSwitchEvent{
		At:      now,
		Type:    "disabled",
		Message: "已关闭自动切换",
	})
	settings.AutoSwitch = status
	if err := pool.saveRuntimeSettings(settings); err != nil {
		return AutoSwitchRunResult{}, err
	}
	return AutoSwitchRunResult{Switched: false, Status: status}, nil
}

func (pool *AccountPool) RunAutoSwitchNow() (AutoSwitchRunResult, error) {
	pool.autoSwitchMu.Lock()
	defer pool.autoSwitchMu.Unlock()
	return pool.runAutoSwitchNowLocked()
}

func (pool *AccountPool) runAutoSwitchNowLocked() (AutoSwitchRunResult, error) {
	settings := pool.loadRuntimeSettings()
	status := normalizeAutoSwitchStatus(settings.AutoSwitch)
	now := pool.now().UTC()
	status.LastCheckedAt = ptr(now.Format(time.RFC3339))

	if !status.Enabled {
		status.LastMessage = "自动切换未开启"
		settings.AutoSwitch = status
		if err := pool.saveRuntimeSettings(settings); err != nil {
			return AutoSwitchRunResult{}, err
		}
		return AutoSwitchRunResult{Switched: false, Status: status}, nil
	}

	profiles, err := pool.ListProfiles()
	if err != nil {
		status.LastMessage = "读取账号池失败，暂不自动切换"
		settings.AutoSwitch = status
		if saveErr := pool.saveRuntimeSettings(settings); saveErr != nil {
			return AutoSwitchRunResult{}, saveErr
		}
		return AutoSwitchRunResult{}, err
	}

	active := currentManagedProfile(profiles)
	fromName := ""
	triggerReason := "当前没有运行账号"
	if active != nil {
		fromName = active.Name
		switch {
		case !active.HasCredential:
			triggerReason = firstNonEmpty(active.StatusReason, "当前账号未登录")
		default:
			result, probeErr := pool.ProbeProfile(active.Name)
			if probeErr != nil {
				status.LastMessage = fmt.Sprintf("%s 检查失败，暂不自动切换", active.Name)
				settings.AutoSwitch = status
				if err := pool.saveRuntimeSettings(settings); err != nil {
					return AutoSwitchRunResult{}, err
				}
				return AutoSwitchRunResult{Switched: false, Status: status}, nil
			}
			if result.Status == "healthy" {
				status.LastMessage = fmt.Sprintf("%s 额度可用，无需切换", active.Name)
				settings.AutoSwitch = status
				if err := pool.saveRuntimeSettings(settings); err != nil {
					return AutoSwitchRunResult{}, err
				}
				return AutoSwitchRunResult{Switched: false, Status: status}, nil
			}
			triggerReason = firstNonEmpty(result.Reason, active.StatusReason, "当前账号不可用")
		}
	}

	candidates := make([]autoSwitchCandidate, 0, len(profiles))
	for _, profile := range profiles {
		if !isEligibleAutoSwitchCandidate(profile, fromName) {
			continue
		}
		result, probeErr := pool.ProbeProfile(profile.Name)
		if probeErr != nil {
			candidates = append(candidates, autoSwitchCandidate{
				name:       profile.Name,
				probeError: probeErr,
			})
			continue
		}
		if result.Status != "healthy" {
			candidates = append(candidates, autoSwitchCandidate{
				name:     profile.Name,
				healthy:  false,
				reason:   result.Reason,
				fiveHour: usageLeftPercent(result.Usage.FiveHour),
				week:     usageLeftPercent(result.Usage.Week),
			})
			continue
		}
		candidates = append(candidates, autoSwitchCandidate{
			name:     profile.Name,
			healthy:  true,
			reason:   result.Reason,
			fiveHour: usageLeftPercent(result.Usage.FiveHour),
			week:     usageLeftPercent(result.Usage.Week),
		})
	}

	best, found := bestAutoSwitchCandidate(candidates)
	if !found {
		status.LastMessage = "没有可自动切换的可用账号"
		settings.AutoSwitch = status
		if err := pool.saveRuntimeSettings(settings); err != nil {
			return AutoSwitchRunResult{}, err
		}
		return AutoSwitchRunResult{Switched: false, Status: status}, nil
	}

	if !canAutoSwitchNow(status, now) {
		status.LastMessage = fmt.Sprintf("已找到 %s，但距离上次自动切换过近，暂不重复切换", best.name)
		settings.AutoSwitch = status
		if err := pool.saveRuntimeSettings(settings); err != nil {
			return AutoSwitchRunResult{}, err
		}
		return AutoSwitchRunResult{Switched: false, Status: status}, nil
	}

	if err := pool.ActivateProfile(best.name); err != nil {
		status.LastMessage = fmt.Sprintf("自动切换到 %s 失败", best.name)
		settings.AutoSwitch = status
		if saveErr := pool.saveRuntimeSettings(settings); saveErr != nil {
			return AutoSwitchRunResult{}, saveErr
		}
		return AutoSwitchRunResult{}, err
	}

	timestamp := now.Format(time.RFC3339)
	status.LastSwitchedAt = ptr(timestamp)
	status.LastFrom = nil
	if fromName != "" {
		status.LastFrom = ptr(fromName)
	}
	status.LastTo = ptr(best.name)
	status.LastMessage = fmt.Sprintf("已自动切换到 %s", best.name)
	event := AutoSwitchEvent{
		At:      timestamp,
		Type:    "switch",
		Message: status.LastMessage,
		To:      ptr(best.name),
		Reason:  ptr(triggerReason),
	}
	if fromName != "" {
		event.From = ptr(fromName)
	}
	status = appendAutoSwitchEvent(status, event)
	settings.AutoSwitch = status
	if err := pool.saveRuntimeSettings(settings); err != nil {
		return AutoSwitchRunResult{}, err
	}
	return AutoSwitchRunResult{Switched: true, Status: status}, nil
}

func NextAutoSwitchPollInterval() time.Duration {
	if autoSwitchPollIntervalMax <= autoSwitchPollIntervalMin {
		return autoSwitchPollIntervalMin
	}
	span := autoSwitchPollIntervalMax - autoSwitchPollIntervalMin
	return autoSwitchPollIntervalMin + time.Duration(rand.Int63n(int64(span)+1))
}

func AutoSwitchPollIntervalRange() (time.Duration, time.Duration) {
	return autoSwitchPollIntervalMin, autoSwitchPollIntervalMax
}

func normalizeAutoSwitchStatus(status AutoSwitchStatus) AutoSwitchStatus {
	status.PollIntervalMinSeconds = int(autoSwitchPollIntervalMin.Seconds())
	status.PollIntervalMaxSeconds = int(autoSwitchPollIntervalMax.Seconds())
	status.MinSwitchIntervalSeconds = int(autoSwitchMinSwitchInterval.Seconds())
	if status.Events == nil {
		status.Events = []AutoSwitchEvent{}
	}
	if len(status.Events) > autoSwitchEventLimit {
		status.Events = append([]AutoSwitchEvent(nil), status.Events[:autoSwitchEventLimit]...)
	}
	return status
}

func appendAutoSwitchEvent(status AutoSwitchStatus, event AutoSwitchEvent) AutoSwitchStatus {
	events := []AutoSwitchEvent{event}
	events = append(events, status.Events...)
	if len(events) > autoSwitchEventLimit {
		events = events[:autoSwitchEventLimit]
	}
	status.Events = events
	return normalizeAutoSwitchStatus(status)
}

func canAutoSwitchNow(status AutoSwitchStatus, now time.Time) bool {
	if status.LastSwitchedAt == nil || strings.TrimSpace(*status.LastSwitchedAt) == "" {
		return true
	}
	lastSwitched, err := time.Parse(time.RFC3339, *status.LastSwitchedAt)
	if err != nil {
		return true
	}
	return now.Sub(lastSwitched) >= autoSwitchMinSwitchInterval
}

func currentManagedProfile(profiles []ProfileSnapshot) *ProfileSnapshot {
	for _, profile := range profiles {
		if profile.IsDefault {
			continue
		}
		if profile.IsActive {
			copy := profile
			return &copy
		}
	}
	return nil
}

func isEligibleAutoSwitchCandidate(profile ProfileSnapshot, currentName string) bool {
	if profile.IsDefault || !profile.HasCredential {
		return false
	}
	return profile.Name != currentName
}

func bestAutoSwitchCandidate(candidates []autoSwitchCandidate) (*autoSwitchCandidate, bool) {
	healthy := make([]autoSwitchCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.probeError != nil {
			continue
		}
		if !candidate.healthy {
			continue
		}
		healthy = append(healthy, candidate)
	}
	if len(healthy) == 0 {
		return nil, false
	}
	sort.Slice(healthy, func(i, j int) bool {
		if healthy[i].fiveHour != healthy[j].fiveHour {
			return healthy[i].fiveHour > healthy[j].fiveHour
		}
		if healthy[i].week != healthy[j].week {
			return healthy[i].week > healthy[j].week
		}
		return healthy[i].name < healthy[j].name
	})
	return &healthy[0], true
}

func usageLeftPercent(window *UsageWindow) int {
	if window == nil {
		return -1
	}
	return window.LeftPercent
}
