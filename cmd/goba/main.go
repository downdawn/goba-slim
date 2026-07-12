package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/downdawn/goba-slim/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	//nolint:contextcheck // 根命令通过 ExecuteContext 接收并向子命令传播 ctx。
	cmd := cli.NewRoot(cli.Dependencies{})
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	if err := cmd.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return err
	}
	return nil
}
