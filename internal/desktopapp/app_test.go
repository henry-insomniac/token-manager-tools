package desktopapp

import (
	"net"
	"testing"
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
