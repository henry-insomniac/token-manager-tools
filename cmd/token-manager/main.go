package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/henry-insomniac/token-manager-tools/internal/accountpool"
	"github.com/henry-insomniac/token-manager-tools/internal/platform"
	localserver "github.com/henry-insomniac/token-manager-tools/internal/server"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}
	switch args[0] {
	case "serve":
		return runServe(args[1:])
	case "start":
		return runStart(args[1:])
	case "stop":
		if len(args) != 1 {
			return fmt.Errorf("用法: token-manager stop")
		}
		return runStop()
	case "status":
		if len(args) != 1 {
			return fmt.Errorf("用法: token-manager status")
		}
		return runStatus()
	}

	pool, err := accountpool.New(accountpool.Config{})
	if err != nil {
		return err
	}

	switch args[0] {
	case "list":
		profiles, err := pool.ListProfiles()
		if err != nil {
			return err
		}
		for _, profile := range profiles {
			active := ""
			if profile.IsActive {
				active = " · 当前激活"
			}
			fmt.Printf("%s\t%s%s\t%s\n", profile.Name, profile.Status, active, profile.StatusReason)
		}
	case "create":
		if len(args) != 2 {
			return fmt.Errorf("用法: token-manager create <槽位名>")
		}
		profile, err := pool.CreateProfile(args[1])
		if err != nil {
			return err
		}
		fmt.Printf("已创建账号槽位: %s\n", profile.Name)
	case "activate":
		if len(args) != 2 {
			return fmt.Errorf("用法: token-manager activate <槽位名>")
		}
		if err := pool.ActivateProfile(args[1]); err != nil {
			return err
		}
		fmt.Printf("已切换默认运行槽位: %s\n", args[1])
	case "remove":
		if len(args) != 2 {
			return fmt.Errorf("用法: token-manager remove <槽位名>")
		}
		result, err := pool.RemoveProfile(args[1])
		if err != nil {
			return err
		}
		fmt.Println(result.Message)
	case "login":
		if len(args) < 2 || len(args) > 3 {
			return fmt.Errorf("用法: token-manager login <槽位名> [--manual]")
		}
		manual := strings.EqualFold(os.Getenv("TOKEN_MANAGER_LOGIN_MODE"), "manual")
		if len(args) == 3 {
			if args[2] != "--manual" {
				return fmt.Errorf("未知 login 参数: %s", args[2])
			}
			manual = true
		}
		return loginProfile(pool, args[1], manual)
	case "probe":
		if len(args) != 2 {
			return fmt.Errorf("用法: token-manager probe <槽位名>")
		}
		result, err := pool.ProbeProfile(args[1])
		if err != nil {
			return err
		}
		fmt.Printf("%s\t%s\t%s\n", result.ProfileName, result.Status, result.Reason)
		if result.AccountEmail != "" {
			fmt.Printf("账号: %s\n", result.AccountEmail)
		}
		if result.Usage.FiveHour != nil {
			fmt.Printf("5 小时剩余: %d%%\n", result.Usage.FiveHour.LeftPercent)
		}
		if result.Usage.Week != nil {
			fmt.Printf("本周剩余: %d%%\n", result.Usage.Week.LeftPercent)
		}
	default:
		printUsage()
		return fmt.Errorf("未知命令: %s", args[0])
	}
	return nil
}

func printUsage() {
	fmt.Println("Token Manager Tools")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  token-manager list")
	fmt.Println("  token-manager create <槽位名>")
	fmt.Println("  token-manager login <槽位名> [--manual]")
	fmt.Println("  token-manager probe <槽位名>")
	fmt.Println("  token-manager activate <槽位名>")
	fmt.Println("  token-manager remove <槽位名>")
	fmt.Println("  token-manager start [地址] [--no-open]")
	fmt.Println("  token-manager stop")
	fmt.Println("  token-manager status")
	fmt.Println("  token-manager serve [地址] [--no-open]")
}

