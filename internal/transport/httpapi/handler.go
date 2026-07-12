package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/downdawn/goba-slim/api/openapi/generated"
	"github.com/downdawn/goba-slim/internal/modules/auth"
	"github.com/downdawn/goba-slim/internal/modules/user"
	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/downdawn/goba-slim/internal/platform/health"
	"github.com/downdawn/goba-slim/internal/shared/apperror"
	"github.com/gin-gonic/gin"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Handler 实现 OpenAPI 定义的 HTTP 处理器。
type Handler struct {
	health *health.Service
	auth   *auth.Service
	users  *user.Service
	config config.AuthConfig
	cors   config.CORSConfig
}

type HandlerOptions struct {
	Health     *health.Service
	Auth       *auth.Service
	Users      *user.Service
	AuthConfig config.AuthConfig
	CORS       config.CORSConfig
}

func NewHandler(options HandlerOptions) *Handler {
	return &Handler{health: options.Health, auth: options.Auth, users: options.Users, config: options.AuthConfig, cors: options.CORS}
}

func (h *Handler) Login(ctx *gin.Context) {
	var request generated.LoginRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		WriteError(ctx, apperror.Validation("INVALID_LOGIN", "error.invalid_login", "登录参数无效", err))
		return
	}
	pair, err := h.auth.Login(ctx.Request.Context(), request.Username, request.Password, ctx.ClientIP())
	if err != nil {
		WriteError(ctx, authHTTPError(err))
		return
	}
	h.setRefreshCookie(ctx, pair.RefreshToken)
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, tokenResponse(pair))
}

func (h *Handler) RefreshToken(ctx *gin.Context) {
	if !h.originAllowed(ctx.GetHeader("Origin")) {
		WriteError(ctx, apperror.New("ORIGIN_REJECTED", "error.origin_rejected", "请求来源不受信任", http.StatusForbidden, nil))
		return
	}
	refresh, err := ctx.Cookie(h.config.RefreshCookie)
	if err != nil {
		WriteError(ctx, authHTTPError(auth.ErrInvalidToken))
		return
	}
	pair, err := h.auth.Refresh(ctx.Request.Context(), refresh)
	if err != nil {
		h.deleteRefreshCookie(ctx)
		WriteError(ctx, authHTTPError(err))
		return
	}
	h.setRefreshCookie(ctx, pair.RefreshToken)
	ctx.Header("Cache-Control", "no-store")
	ctx.JSON(http.StatusOK, tokenResponse(pair))
}

func (h *Handler) Logout(ctx *gin.Context) {
	identity, err := h.identity(ctx)
	if err != nil {
		WriteError(ctx, err)
		return
	}
	if err := h.auth.Logout(ctx.Request.Context(), identity.SessionID); err != nil {
		WriteError(ctx, err)
		return
	}
	h.deleteRefreshCookie(ctx)
	ctx.Status(http.StatusNoContent)
}

func (*Handler) GetPasswordPolicy(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, generated.PasswordPolicy{MinLength: 8, MaxLength: 128, RequireLetter: true, RequireDigit: true, RejectCommon: true})
}

func (h *Handler) GetCurrentUser(ctx *gin.Context) {
	identity, err := h.identity(ctx)
	if err != nil {
		WriteError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, userResponse(identity.User))
}

func (h *Handler) ChangePassword(ctx *gin.Context) {
	identity, err := h.identity(ctx)
	if err != nil {
		WriteError(ctx, err)
		return
	}
	var request generated.ChangePasswordRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		WriteError(ctx, apperror.Validation("INVALID_PASSWORD", "error.invalid_password", "密码参数无效", err))
		return
	}
	if _, err := h.users.ChangePassword(ctx.Request.Context(), identity.User.ID, request.OldPassword, request.NewPassword); err != nil {
		WriteError(ctx, userHTTPError(err))
		return
	}
	if err := h.auth.RevokeUser(ctx.Request.Context(), identity.User.ID); err != nil {
		WriteError(ctx, err)
		return
	}
	h.deleteRefreshCookie(ctx)
	ctx.Status(http.StatusNoContent)
}

