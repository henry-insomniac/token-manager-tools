package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/henry-insomniac/token-manager-tools/internal/accountpool"
	"github.com/henry-insomniac/token-manager-tools/internal/desktopapp"
	"github.com/henry-insomniac/token-manager-tools/internal/desktopruntime"
	localserver "github.com/henry-insomniac/token-manager-tools/internal/server"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsmac "github.com/wailsapp/wails/v2/pkg/options/mac"
)

func main() {
	startHidden := hasArg(os.Args[1:], desktopruntime.StartHiddenArg)
	app, err := desktopapp.New(accountpool.Config{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	err = wails.Run(&options.App{
		Title:             "Token Manager Tools",
		Width:             1280,
		Height:            860,
		MinWidth:          1080,
		MinHeight:         720,
		StartHidden:       startHidden,
		HideWindowOnClose: true,
		AssetServer: &assetserver.Options{
			Assets:  localserver.StaticAssets(),
			Handler: app.AssetsHandler(),
		},
		Menu: app.ApplicationMenu(),
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "com.henryinsomniac.token-manager-tools.desktop",
			OnSecondInstanceLaunch: func(secondInstanceData options.SecondInstanceData) {
				app.ShowWindow()
			},
		},
		Bind:             app.Bindings(),
		OnStartup:        app.Startup,
		OnDomReady:       app.DomReady,
		OnShutdown:       app.Shutdown,
		BackgroundColour: &options.RGBA{R: 21, G: 25, B: 22, A: 255},
		Mac:              &wailsmac.Options{},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func hasArg(args []string, want string) bool {
	for _, arg := range args {
		if strings.TrimSpace(arg) == want {
			return true
		}
	}
	return false
}