func runServe(args []string) error {
	addr, openBrowser, err := parseServeArgs(args)
	if err != nil {
		return err
	}
	config := accountpool.Config{}
	if strings.TrimSpace(os.Getenv("TOKEN_MANAGER_OAUTH_REDIRECT_URL")) == "" {
		config.OAuthRedirectURL = oauthRedirectURLForAddr(addr)
	}
	pool, err := accountpool.New(config)
	if err != nil {
		return err
	}
	return serveProfiles(pool, addr, openBrowser)
}

func parseServeArgs(args []string) (string, bool, error) {
	addr := firstNonEmpty(os.Getenv("TOKEN_MANAGER_SERVER_ADDR"), "127.0.0.1:1455")
	openBrowser := os.Getenv("TOKEN_MANAGER_SERVER_NO_OPEN") != "1"
	for _, arg := range args {
		switch arg {
		case "--no-open":
			openBrowser = false
		default:
			if strings.HasPrefix(arg, "-") {
				return "", false, fmt.Errorf("未知 serve 参数: %s", arg)
			}
			addr = arg
		}
	}
	normalized, err := normalizeListenAddr(addr)
	if err != nil {
		return "", false, err
	}
	return normalized, openBrowser, nil
}

func normalizeListenAddr(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", errors.New("监听地址不能为空")
	}
	if _, err := strconv.Atoi(addr); err == nil {
		return "127.0.0.1:" + addr, nil
	}
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr, nil
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("监听地址无效: %s", addr)
	}
	if err := validateListenHost(host); err != nil {
		return "", err
	}
	return addr, nil
}

func validateListenHost(host string) error {
	host = strings.Trim(host, "[]")
	if host == "" || host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	if os.Getenv("TOKEN_MANAGER_ALLOW_REMOTE") == "1" {
		return nil
	}
	return fmt.Errorf("为保护本机 token，serve 默认只允许监听 127.0.0.1/localhost；如确需远程访问，先设置 TOKEN_MANAGER_ALLOW_REMOTE=1")
}

func serveProfiles(pool *accountpool.AccountPool, addr string, openBrowser bool) error {
	listeners, err := listenLoopbackAddrs(addr)
	if err != nil {
		return err
	}
	for _, listener := range listeners {
		defer listener.Close()
	}

	uiURL := browserURLForAddr(addr)
	fmt.Printf("账号池服务已启动: %s\n", uiURL)
	if openBrowser {
		if err := platform.OpenBrowser(uiURL); err != nil {
			fmt.Printf("自动打开浏览器失败，请手动访问上面的地址。原因: %v\n", err)
		}
	}
	handler := localserver.NewHandler(pool)
	errCh := make(chan error, len(listeners))
	for _, listener := range listeners {
		server := &http.Server{
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
		}
		go func(listener net.Listener) {
			if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
		}(listener)
	}
	return <-errCh
}

func browserURLForAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://localhost:1455/"
	}
	switch strings.Trim(host, "[]") {
	case "", "localhost", "127.0.0.1", "::1":
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port) + "/"
}

func callbackHostForAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "127.0.0.1:1455"
	}
	if host == "" || host == "::" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
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

func listenLoopbackAddrs(addr string) ([]net.Listener, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	host = strings.Trim(host, "[]")
	if host != "" && host != "localhost" {
		if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
			listener, err := net.Listen("tcp", addr)
			if err != nil {
				return nil, err
			}
			return []net.Listener{listener}, nil
		}
	}

	candidates := []string{
		net.JoinHostPort("127.0.0.1", port),
		net.JoinHostPort("::1", port),
	}
	if host == "::1" {
		candidates[0], candidates[1] = candidates[1], candidates[0]
	}

	listeners := make([]net.Listener, 0, len(candidates))
	seen := map[string]struct{}{}
	for index, candidate := range candidates {
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		listener, err := net.Listen("tcp", candidate)
		if err != nil {
			if index == 0 {
				return nil, err
			}
			if isAddrInUse(err) {
				for _, listener := range listeners {
					listener.Close()
				}
				return nil, fmt.Errorf("本机已有程序占用了 %s，导致 localhost 回调会串到别的服务；请改用其他端口，例如 token-manager serve 18080", candidate)
			}
			continue
		}
		listeners = append(listeners, listener)
	}
	if len(listeners) == 0 {
		return nil, fmt.Errorf("监听地址无效: %s", addr)
	}
	return listeners, nil
}

