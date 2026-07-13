package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/downdawn/goba-slim/api/openapi/generated"
	"github.com/downdawn/goba-slim/internal/modules/auth"
	filemodule "github.com/downdawn/goba-slim/internal/modules/file"
	"github.com/downdawn/goba-slim/internal/modules/systemconfig"
	"github.com/downdawn/goba-slim/internal/modules/user"
	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/downdawn/goba-slim/internal/platform/health"
	"github.com/downdawn/goba-slim/internal/shared/apperror"
	"github.com/gin-gonic/gin"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Handler 实现 OpenAPI 定义的 HTTP 处理器。
type Handler struct {
	health        *health.Service
	auth          *auth.Service
	files         *filemodule.Service
	systemConfigs *systemconfig.Service
	users         *user.Service
	config        config.AuthConfig
	cors          config.CORSConfig
}

type HandlerOptions struct {
	Health        *health.Service
	Auth          *auth.Service
	Files         *filemodule.Service
	SystemConfigs *systemconfig.Service
	Users         *user.Service
	AuthConfig    config.AuthConfig
	CORS          config.CORSConfig
}

func NewHandler(options HandlerOptions) *Handler {
	return &Handler{health: options.Health, auth: options.Auth, files: options.Files, systemConfigs: options.SystemConfigs, users: options.Users, config: options.AuthConfig, cors: options.CORS}
}

func (h *Handler) ListPublicSystemConfigs(ctx *gin.Context) {
	items, err := h.systemConfigs.ListPublic(ctx.Request.Context())
	if err != nil {
		WriteError(ctx, systemConfigHTTPError(err))
		return
	}
	response := generated.PublicSystemConfigList{Items: make([]generated.PublicSystemConfig, 0, len(items))}
	for _, item := range items {
		value, decodeErr := decodeSystemConfigValue(item.Value)
		if decodeErr != nil {
			WriteError(ctx, decodeErr)
			return
		}
		response.Items = append(response.Items, generated.PublicSystemConfig{Key: item.Key, Value: value, ValueType: generated.SystemConfigValueType(item.ValueType)})
	}
	ctx.JSON(http.StatusOK, response)
}

func (h *Handler) ListSystemConfigs(ctx *gin.Context) {
	if _, err := h.superuser(ctx); err != nil {
		WriteError(ctx, err)
		return
	}
	items, err := h.systemConfigs.List(ctx.Request.Context())
	if err != nil {
		WriteError(ctx, systemConfigHTTPError(err))
		return
	}
	response := generated.SystemConfigList{Items: make([]generated.SystemConfig, 0, len(items))}
	for _, item := range items {
		mapped, mapErr := systemConfigResponse(item)
		if mapErr != nil {
			WriteError(ctx, mapErr)
			return
		}
		response.Items = append(response.Items, mapped)
	}
	ctx.JSON(http.StatusOK, response)
}

func (h *Handler) GetSystemConfig(ctx *gin.Context, key string) {
	if _, err := h.superuser(ctx); err != nil {
		WriteError(ctx, err)
		return
	}
	item, err := h.systemConfigs.Get(ctx.Request.Context(), key)
	if err != nil {
		WriteError(ctx, systemConfigHTTPError(err))
		return
	}
	response, err := systemConfigResponse(item)
	if err != nil {
		WriteError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, response)
}

func (h *Handler) CreateSystemConfig(ctx *gin.Context) {
	if _, err := h.superuser(ctx); err != nil {
		WriteError(ctx, err)
		return
	}
	var request systemConfigRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		WriteError(ctx, systemConfigHTTPError(systemconfig.ErrInvalidInput))
		return
	}
	if request.IsPublic == nil {
		WriteError(ctx, systemConfigHTTPError(systemconfig.ErrInvalidInput))
		return
	}
	input, err := systemConfigInput(request.Key, request.Value, request.ValueType, *request.IsPublic, request.Description)
	if err != nil {
		WriteError(ctx, err)
		return
	}
	item, err := h.systemConfigs.Create(ctx.Request.Context(), input)
	if err != nil {
		WriteError(ctx, systemConfigHTTPError(err))
		return
	}
	response, err := systemConfigResponse(item)
	if err != nil {
		WriteError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, response)
}

