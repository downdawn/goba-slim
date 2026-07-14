package cli

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

type ReadinessProbeFunc func(context.Context, string) error

func newHealthcheckCommand(deps Dependencies) *cobra.Command {
	url := "http://127.0.0.1:8000/readyz"
	cmd := &cobra.Command{
		Use:    "healthcheck",
		Short:  "检查本机 API readiness",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := deps.Probe(cmd.Context(), url); err != nil {
				return fmt.Errorf("服务尚未就绪: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&url, "url", url, "readiness URL")
	return cmd
}

func probeReadiness(ctx context.Context, url string) error {
	requestCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("创建 readiness 请求失败: %w", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("readiness 返回 HTTP %d", response.StatusCode)
	}
	return nil
}