func isAddrInUse(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func loginProfile(pool *accountpool.AccountPool, profileName string, manual bool) error {
	flow, err := pool.StartLogin(profileName)
	if err != nil {
		return err
	}
	if manual {
		return loginProfileManual(pool, profileName, flow, os.Stdin)
	}

	u, err := url.Parse(flow.RedirectURL)
	if err != nil {
		return err
	}
	if u.Host == "" || u.Path == "" {
		return fmt.Errorf("无效 callback 地址: %s", flow.RedirectURL)
	}

	type callbackResult struct {
		email string
		err   error
	}
	resultCh := make(chan callbackResult, 1)
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:              u.Host,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	mux.HandleFunc(u.Path, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("state"); got != flow.State {
			http.Error(w, "Unknown login flow", http.StatusBadRequest)
			resultCh <- callbackResult{err: errors.New("登录回调校验失败")}
			return
		}
		if authErr := r.URL.Query().Get("error"); authErr != "" {
			http.Error(w, "Authentication failed", http.StatusBadRequest)
			resultCh <- callbackResult{err: fmt.Errorf("登录失败: %s", authErr)}
			return
		}
		code := r.URL.Query().Get("code")
		if strings.TrimSpace(code) == "" {
			http.Error(w, "Missing code", http.StatusBadRequest)
			resultCh <- callbackResult{err: errors.New("登录回调缺少 code")}
			return
		}
		tokens, err := pool.CompleteLogin(profileName, code, flow.Verifier)
		if err != nil {
			http.Error(w, "Token exchange failed", http.StatusInternalServerError)
			resultCh <- callbackResult{err: err}
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<!doctype html><html><body><p>登录成功，可以关闭这个页面。</p></body></html>")
		resultCh <- callbackResult{email: tokens.Email}
	})

	listener, err := net.Listen("tcp", u.Host)
	if err != nil {
		fmt.Printf("登录回调端口不可用，已切到手动模式: %v\n", err)
		return loginProfileManual(pool, profileName, flow, os.Stdin)
	}
	defer listener.Close()
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			resultCh <- callbackResult{err: err}
		}
	}()
	defer server.Shutdown(context.Background())

	fmt.Println("请在浏览器完成登录：")
	fmt.Println(flow.AuthURL)
	if err := platform.OpenBrowser(flow.AuthURL); err != nil {
		fmt.Printf("自动打开浏览器失败，请手动复制上面的地址。原因: %v\n", err)
	}

	select {
	case result := <-resultCh:
		if result.err != nil {
			return result.err
		}
		if result.email != "" {
			fmt.Printf("登录完成: %s\n", result.email)
		} else {
			fmt.Println("登录完成")
		}
		return nil
	case <-time.After(10 * time.Minute):
		return errors.New("登录超时，请重新执行 login")
	}
}

func loginProfileManual(pool *accountpool.AccountPool, profileName string, flow accountpool.LoginFlow, input io.Reader) error {
	fmt.Println("手动登录模式：")
	fmt.Println(flow.AuthURL)
	if err := platform.OpenBrowser(flow.AuthURL); err != nil {
		fmt.Printf("自动打开浏览器失败，请手动复制上面的地址。原因: %v\n", err)
	}
	fmt.Println("完成登录后，复制浏览器最终跳转地址，粘贴到这里后回车。")
	fmt.Print("> ")

	line, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	code, err := parseManualLoginCode(line, flow.State)
	if err != nil {
		return err
	}
	tokens, err := pool.CompleteLogin(profileName, code, flow.Verifier)
	if err != nil {
		return err
	}
	if tokens.Email != "" {
		fmt.Printf("登录完成: %s\n", tokens.Email)
	} else {
		fmt.Println("登录完成")
	}
	return nil
}

func parseManualLoginCode(rawInput, expectedState string) (string, error) {
	return accountpool.ParseManualLoginCode(rawInput, expectedState)
}
