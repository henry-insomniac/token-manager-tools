//go:build darwin

package desktopapp

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework Cocoa
#include <stdlib.h>

void TMApplyStatusItem(const char *title, const char *tooltip, const char *menuJSON);
void TMRemoveStatusItem(void);
char *TMStatusItemDebugState(void);
*/
import "C"

import (
	"encoding/json"
	"strings"
	"sync"
	"unsafe"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	statusItemHandlerMu sync.RWMutex
	statusItemHandler   func(string)
)

func (app *App) installStatusItem() {
	setStatusItemHandler(app.handleStatusItemAction)
}

func (app *App) removeStatusItem() {
	setStatusItemHandler(nil)
	C.TMRemoveStatusItem()
}

func (app *App) RefreshStatusItem() {
	profiles, err := app.service.ListProfiles()
	items := buildStatusItemMenuItems(profiles, err)
	payload, marshalErr := json.Marshal(items)
	if marshalErr != nil {
		return
	}
	title := C.CString(statusItemTitle)
	tooltip := C.CString(statusItemTooltip)
	menuJSON := C.CString(string(payload))
	defer C.free(unsafe.Pointer(title))
	defer C.free(unsafe.Pointer(tooltip))
	defer C.free(unsafe.Pointer(menuJSON))
	C.TMApplyStatusItem(title, tooltip, menuJSON)
	app.logStatusItemState("refresh")
}

func (app *App) handleStatusItemAction(action string) {
	kind, profileName := parseStatusAction(action)
	switch kind {
	case statusActionShowWindow:
		app.ShowWindow()
	case statusActionRefreshUsage:
		go app.refreshUsageFromMenu()
	case statusActionRunAutoSwitch:
		go app.runAutoSwitchCheckFromMenu()
	case statusActionFocusPrefix:
		app.FocusProfile(profileName)
	case statusActionProbePrefix:
		go app.probeProfileFromMenu(profileName)
	case statusActionQuit:
		if app.ctx != nil {
			runtime.Quit(app.ctx)
		}
	}
}

func setStatusItemHandler(handler func(string)) {
	statusItemHandlerMu.Lock()
	defer statusItemHandlerMu.Unlock()
	statusItemHandler = handler
}

func (app *App) logStatusItemState(stage string) {
	if app.ctx == nil {
		return
	}
	debug := C.TMStatusItemDebugState()
	if debug == nil {
		runtime.LogInfof(app.ctx, "status item state (%s): <nil>", stage)
		return
	}
	defer C.free(unsafe.Pointer(debug))
	runtime.LogInfof(app.ctx, "status item state (%s): %s", stage, C.GoString(debug))
}

//export tokenManagerDesktopStatusItemAction
func tokenManagerDesktopStatusItemAction(action *C.char) {
	goAction := strings.TrimSpace(C.GoString(action))
	statusItemHandlerMu.RLock()
	handler := statusItemHandler
	statusItemHandlerMu.RUnlock()
	if handler == nil || goAction == "" {
		return
	}
	go handler(goAction)
}
