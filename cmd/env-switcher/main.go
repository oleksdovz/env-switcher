package main

import (
	"context"
	"os"

	"github.com/dolf/env-switcher/internal/app"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	a := app.New(app.BuildInfo{Version: version, Commit: commit, Date: date})
	os.Exit(a.Run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
