//go:build !darwin

package desktopapp

func (app *App) installStatusItem() {}

func (app *App) removeStatusItem() {}

func (app *App) RefreshStatusItem() {}

func (app *App) logStatusItemState(stage string) {}
