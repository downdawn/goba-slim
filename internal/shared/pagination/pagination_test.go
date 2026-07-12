package pagination

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParamsNormalizeAndOffset(t *testing.T) {
	require.Equal(t, Params{Page: 1, Size: DefaultPageSize}, (Params{}).Normalize())
	require.Equal(t, Params{Page: 2, Size: MaxPageSize}, (Params{Page: 2, Size: MaxPageSize + 1}).Normalize())
	require.Equal(t, int32(40), (Params{Page: 3, Size: 20}).Offset())
}
