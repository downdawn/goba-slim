package httpapi

import (
	"net/http"

	"github.com/downdawn/goba-slim/api/openapi/generated"
	"github.com/downdawn/goba-slim/internal/platform/health"
	"github.com/gin-gonic/gin"
)

// Handler 实现 OpenAPI 定义的 HTTP 处理器。
type Handler struct {
	health *health.Service
}

// NewHandler 创建仅依赖健康服务的 HTTP 处理器。
func NewHandler(healthService *health.Service) *Handler {
	return &Handler{health: healthService}
}

// GetLiveness 返回不访问外部依赖的进程存活状态。
func (h *Handler) GetLiveness(ctx *gin.Context) {
	result := h.health.Live()
	ctx.JSON(http.StatusOK, generated.LivenessResponse{
		Status: generated.LivenessResponseStatus(result.Status),
	})
}

// GetReadiness 返回必要依赖及模块的就绪状态，且不公开内部错误信息。
func (h *Handler) GetReadiness(ctx *gin.Context) {
	result := h.health.Ready(ctx.Request.Context())
	response := generated.ReadinessResponse{
		Ready:  result.Ready,
		Checks: make(map[string]generated.CheckStatus, len(result.Checks)),
	}
	for name, check := range result.Checks {
		response.Checks[name] = generated.CheckStatus{
			Status: generated.CheckStatusStatus(check.Status),
		}
	}

	status := http.StatusOK
	if !result.Ready {
		status = http.StatusServiceUnavailable
	}
	ctx.JSON(status, response)
}

var _ generated.ServerInterface = (*Handler)(nil)
