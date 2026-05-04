package main

import (
	"context"
	"fmt"
	"os"

	"github.com/invariantcontinuum/agentctl/internal/cli"
	"github.com/invariantcontinuum/agentctl/internal/driver"
	"github.com/invariantcontinuum/agentctl/internal/store"
)

func main() {
	repoPath, err := store.DefaultPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "agentctl: %v\n", err)
		os.Exit(1)
	}

	app := cli.New(os.Stdout, os.Stderr, store.NewJSONRepository(repoPath), driver.NewLocal())
	os.Exit(app.Run(context.Background(), os.Args[1:]))
}
