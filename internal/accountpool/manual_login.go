package accountpool

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

type ManualLoginInput struct {
	Code  string
	State string
}

func ParseManualLoginInput(rawInput string) (ManualLoginInput, error) {
	input := strings.TrimSpace(rawInput)
	if input == "" {
		return ManualLoginInput{}, errors.New("未输入登录回调地址或 code")
	}

	if parsed, err := url.Parse(input); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return parseManualLoginQuery(parsed.Query())
	}
	if strings.Contains(input, "code=") {
		query := strings.TrimPrefix(input, "?")
		if parsedQuery, err := url.ParseQuery(query); err == nil && parsedQuery.Get("code") != "" {
			return parseManualLoginQuery(parsedQuery)
		}
	}
	return ManualLoginInput{Code: input}, nil
}

func ParseManualLoginCode(rawInput, expectedState string) (string, error) {
	parsed, err := ParseManualLoginInput(rawInput)
	if err != nil {
		return "", err
	}
	if state := strings.TrimSpace(parsed.State); state != "" && state != strings.TrimSpace(expectedState) {
		return "", errors.New("登录回调校验失败")
	}
	if strings.TrimSpace(parsed.Code) == "" {
		return "", errors.New("登录回调缺少 code")
	}
	return parsed.Code, nil
}

func parseManualLoginQuery(query url.Values) (ManualLoginInput, error) {
	if authErr := strings.TrimSpace(query.Get("error")); authErr != "" {
		return ManualLoginInput{}, fmt.Errorf("登录失败: %s", authErr)
	}
	code := strings.TrimSpace(query.Get("code"))
	if code == "" {
		return ManualLoginInput{}, errors.New("登录回调缺少 code")
	}
	return ManualLoginInput{
		Code:  code,
		State: strings.TrimSpace(query.Get("state")),
	}, nil
}
