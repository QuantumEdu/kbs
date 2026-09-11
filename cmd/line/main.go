package main

import (
	"context"
	"os"

	"github.com/quantum-6/skillvault/internal/line"
	"github.com/quantum-6/skillvault/internal/line/tui"
)

func main() {
	cli := line.DefaultCLI()
	cli.RunTUI = func(ctx context.Context, engine *line.Engine) error {
		return tui.Run(ctx, engine)
	}
	os.Exit(cli.Run(context.Background(), os.Args[1:]))
}