func (h *Handler) UpdateSystemConfig(ctx *gin.Context, key string) {
	if _, err := h.superuser(ctx); err != nil {
		WriteError(ctx, err)
		return
	}
	var request systemConfigRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		WriteError(ctx, systemConfigHTTPError(systemconfig.ErrInvalidInput))
		return
	}
	if request.IsPublic == nil {
		WriteError(ctx, systemConfigHTTPError(systemconfig.ErrInvalidInput))
		return
	}
	input, err := systemConfigInput(key, request.Value, request.ValueType, *request.IsPublic, request.Description)
	if err != nil {
		WriteError(ctx, err)
		return
	}
	item, err := h.systemConfigs.Update(ctx.Request.Context(), key, input)
	if err != nil {
		WriteError(ctx, systemConfigHTTPError(err))
		return
	}
	response, err := systemConfigResponse(item)
	if err != nil {
		WriteError(ctx, err)
		return
	}
	ctx.JSON(http.StatusOK, response)
}

func (h *Handler) DeleteSystemConfig(ctx *gin.Context, key string) {
	if _, err := h.superuser(ctx); err != nil {
		WriteError(ctx, err)
		return
	}
	if err := h.systemConfigs.Delete(ctx.Request.Context(), key); err != nil {
		WriteError(ctx, systemConfigHTTPError(err))
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (h *Handler) UploadFile(ctx *gin.Context) {
	identity, err := h.identity(ctx)
	if err != nil {
		WriteError(ctx, err)
		return
	}
	if h.files == nil {
		WriteError(ctx, fileHTTPError(filemodule.ErrUnavailable))
		return
	}
	reader, err := ctx.Request.MultipartReader()
	if err != nil {
		WriteError(ctx, fileHTTPError(filemodule.ErrInvalidFile))
		return
	}
	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			WriteError(ctx, fileHTTPError(filemodule.ErrInvalidFile))
			return
		}
		if nextErr != nil {
			WriteError(ctx, fileHTTPError(nextErr))
			return
		}
		if part.FormName() != "file" || part.FileName() == "" {
			_, _ = io.Copy(io.Discard, part)
			_ = part.Close()
			continue
		}
		uploaded, uploadErr := h.files.Upload(ctx.Request.Context(), identity.User.ID, part)
		_ = part.Close()
		if uploadErr != nil {
			WriteError(ctx, fileHTTPError(uploadErr))
			return
		}
		ctx.JSON(http.StatusCreated, generated.FileObject{
			Key: uploaded.Key.String(), Url: uploaded.URL(), ContentType: uploaded.ContentType, Size: uploaded.Size,
		})
		return
	}
}

func (h *Handler) GetFile(ctx *gin.Context, ownerID openapi_types.UUID, fileName string) {
	if h.files == nil {
		WriteError(ctx, fileHTTPError(filemodule.ErrNotFound))
		return
	}
	uploaded, object, err := h.files.Open(ctx.Request.Context(), ownerID.String()+"/"+fileName)
	if err != nil {
		WriteError(ctx, fileHTTPError(err))
		return
	}
	defer func() { _ = object.Content.Close() }()
	ctx.Header("Cache-Control", "public, max-age=31536000, immutable")
	ctx.Header("Content-Type", uploaded.ContentType)
	http.ServeContent(ctx.Writer, ctx.Request, fileName, object.ModifiedTime, object.Content)
}

