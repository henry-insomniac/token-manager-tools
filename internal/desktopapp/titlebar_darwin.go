//go:build darwin

package desktopapp

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa
#include <stdlib.h>

void TMInstallTitlebarAccessory(const char *summary);
void TMUpdateTitlebarAccessory(const char *summary);
void TMRemoveTitlebarAccessory(void);
*/
import "C"

import (
	"strings"
	"sync"
	"unsafe"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	titlebarHandlerMu sync.RWMutex
	titlebarHandler   func(string)
)

func (app *App) installTitlebarAccessory() {
	setTitlebarHandler(app.handleTitlebarAction)
	app.RefreshTitlebarAccessory()
}

func (app *App) removeTitlebarAccessory() {
	setTitlebarHandler(nil)
	C.TMRemoveTitlebarAccessory()
}

func (app *App) RefreshTitlebarAccessory() {
	profiles, err := app.service.ListProfiles()
	summary := "当前：读取槽位失败"
	if err == nil {
		summary = titlebarSummary(profiles)
	}
	value := C.CString(summary)
	defer C.free(unsafe.Pointer(value))
	C.TMInstallTitlebarAccessory(value)
	C.TMUpdateTitlebarAccessory(value)
}

func (app *App) handleTitlebarAction(action string) {
	switch strings.TrimSpace(action) {
	case titlebarActionRefreshUsage:
		go app.refreshUsageFromMenu()
	case titlebarActionRunAutoSwitch:
		go app.runAutoSwitchCheckFromMenu()
	case titlebarActionShowQuota:
		active := currentManagedProfileFromService(app)
		if active != "" {
			app.FocusProfile(active)
			return
		}
		app.ShowWindow()
	}
}

func currentManagedProfileFromService(app *App) string {
	profiles, err := app.service.ListProfiles()
	if err != nil {
		return ""
	}
	active := currentManagedProfile(profiles)
	if active == nil {
		return ""
	}
	return active.Name
}

func setTitlebarHandler(handler func(string)) {
	titlebarHandlerMu.Lock()
	defer titlebarHandlerMu.Unlock()
	titlebarHandler = handler
}

//export tokenManagerDesktopTitlebarAction
func tokenManagerDesktopTitlebarAction(action *C.char) {
	goAction := strings.TrimSpace(C.GoString(action))
	titlebarHandlerMu.RLock()
	handler := titlebarHandler
	titlebarHandlerMu.RUnlock()
	if handler == nil || goAction == "" {
		return
	}
	go handler(goAction)
}

func (app *App) logTitlebarAccessoryState() {
	if app.ctx == nil {
		return
	}
	runtime.LogInfof(app.ctx, "titlebar accessory refreshed")
}
