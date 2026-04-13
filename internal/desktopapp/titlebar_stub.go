//go:build !darwin

package desktopapp

func (app *App) installTitlebarAccessory() {}

func (app *App) removeTitlebarAccessory() {}

func (app *App) RefreshTitlebarAccessory() {}
