package main

import (
	"strings"
	"testing"
)

func TestParseManualLoginCodeFromCallbackURL(t *testing.T) {
	code, err := parseManualLoginCode("http://localhost:1455/auth/callback?code=auth-code&state=state-a", "state-a")
	if err != nil {
		t.Fatalf("parseManualLoginCode: %v", err)
	}
	if code != "auth-code" {
		t.Fatalf("unexpected code: %s", code)
	}
}

func TestParseManualLoginCodeRejectsStateMismatch(t *testing.T) {
	_, err := parseManualLoginCode("http://localhost:1455/auth/callback?code=auth-code&state=wrong", "state-a")
	if err == nil {
		t.Fatalf("expected state mismatch")
	}
	if !strings.Contains(err.Error(), "登录回调校验失败") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseManualLoginCodeSupportsQueryAndRawCode(t *testing.T) {
	code, err := parseManualLoginCode("?code=query-code&state=state-a", "state-a")
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if code != "query-code" {
		t.Fatalf("unexpected query code: %s", code)
	}

	code, err = parseManualLoginCode("raw-code", "state-a")
	if err != nil {
		t.Fatalf("parse raw code: %v", err)
	}
	if code != "raw-code" {
		t.Fatalf("unexpected raw code: %s", code)
	}
}

func TestParseServeArgs(t *testing.T) {
	addr, openBrowser, err := parseServeArgs([]string{"1666", "--no-open"})
	if err != nil {
		t.Fatalf("parseServeArgs: %v", err)
	}
	if addr != "127.0.0.1:1666" || openBrowser {
		t.Fatalf("unexpected serve args: addr=%s open=%v", addr, openBrowser)
	}
}

func TestParseServeArgsRejectsRemoteHostByDefault(t *testing.T) {
	_, _, err := parseServeArgs([]string{"0.0.0.0:1455"})
	if err == nil {
		t.Fatalf("expected remote listen host to be rejected")
	}
	if !strings.Contains(err.Error(), "serve 默认只允许监听") {
		t.Fatalf("unexpected error: %v", err)
	}
}
