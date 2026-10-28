// Command go-dash is a link aggregator panel with an embedded Vue/DHTMLX
// frontend and a SQLite database.
package main

import (
	"embed"
	"os"

	"github.com/jniltinho/go-dash/cmd"
)

// web/dist is produced by `make frontend` (vp build); web/dist/.gitkeep keeps
// the directory present so the embed directive always compiles.
//
//go:embed all:web/dist
var embeddedFiles embed.FS

func main() {
	os.Exit(cmd.Execute(embeddedFiles))
}
