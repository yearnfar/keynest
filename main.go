package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	appService := NewApp()
	app := application.New(application.Options{
		Name:        "KeyNest",
		Description: "Local KeePass-compatible password manager",
		Services: []application.Service{
			application.NewService(appService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		FileAssociations: []string{".kdbx"},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "KeyNest",
		Width:            1280,
		Height:           820,
		MinWidth:         900,
		MinHeight:        620,
		BackgroundColour: application.NewRGB(244, 246, 245),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
