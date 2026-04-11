package accountpool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

type usagePayload struct {
	PlanType  string `json:"plan_type"`
	RateLimit struct {
		PrimaryWindow *struct {
			UsedPercent *float64 `json:"used_percent"`
			ResetAt     *int64   `json:"reset_at"`
		} `json:"primary_window"`
		SecondaryWindow *struct {
			UsedPercent *float64 `json:"used_percent"`
			ResetAt     *int64   `json:"reset_at"`
		} `json:"secondary_window"`
	} `json:"rate_limit"`
}

func (pool *AccountPool) ProbeProfile(rawName string) (ProbeResult, error) {
	name, err := normalizeProfileName(rawName, false)
	if err != nil {
		return ProbeResult{}, err
	}
	tokens, err := pool.tokensForProfile(name)
	if err != nil {
		return ProbeResult{}, err
	}
	usage, err := pool.FetchUsage(tokens)
	if shouldRefreshAfterUsageError(err) {
		refreshed, refreshErr := pool.RefreshTokens(tokens.Refresh)
		if refreshErr != nil {
			return ProbeResult{}, fmt.Errorf("认证已失效，刷新失败，请重新登录: %w", refreshErr)
		}
		if refreshed.IDToken == "" {
			refreshed.IDToken = tokens.IDToken
		}
		if err := pool.PersistTokens(name, refreshed); err != nil {
			return ProbeResult{}, err
		}
		if err := pool.syncDefaultIfActive(name); err != nil {
			return ProbeResult{}, err
		}
		tokens = refreshed
		usage, err = pool.FetchUsage(tokens)
	}
	if err != nil {
		return ProbeResult{}, err
	}
	status, reason := classifyUsage(usage)
	return ProbeResult{
		ProfileName:  name,
		Status:       status,
		Reason:       reason,
		Usage:        usage,
		AccountID:    tokens.AccountID,
		AccountEmail: tokens.Email,
	}, nil
}

func (pool *AccountPool) FetchUsage(tokens OAuthTokens) (UsageSnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pool.usageURL, nil)
	if err != nil {
		return UsageSnapshot{}, err
	}
	req.Header.Set("Authorization", "Bearer "+tokens.Access)
	req.Header.Set("User-Agent", "CodexBar")
	req.Header.Set("Accept", "application/json")
	if strings.TrimSpace(tokens.AccountID) != "" {
		req.Header.Set("ChatGPT-Account-Id", tokens.AccountID)
	}
	resp, err := pool.httpClient.Do(req)
	if err != nil {
		return UsageSnapshot{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return UsageSnapshot{}, &remoteStatusError{
			Operation:  "usage",
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(body)),
		}
	}

	var payload usagePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return UsageSnapshot{}, err
	}

	var primaryUsed *float64
	var primaryReset *int64
	if payload.RateLimit.PrimaryWindow != nil {
		primaryUsed = payload.RateLimit.PrimaryWindow.UsedPercent
		primaryReset = payload.RateLimit.PrimaryWindow.ResetAt
	}
	var secondaryUsed *float64
	var secondaryReset *int64
	if payload.RateLimit.SecondaryWindow != nil {
		secondaryUsed = payload.RateLimit.SecondaryWindow.UsedPercent
		secondaryReset = payload.RateLimit.SecondaryWindow.ResetAt
	}

	return UsageSnapshot{
		Plan:     nullableString(payload.PlanType),
		FiveHour: pool.buildUsageWindow("5h", primaryUsed, primaryReset),
		Week:     pool.buildUsageWindow("week", secondaryUsed, secondaryReset),
	}, nil
}

func (pool *AccountPool) buildUsageWindow(label string, usedPercent *float64, resetAtSeconds *int64) *UsageWindow {
	if usedPercent == nil && resetAtSeconds == nil {
		return nil
	}
	used := clampPercentFloat(derefFloat64(usedPercent))
	var resetAt *string
	var resetInMs *int
	if resetAtSeconds != nil && *resetAtSeconds > 0 {
		resetTime := time.Unix(*resetAtSeconds, 0).UTC()
		resetAt = ptr(resetTime.Format(time.RFC3339))
		resetInMs = ptr(max(0, int(resetTime.Sub(pool.now()).Milliseconds())))
	}
	return &UsageWindow{
		Label:       label,
		UsedPercent: used,
		LeftPercent: 100 - used,
		ResetAt:     resetAt,
		ResetInMs:   resetInMs,
	}
}

func classifyUsage(usage UsageSnapshot) (string, string) {
	if usage.Week != nil && usage.Week.LeftPercent <= 0 {
		return "exhausted", "周额度已耗尽"
	}
	if usage.FiveHour != nil && usage.FiveHour.LeftPercent <= 0 {
		return "cooldown", "5 小时额度已耗尽"
	}
	if usage.FiveHour != nil || usage.Week != nil {
		return "healthy", "额度可用"
	}
	return "unknown", "未返回额度窗口"
}

func clampPercentFloat(value float64) int {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return int(math.Round(value))
}

func derefFloat64(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func nullableString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return ptr(value)
}

func max(left, right int) int {
	if left > right {
		return left
	}
	return right
}

type remoteStatusError struct {
	Operation  string
	StatusCode int
	Body       string
}

func (err *remoteStatusError) Error() string {
	if strings.TrimSpace(err.Body) == "" {
		return fmt.Sprintf("%s_failed %d", err.Operation, err.StatusCode)
	}
	return fmt.Sprintf("%s_failed %d %s", err.Operation, err.StatusCode, err.Body)
}

func shouldRefreshAfterUsageError(err error) bool {
	var statusErr *remoteStatusError
	if !errors.As(err, &statusErr) {
		return false
	}
	return statusErr.Operation == "usage" && (statusErr.StatusCode == http.StatusUnauthorized || statusErr.StatusCode == http.StatusForbidden)
}