func (h *Handler) DeleteFile(ctx *gin.Context, ownerID openapi_types.UUID, fileName string) {
	identity, err := h.identity(ctx)
	if err != nil {
		WriteError(ctx, err)
		return
	}
	if h.files == nil {
		WriteError(ctx, fileHTTPError(filemodule.ErrNotFound))
		return
	}
	err = h.files.Delete(ctx.Request.Context(), identity.User.ID, identity.User.IsSuperuser, ownerID.String()+"/"+fileName)
	if err != nil {
		WriteError(ctx, fileHTTPError(err))
		return
	}
	ctx.Status(http.StatusNoContent)
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
	case errors.Is(err, auth.ErrUnavailable):
		return apperror.New("AUTHENTICATION_UNAVAILABLE", "error.authentication_unavailable", "认证服务暂时不可用", http.StatusServiceUnavailable, err)
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

func fileHTTPError(err error) error {
	var maxBytesError *http.MaxBytesError
	switch {
	case errors.Is(err, filemodule.ErrInvalidFile):
		return apperror.Validation("FILE_INVALID", "error.file_invalid", "文件无效", err)
	case errors.Is(err, filemodule.ErrInvalidKey):
		return apperror.Validation("FILE_KEY_INVALID", "error.file_key_invalid", "文件 Key 无效", err)
	case errors.Is(err, filemodule.ErrTypeNotAllowed):
		return apperror.New("FILE_TYPE_NOT_ALLOWED", "error.file_type_not_allowed", "文件类型不受支持", http.StatusUnsupportedMediaType, err)
	case errors.Is(err, filemodule.ErrTooLarge), errors.As(err, &maxBytesError):
		return apperror.New("FILE_TOO_LARGE", "error.file_too_large", "文件超过大小限制", http.StatusRequestEntityTooLarge, err)
	case errors.Is(err, filemodule.ErrNotFound):
		return apperror.NotFound("FILE_NOT_FOUND", "error.file_not_found", "文件不存在", err)
	case errors.Is(err, filemodule.ErrForbidden):
		return apperror.New("FILE_DELETE_FORBIDDEN", "error.file_delete_forbidden", "无权删除该文件", http.StatusForbidden, err)
	case errors.Is(err, filemodule.ErrUnavailable):
		return apperror.New("FILE_STORAGE_UNAVAILABLE", "error.file_storage_unavailable", "文件存储暂时不可用", http.StatusServiceUnavailable, err)
	default:
		return err
	}
}

func systemConfigHTTPError(err error) error {
	switch {
	case errors.Is(err, systemconfig.ErrInvalidType):
		return apperror.Validation("SYSTEM_CONFIG_INVALID_TYPE", "error.system_config_invalid_type", "动态配置值与类型不匹配", err)
	case errors.Is(err, systemconfig.ErrSensitiveKey):
		return apperror.New("SYSTEM_CONFIG_SENSITIVE_KEY", "error.system_config_sensitive_key", "该配置键属于启动或安全边界", http.StatusForbidden, err)
	case errors.Is(err, systemconfig.ErrInvalidInput):
		return apperror.Validation("SYSTEM_CONFIG_INVALID", "error.system_config_invalid", "动态配置参数无效", err)
	case errors.Is(err, systemconfig.ErrNotFound):
		return apperror.NotFound("SYSTEM_CONFIG_NOT_FOUND", "error.system_config_not_found", "动态配置不存在", err)
	case errors.Is(err, systemconfig.ErrConflict):
		return apperror.New("SYSTEM_CONFIG_CONFLICT", "error.system_config_conflict", "动态配置键已存在", http.StatusConflict, err)
	case errors.Is(err, systemconfig.ErrPostCommit):
		return apperror.New("SYSTEM_CONFIG_COMMITTED_WITH_CACHE_ERROR", "error.system_config_committed_with_cache_error", "配置已保存，但缓存刷新失败", http.StatusServiceUnavailable, err)
	default:
		return err
	}
}

type systemConfigRequest struct {
	Key         string                          `json:"key"`
	Value       json.RawMessage                 `json:"value" binding:"required"`
	ValueType   generated.SystemConfigValueType `json:"value_type" binding:"required"`
	IsPublic    *bool                           `json:"is_public" binding:"required"`
	Description *string                         `json:"description"`
}

func systemConfigInput(key string, value json.RawMessage, valueType generated.SystemConfigValueType, public bool, description *string) (systemconfig.Input, error) {
	input := systemconfig.Input{Key: key, Value: append(json.RawMessage(nil), value...), ValueType: systemconfig.ValueType(valueType), IsPublic: public}
	if description != nil {
		input.Description = *description
	}
	return input, nil
}

func systemConfigResponse(item systemconfig.Config) (generated.SystemConfig, error) {
	if !json.Valid(item.Value) {
		return generated.SystemConfig{}, fmt.Errorf("解析动态配置值失败")
	}
	return generated.SystemConfig{Key: item.Key, Value: json.RawMessage(item.Value), ValueType: generated.SystemConfigValueType(item.ValueType), IsPublic: item.IsPublic, Description: item.Description, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}, nil
}

func decodeSystemConfigValue(raw json.RawMessage) (any, error) {
	if !json.Valid(raw) {
		return nil, fmt.Errorf("解析动态配置值失败")
	}
	return json.RawMessage(raw), nil
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
