// GoBA Slim 本地开发入口，可在 GoLand 中直接右键运行或调试本文件。
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/downdawn/goba-slim/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	command := cli.NewRoot(cli.Dependencies{})
	command.SetArgs([]string{"serve", "--config", "configs/config.local.yaml", "--load-dotenv"})
	command.SetOut(os.Stdout)
	command.SetErr(os.Stderr)
	if err := command.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
