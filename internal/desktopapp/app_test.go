package desktopapp

import (
	"net"
	"strings"
	"syscall"
	"testing"

	"github.com/henry-insomniac/token-manager-tools/internal/accountpool"
)

func TestCallbackAddrFromRedirectURL(t *testing.T) {
	got := callbackAddrFromRedirectURL("http://localhost:18765/auth/callback")
	if got != "127.0.0.1:18765" {
		t.Fatalf("unexpected callback addr: %s", got)
	}

	if callbackAddrFromRedirectURL("https://chatgpt.com/api/auth/session") != "" {
		t.Fatalf("non-loopback redirect url should not produce callback addr")
	}
}

func TestListenLoopbackAddrsWithDynamicPort(t *testing.T) {
	listeners, err := listenLoopbackAddrs("127.0.0.1:0")
	if err != nil {
		t.Fatalf("listenLoopbackAddrs: %v", err)
	}
	defer func() {
		for _, listener := range listeners {
			_ = listener.Close()
		}
	}()

	if len(listeners) == 0 {
		t.Fatalf("expected at least one listener")
	}

	_, firstPort, err := net.SplitHostPort(listeners[0].Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort first: %v", err)
	}
	if firstPort == "0" || firstPort == "" {
		t.Fatalf("dynamic port was not resolved: %q", firstPort)
	}

	for _, listener := range listeners[1:] {
		_, port, err := net.SplitHostPort(listener.Addr().String())
		if err != nil {
			t.Fatalf("SplitHostPort extra: %v", err)
		}
		if port != firstPort {
			t.Fatalf("expected listeners to share one port, got %s and %s", firstPort, port)
		}
	}
}

func TestNewDesktopAppUsesFixedLoopbackCallback(t *testing.T) {
	app, err := New(accountpool.Config{HomeDir: t.TempDir()})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if app.callbackAddr != "127.0.0.1:1455" {
		t.Fatalf("callbackAddr = %q, want %q", app.callbackAddr, "127.0.0.1:1455")
	}
	if got := app.service.OAuthRedirectURL(); got != "http://localhost:1455/auth/callback" {
		t.Fatalf("OAuthRedirectURL() = %q, want %q", got, "http://localhost:1455/auth/callback")
	}
}

func TestNormalizeCallbackListenErrorForOccupiedPort(t *testing.T) {
	err := normalizeCallbackListenError("127.0.0.1:1455", &net.OpError{Err: syscall.EADDRINUSE})
	if err == nil || !strings.Contains(err.Error(), "1455") || !strings.Contains(err.Error(), "已被占用") {
		t.Fatalf("unexpected error: %v", err)
	}
}
