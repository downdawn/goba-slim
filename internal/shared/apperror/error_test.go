package apperror

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrorPreservesCauseAndDetails(t *testing.T) {
	cause := errors.New("cause")
	err := Validation("INVALID_INPUT", "error.invalid_input", "参数错误", cause).WithDetails(map[string]any{"field": "name"}, true)

	require.ErrorIs(t, err, cause)
	require.Equal(t, http.StatusBadRequest, err.HTTPStatus)
	require.Equal(t, "INVALID_INPUT", err.Code)
	require.True(t, err.ExposeDetails)
	require.Equal(t, map[string]any{"field": "name"}, err.Details)
}
