package id

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUUIDv7GeneratesVersionSeven(t *testing.T) {
	value, err := (UUIDv7{}).New()

	require.NoError(t, err)
	require.Equal(t, uuid.Version(7), value.Version())
}