func (h *Handler) ListUsers(ctx *gin.Context, params generated.ListUsersParams) {
	if _, err := h.superuser(ctx); err != nil {
		WriteError(ctx, err)
		return
	}
	filter := user.ListFilter{}
	if params.Page != nil {
		if *params.Page < 1 || int64(*params.Page) > int64(^uint32(0)>>1) {
			WriteError(ctx, apperror.Validation("INVALID_PAGE", "error.invalid_page", "分页参数无效", nil))
			return
		}
		// #nosec G115 -- 已显式检查正数 int32 上界。
		filter.Page = int32(*params.Page)
	}
	if params.Size != nil {
		if *params.Size < 1 || *params.Size > 100 {
			WriteError(ctx, apperror.Validation("INVALID_PAGE", "error.invalid_page", "分页参数无效", nil))
			return
		}
		// #nosec G115 -- OpenAPI 与上述检查将 size 限制在 1 到 100。
		filter.Size = int32(*params.Size)
	}
	if params.Username != nil {
		filter.Username = *params.Username
	}
	if params.Status != nil {
		filter.Status = user.Status(*params.Status)
	}
	page, err := h.users.List(ctx.Request.Context(), filter)
	if err != nil {
		WriteError(ctx, userHTTPError(err))
		return
	}
	items := make([]generated.User, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, userResponse(item))
	}
	ctx.JSON(http.StatusOK, generated.UserPage{Items: items, Pagination: generated.Pagination{Page: int(page.Page), Size: int(page.Size), Total: page.Total}})
}

func (h *Handler) CreateUser(ctx *gin.Context) {
	if _, err := h.superuser(ctx); err != nil {
		WriteError(ctx, err)
		return
	}
	var request generated.CreateUserRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		WriteError(ctx, apperror.Validation("INVALID_USER", "error.invalid_user", "用户参数无效", err))
		return
	}
	input := user.CreateInput{Username: request.Username, Password: request.Password}
	if request.DisplayName != nil {
		input.DisplayName = *request.DisplayName
	}
	if request.Email != nil {
		input.Email = string(*request.Email)
	}
	if request.AvatarUrl != nil {
		input.AvatarURL = *request.AvatarUrl
	}
	if request.IsSuperuser != nil {
		input.IsSuperuser = *request.IsSuperuser
	}
	if request.AllowMultipleSessions != nil {
		input.AllowMultipleSessions = *request.AllowMultipleSessions
	}
	created, err := h.users.Create(ctx.Request.Context(), input)
	if err != nil {
		WriteError(ctx, userHTTPError(err))
		return
	}
	ctx.JSON(http.StatusCreated, userResponse(created))
}

func (h *Handler) UpdateUser(ctx *gin.Context, userID openapi_types.UUID) {
	if _, err := h.superuser(ctx); err != nil {
		WriteError(ctx, err)
		return
	}
	var request generated.UpdateUserRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		WriteError(ctx, apperror.Validation("INVALID_USER", "error.invalid_user", "用户参数无效", err))
		return
	}
	input := user.UpdateProfileInput{Username: request.Username, DisplayName: request.DisplayName}
	if request.Email != nil {
		input.Email = string(*request.Email)
	}
	if request.AvatarUrl != nil {
		input.AvatarURL = *request.AvatarUrl
	}
	updated, err := h.users.UpdateProfile(ctx.Request.Context(), userID, input)
	if err != nil {
		WriteError(ctx, userHTTPError(err))
		return
	}
	ctx.JSON(http.StatusOK, userResponse(updated))
}

func (h *Handler) SetUserStatus(ctx *gin.Context, userID openapi_types.UUID) {
	if _, err := h.superuser(ctx); err != nil {
		WriteError(ctx, err)
		return
	}
	var request generated.SetUserStatusRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		WriteError(ctx, apperror.Validation("INVALID_STATUS", "error.invalid_status", "用户状态无效", err))
		return
	}
	updated, err := h.users.SetStatus(ctx.Request.Context(), userID, user.Status(request.Status))
	if err != nil {
		WriteError(ctx, userHTTPError(err))
		return
	}
	if updated.Status != user.StatusActive {
		if err := h.auth.RevokeUser(ctx.Request.Context(), updated.ID); err != nil {
			WriteError(ctx, err)
			return
		}
	}
	ctx.JSON(http.StatusOK, userResponse(updated))
}

