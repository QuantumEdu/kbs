package main

import (
	"context"
	"os"

	"github.com/quantum-6/skillvault/internal/line"
)

func main() {
	os.Exit(line.DefaultCLI().Run(context.Background(), os.Args[1:]))
}
