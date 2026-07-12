package requestmeta

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequestIDRoundTrip(t *testing.T) {
	ctx := WithRequestID(context.Background(), "request-123")

	value, ok := RequestID(ctx)

	require.True(t, ok)
	require.Equal(t, "request-123", value)
}

func TestRequestIDRejectsEmptyValue(t *testing.T) {
	value, ok := RequestID(WithRequestID(context.Background(), ""))

	require.False(t, ok)
	require.Empty(t, value)
}
