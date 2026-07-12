package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/downdawn/goba-slim/internal/shared/apperror"
	"github.com/downdawn/goba-slim/internal/shared/requestmeta"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWriteErrorMapsWrappedApplicationError(t *testing.T) {
	appErr := apperror.NotFound("USER_NOT_FOUND", "error.user_not_found", "用户不存在", errors.New("sql: no rows"))
	recorder, ctx := testContext("req-404")

	WriteError(ctx, fmt.Errorf("lookup user: %w", appErr))

	require.Equal(t, http.StatusNotFound, recorder.Code)
	var response ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, ErrorResponse{Code: "USER_NOT_FOUND", Message: "用户不存在", RequestID: "req-404"}, response)
	require.NotContains(t, recorder.Body.String(), "sql")
}

func TestWriteErrorHidesUnknownCause(t *testing.T) {
	recorder, ctx := testContext("req-1")

	WriteError(ctx, errors.New(`password=secret sql="SELECT *" path=C:\private`))

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "secret")
	require.NotContains(t, recorder.Body.String(), "SELECT")
	require.NotContains(t, recorder.Body.String(), `C:\private`)
	var response ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, ErrorResponse{Code: "INTERNAL_ERROR", Message: "服务器内部错误", RequestID: "req-1"}, response)
}

func TestWriteErrorOnlyExposesAllowedDetails(t *testing.T) {
	tests := []struct {
		name    string
		expose  bool
		details map[string]any
	}{
		{name: "hidden", expose: false, details: nil},
		{name: "exposed", expose: true, details: map[string]any{"field": "username"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder, ctx := testContext("")
			err := apperror.Validation("VALIDATION_ERROR", "error.validation", "参数错误", nil).
				WithDetails(map[string]any{"field": "username"}, tt.expose)

			WriteError(ctx, err)

			var response ErrorResponse
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
			require.Equal(t, tt.details, response.Details)
		})
	}
}

func TestResponseFromErrorSafelyFallsBackForInvalidErrors(t *testing.T) {
	var typedNil *apperror.Error
	tests := []struct {
		name string
		err  error
	}{
		{name: "nil", err: nil},
		{name: "typed nil", err: typedNil},
		{name: "status zero", err: apperror.New("BAD", "error.bad", "错误", 0, nil)},
		{name: "status below range", err: apperror.New("BAD", "error.bad", "错误", 99, nil)},
		{name: "status above range", err: apperror.New("BAD", "error.bad", "错误", 600, nil)},
		{name: "empty code", err: apperror.New("", "error.bad", "错误", http.StatusBadRequest, nil)},
		{name: "empty message", err: apperror.New("BAD", "error.bad", "", http.StatusBadRequest, nil)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, response := responseFromError(tt.err)
			require.Equal(t, http.StatusInternalServerError, status)
			require.Equal(t, ErrorResponse{Code: "INTERNAL_ERROR", Message: "服务器内部错误"}, response)
		})
	}
}

func TestResponseFromErrorSnapshotsExposedDetails(t *testing.T) {
	details := map[string]any{"field": "username"}
	err := apperror.Validation("VALIDATION_ERROR", "error.validation", "参数错误", nil).
		WithDetails(details, true)

	_, response := responseFromError(err)
	err.Details["field"] = "password"

	require.Equal(t, map[string]any{"field": "username"}, response.Details)
}

func TestWriteErrorDoesNotAppendAfterResponseWritten(t *testing.T) {
	recorder, ctx := testContext("")
	ctx.String(http.StatusOK, "ok")

	WriteError(ctx, errors.New("secret"))

	require.True(t, ctx.IsAborted())
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "ok", recorder.Body.String())
}

func TestWriteErrorUsesJSONContentType(t *testing.T) {
	recorder, ctx := testContext("")

	WriteError(ctx, nil)

	require.Equal(t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"))
}

func testContext(requestID string) (*httptest.ResponseRecorder, *gin.Context) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	if requestID != "" {
		ctx.Request = ctx.Request.WithContext(requestmeta.WithRequestID(ctx.Request.Context(), requestID))
	}
	return recorder, ctx
}
