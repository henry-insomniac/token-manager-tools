package desktopapp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/henry-insomniac/token-manager-tools/internal/accountpool"
	"github.com/henry-insomniac/token-manager-tools/internal/appservice"
	"github.com/henry-insomniac/token-manager-tools/internal/desktopruntime"
	"github.com/henry-insomniac/token-manager-tools/internal/logincallback"
	localserver "github.com/henry-insomniac/token-manager-tools/internal/server"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	LoginResultEvent    = "token-manager-login-result"
	DesktopActionEvent  = "token-manager-desktop-action"
	FocusProfileEvent   = "token-manager-focus-profile"
	defaultCallbackAddr = "127.0.0.1:0"
)

type App struct {
	service        *appservice.Service
	runtimeManager *desktopruntime.Manager
	bindings       *Bindings

	ctx              context.Context
	callbackAddr     string
	redirectURLFixed bool
	callbackOnce     sync.Once
	callback         *http.Server
	listeners        []net.Listener
}

func New(config accountpool.Config) (*App, error) {
	executablePath, err := desktopruntime.ExecutablePath()
	if err != nil {
		return nil, err
	}
	explicitRedirectURL := firstNonEmpty(config.OAuthRedirectURL, os.Getenv("TOKEN_MANAGER_OAUTH_REDIRECT_URL"))
	callbackAddr := firstNonEmpty(
		os.Getenv("TOKEN_MANAGER_DESKTOP_CALLBACK_ADDR"),
		callbackAddrFromRedirectURL(explicitRedirectURL),
		defaultCallbackAddr,
	)
	if strings.TrimSpace(explicitRedirectURL) == "" && !hasDynamicPort(callbackAddr) {
		config.OAuthRedirectURL = oauthRedirectURLForAddr(callbackAddr)
	}
	pool, err := accountpool.New(config)
	if err != nil {
		return nil, err
	}
	return &App{
		service:          appservice.New(pool),
		runtimeManager:   desktopruntime.NewManager(executablePath),
		bindings:         nil,
		callbackAddr:     callbackAddr,
		redirectURLFixed: strings.TrimSpace(explicitRedirectURL) != "",
	}, nil
}

func (app *App) Startup(ctx context.Context) {
	app.ctx = ctx
	if app.bindings == nil {
		app.bindings = NewBindings(app.service, app.runtimeManager, app)
	}
	if err := app.startCallbackServer(); err != nil {
		runtime.LogErrorf(ctx, "start callback server failed: %v", err)
	}
	app.RefreshApplicationMenu()
}

func (app *App) DomReady(ctx context.Context) {
	app.ctx = ctx
	go func() {
		time.Sleep(250 * time.Millisecond)
		app.installStatusItem()
		app.installTitlebarAccessory()
		app.RefreshDesktopMenus()
		app.RefreshTitlebarAccessory()
		time.Sleep(1200 * time.Millisecond)
		app.RefreshStatusItem()
		app.RefreshTitlebarAccessory()
		app.logStatusItemState("dom-ready-delayed")
	}()
}

func (app *App) Shutdown(ctx context.Context) {
	app.removeStatusItem()
	app.removeTitlebarAccessory()
	if app.callback != nil {
		_ = app.callback.Shutdown(context.Background())
	}
	for _, listener := range app.listeners {
		_ = listener.Close()
	}
}

func (app *App) AssetsHandler() http.Handler {
	return localserver.NewHandler(app.service)
}

func (app *App) Bindings() []interface{} {
	if app.bindings == nil {
		app.bindings = NewBindings(app.service, app.runtimeManager, app)
	}
	return []interface{}{app.bindings}
}

func (app *App) ShowWindow() {
	if app.ctx == nil {
		return
	}
	runtime.Show(app.ctx)
	runtime.WindowUnminimise(app.ctx)
	runtime.WindowShow(app.ctx)
}

func (app *App) FocusProfile(name string) {
	if app.ctx == nil {
		return
	}
	app.ShowWindow()
	runtime.EventsEmit(app.ctx, FocusProfileEvent, map[string]string{
		"profileName": name,
		"at":          time.Now().Format(time.RFC3339Nano),
	})
}

