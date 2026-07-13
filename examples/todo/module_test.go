package todo

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestServiceCreateAndList(t *testing.T) {
	repository := &MemoryRepository{}
	service, err := NewService(repository, fixedID{value: uuid.MustParse("0198a1d1-2c3b-7abc-8def-0123456789ab")})
	require.NoError(t, err)

	created, err := service.Create(t.Context(), "  review module boundaries  ")
	require.NoError(t, err)
	require.Equal(t, "review module boundaries", created.Title)
	items, err := service.List(t.Context())
	require.NoError(t, err)
	require.Equal(t, []Todo{created}, items)
}

type fixedID struct{ value uuid.UUID }

func (f fixedID) New() (uuid.UUID, error) { return f.value, nil }