func (h *Handler) ResetUserPassword(ctx *gin.Context, userID openapi_types.UUID) {
	if _, err := h.superuser(ctx); err != nil {
		WriteError(ctx, err)
		return
	}
	var request generated.ResetPasswordRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		WriteError(ctx, apperror.Validation("INVALID_PASSWORD", "error.invalid_password", "密码参数无效", err))
		return
	}
	if _, err := h.users.ResetPassword(ctx.Request.Context(), userID, request.Password); err != nil {
		WriteError(ctx, userHTTPError(err))
		return
	}
	if err := h.auth.RevokeUser(ctx.Request.Context(), userID); err != nil {
		WriteError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (h *Handler) ArchiveUser(ctx *gin.Context, userID openapi_types.UUID) {
	if _, err := h.superuser(ctx); err != nil {
		WriteError(ctx, err)
		return
	}
	if _, err := h.users.Archive(ctx.Request.Context(), userID); err != nil {
		WriteError(ctx, userHTTPError(err))
		return
	}
	if err := h.auth.RevokeUser(ctx.Request.Context(), userID); err != nil {
		WriteError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (h *Handler) superuser(ctx *gin.Context) (auth.Identity, error) {
	identity, err := h.identity(ctx)
	if err != nil {
		return auth.Identity{}, err
	}
	if !identity.User.IsSuperuser {
		return auth.Identity{}, apperror.New("SUPERUSER_REQUIRED", "error.superuser_required", "需要超级管理员权限", http.StatusForbidden, nil)
	}
	return identity, nil
}

func (h *Handler) identity(ctx *gin.Context) (auth.Identity, error) {
	value := ctx.GetHeader("Authorization")
	scheme, encoded, ok := strings.Cut(value, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || encoded == "" || h.auth == nil {
		return auth.Identity{}, authHTTPError(auth.ErrInvalidToken)
	}
	identity, err := h.auth.Authenticate(ctx.Request.Context(), encoded)
	if err != nil {
		return auth.Identity{}, authHTTPError(err)
	}
	return identity, nil
}

func (h *Handler) setRefreshCookie(ctx *gin.Context, value string) {
	// #nosec G124 -- production 配置强制 Secure，development 允许本机 HTTP 调试。
	http.SetCookie(ctx.Writer, &http.Cookie{Name: h.config.RefreshCookie, Value: value, Path: h.config.CookiePath, Domain: h.config.CookieDomain, MaxAge: int(h.config.RefreshTokenTTL.Seconds()), HttpOnly: true, Secure: h.config.CookieSecure, SameSite: sameSite(h.config.CookieSameSite)})
}

func (h *Handler) deleteRefreshCookie(ctx *gin.Context) {
	// #nosec G124 -- 删除 Cookie 使用与设置时完全相同的受校验安全属性。
	http.SetCookie(ctx.Writer, &http.Cookie{Name: h.config.RefreshCookie, Path: h.config.CookiePath, Domain: h.config.CookieDomain, MaxAge: -1, HttpOnly: true, Secure: h.config.CookieSecure, SameSite: sameSite(h.config.CookieSameSite)})
}

func (h *Handler) originAllowed(origin string) bool {
	if origin == "" {
		return true
	}
	for _, allowed := range h.cors.AllowOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

func sameSite(value string) http.SameSite {
	if value == "none" {
		return http.SameSiteNoneMode
	}
	if value == "lax" {
		return http.SameSiteLaxMode
	}
	return http.SameSiteStrictMode
}

func tokenResponse(pair auth.TokenPair) generated.TokenResponse {
	return generated.TokenResponse{AccessToken: pair.AccessToken, TokenType: generated.Bearer, ExpiresAt: pair.ExpiresAt, SessionId: pair.SessionID, User: userResponse(pair.User)}
}

func userResponse(item user.User) generated.User {
	var email *openapi_types.Email
	if item.Email != nil {
		value := openapi_types.Email(*item.Email)
		email = &value
	}
	return generated.User{Id: item.ID, Username: item.Username, DisplayName: item.DisplayName, Email: email, AvatarUrl: item.AvatarURL, Status: generated.UserStatus(item.Status), IsSuperuser: item.IsSuperuser, AllowMultipleSessions: item.AllowMultipleSessions, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func authHTTPError(err error) error {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials), errors.Is(err, auth.ErrInvalidToken), errors.Is(err, auth.ErrRefreshReuse):
		return apperror.New("AUTHENTICATION_FAILED", "error.authentication_failed", "认证失败", http.StatusUnauthorized, err)
	case errors.Is(err, auth.ErrUserDisabled):
		return apperror.New("USER_DISABLED", "error.user_disabled", "用户已停用", http.StatusForbidden, err)
	case errors.Is(err, auth.ErrRateLimited):
		return apperror.New("LOGIN_RATE_LIMITED", "error.login_rate_limited", "登录尝试过于频繁", http.StatusTooManyRequests, err)
	default:
		return err
	}
}

func userHTTPError(err error) error {
	switch {
	case errors.Is(err, user.ErrPasswordMismatch), errors.Is(err, user.ErrInvalidPassword), errors.Is(err, user.ErrInvalidInput):
		return apperror.Validation("INVALID_PASSWORD", "error.invalid_password", "密码不符合要求", err)
	case errors.Is(err, user.ErrNotFound):
		return apperror.NotFound("USER_NOT_FOUND", "error.user_not_found", "用户不存在", err)
	case errors.Is(err, user.ErrUsernameConflict), errors.Is(err, user.ErrEmailConflict), errors.Is(err, user.ErrLastSuperuser):
		return apperror.New("USER_CONFLICT", "error.user_conflict", "用户状态冲突", http.StatusConflict, err)
	}
	return err
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
