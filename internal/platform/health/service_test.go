package health

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceReportsSortedReadinessChecks(t *testing.T) {
	service := NewService(map[string]Check{
		"database": func(context.Context) error { return errors.New("unavailable") },
		"cache":    func(context.Context) error { return nil },
	})

	result := service.Ready(context.Background())

	require.False(t, result.Ready)
	require.Equal(t, StatusDown, result.Checks["database"].Status)
	require.Equal(t, StatusOK, result.Checks["cache"].Status)
	require.Equal(t, StatusOK, service.Live().Status)
}
