// Command MDViewer is the Wails desktop application. The root package stays
// deliberately small: Wails builds the project from this directory, so main
// and the embedded frontend assets live here, while the application logic is
// in internal/mdviewer.
package main

import (
	"embed"

	"MDViewer/internal/mdviewer"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	mdviewer.Run(assets)
}
