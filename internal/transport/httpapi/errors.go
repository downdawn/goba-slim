package httpapi

import (
	"errors"
	"net/http"

	"github.com/downdawn/goba-slim/internal/shared/apperror"
	"github.com/downdawn/goba-slim/internal/shared/requestmeta"
	"github.com/gin-gonic/gin"
)

type ErrorResponse struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"request_id,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
}

func WriteError(ctx *gin.Context, err error) {
	if ctx.Writer.Written() {
		ctx.Abort()
		return
	}

	statusCode, response := responseFromError(err)
	if requestID, ok := requestmeta.RequestID(ctx.Request.Context()); ok {
		response.RequestID = requestID
	}
	ctx.AbortWithStatusJSON(statusCode, response)
}

func responseFromError(err error) (int, ErrorResponse) {
	fallback := ErrorResponse{
		Code:    "INTERNAL_ERROR",
		Message: "服务器内部错误",
	}

	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr == nil ||
		appErr.HTTPStatus < http.StatusBadRequest || appErr.HTTPStatus > 599 ||
		appErr.Code == "" || appErr.DefaultMessage == "" {
		return http.StatusInternalServerError, fallback
	}

	response := ErrorResponse{
		Code:    appErr.Code,
		Message: appErr.DefaultMessage,
	}
	if appErr.ExposeDetails {
		response.Details = cloneDetails(appErr.Details)
	}
	return appErr.HTTPStatus, response
}

func cloneDetails(details map[string]any) map[string]any {
	if details == nil {
		return nil
	}

	cloned := make(map[string]any, len(details))
	for key, value := range details {
		cloned[key] = value
	}
	return cloned
}
