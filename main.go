package main

import (
	"embed"

	"api2windsurf/internal/app"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	application := app.New()

	err := wails.Run(&options.App{
		Title:     "API2Windsurf",
		Width:     960,
		Height:    720,
		MinWidth:  760,
		MinHeight: 560,
		Frameless: true,
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			Theme:                windows.Dark,
		},
		BackgroundColour: &options.RGBA{R: 24, G: 23, B: 21, A: 255},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "com.api2windsurf.single",
		},
		OnStartup:  application.Startup,
		OnShutdown: application.Shutdown,
		Bind: []interface{}{
			application,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