func (app *App) startCallbackServer() error {
	var startErr error
	app.callbackOnce.Do(func() {
		listeners, err := listenLoopbackAddrs(app.callbackAddr)
		if err != nil {
			startErr = err
			return
		}
		app.listeners = listeners
		if !app.redirectURLFixed && len(listeners) > 0 {
			app.service.SetOAuthRedirectURL(oauthRedirectURLForAddr(listeners[0].Addr().String()))
		}
		server := &http.Server{
			Handler:           app.callbackHandler(),
			ReadHeaderTimeout: 10 * time.Second,
		}
		app.callback = server
		for _, listener := range listeners {
			go func(listener net.Listener) {
				if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) && app.ctx != nil {
					runtime.LogErrorf(app.ctx, "desktop callback serve failed: %v", err)
				}
			}(listener)
		}
	})
	return startErr
}

func (app *App) callbackHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", app.handleOAuthCallback)
	return mux
}

func (app *App) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	result, err := app.service.CompleteOAuthCallback(
		r.URL.Query().Get("state"),
		r.URL.Query().Get("code"),
		r.URL.Query().Get("error"),
	)
	payload := map[string]string{
		"profileName": result.ProfileName,
		"at":          time.Now().Format(time.RFC3339Nano),
	}
	if err != nil {
		payload["status"] = "error"
		payload["title"] = "登录失败"
		payload["body"] = err.Error()
		if app.ctx != nil {
			runtime.EventsEmit(app.ctx, LoginResultEvent, payload)
		}
		logincallback.WriteHTML(w, logincallback.PageData{
			Title:         "登录失败",
			Body:          err.Error(),
			ProfileName:   result.ProfileName,
			Status:        "error",
			RedirectDelay: 1400,
		})
		return
	}
	payload["status"] = "success"
	payload["title"] = "登录成功"
	payload["body"] = result.Message
	if app.ctx != nil {
		runtime.EventsEmit(app.ctx, LoginResultEvent, payload)
		app.RefreshDesktopMenus()
		app.ShowWindow()
	}
	logincallback.WriteHTML(w, logincallback.PageData{
		Title:         "登录成功",
		Body:          result.Message + " 可以返回客户端。",
		ProfileName:   result.ProfileName,
		Status:        "success",
		RedirectDelay: 900,
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func oauthRedirectURLForAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://localhost:1455/auth/callback"
	}
	switch strings.Trim(host, "[]") {
	case "", "localhost", "127.0.0.1", "::1":
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port) + "/auth/callback"
}

func callbackAddrFromRedirectURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil {
		return ""
	}
	if !strings.EqualFold(parsed.Scheme, "http") {
		return ""
	}
	host, port, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		return ""
	}
	host = strings.Trim(host, "[]")
	switch host {
	case "", "localhost", "127.0.0.1", "::1":
		return net.JoinHostPort("127.0.0.1", port)
	default:
		return ""
	}
}

func hasDynamicPort(addr string) bool {
	_, port, err := net.SplitHostPort(addr)
	return err == nil && port == "0"
}

func listenLoopbackAddrs(addr string) ([]net.Listener, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	host = strings.Trim(host, "[]")
	hosts := []string{"127.0.0.1", "::1"}
	if host == "::1" {
		hosts[0], hosts[1] = hosts[1], hosts[0]
	}
	listeners := make([]net.Listener, 0, len(hosts))
	seen := map[string]struct{}{}
	for index, loopbackHost := range hosts {
		candidate := net.JoinHostPort(loopbackHost, port)
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		listener, err := net.Listen("tcp", candidate)
		if err != nil {
			if index == 0 {
				return nil, err
			}
			continue
		}
		listeners = append(listeners, listener)
		if port == "0" {
			_, actualPort, err := net.SplitHostPort(listener.Addr().String())
			if err != nil {
				return nil, err
			}
			port = actualPort
		}
	}
	if len(listeners) == 0 {
		return nil, fmt.Errorf("监听地址无效: %s", addr)
	}
	return listeners, nil
}
