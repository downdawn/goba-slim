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

	if err := execute(ctx, "db", "migrate", "--config", "configs/config.local.yaml", "--load-dotenv"); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := execute(ctx, "serve", "--config", "configs/config.local.yaml", "--load-dotenv"); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func execute(ctx context.Context, args ...string) error {
	//nolint:contextcheck // 构造 CLI 不启动 I/O，执行时才使用传入的 Context。
	command := cli.NewRoot(cli.Dependencies{})
	command.SetArgs(args)
	command.SetOut(os.Stdout)
	command.SetErr(os.Stderr)
	return command.ExecuteContext(ctx)
}
