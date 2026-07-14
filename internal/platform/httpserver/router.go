// Package httpserver 提供 Gin 路由及其 HTTP 安全边界。
package httpserver

import (
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/downdawn/goba-slim/api/openapi/generated"
	"github.com/downdawn/goba-slim/internal/platform/config"
	"github.com/downdawn/goba-slim/internal/shared/apperror"
	"github.com/downdawn/goba-slim/internal/transport/httpapi"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/gin-gonic/gin"
	ginmiddleware "github.com/oapi-codegen/gin-middleware"
)

// Options 集中传入路由所需的已构造依赖，避免 HTTP 边界读取全局配置。
type Options struct {
	Config  config.Config
	Handler *httpapi.Handler
	Logger  *slog.Logger
}

// NewRouter 创建仅包含当前 HTTP 契约和安全边界的 Gin 路由。
func NewRouter(options Options) *gin.Engine {
	if options.Handler == nil {
		panic("httpserver: Handler must not be nil")
	}
	if options.Logger == nil {
		options.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	router := gin.New()
	if err := router.SetTrustedProxies(options.Config.Server.TrustedProxies); err != nil {
		panic("httpserver: invalid trusted proxies")
	}

	router.Use(
		recoveryMiddleware(options.Logger),
		requestIDMiddleware(),
		securityHeadersMiddleware(),
		corsMiddleware(options.Config.CORS),
		bodyLimitMiddleware(defaultMaxBodyBytes, fileUploadBodyLimit(options.Config)),
		accessLogMiddleware(options.Logger),
		timeoutMiddleware(options.Config.Server.ReadTimeout),
	)

	api := router.Group("")
	api.Use(openAPIValidationMiddleware())
	generated.RegisterHandlersWithOptions(optionalModuleRouter{IRouter: api, files: options.Config.Modules.File, systemConfig: options.Config.Modules.SystemConfig}, options.Handler, generated.GinServerOptions{ErrorHandler: func(ctx *gin.Context, err error, _ int) {
		httpapi.WriteError(ctx, apperror.Validation("INVALID_REQUEST", "error.invalid_request", "请求参数无效", err))
	}})
	if options.Config.App.Environment != "production" && options.Config.App.DocsEnabled {
		registerDocumentation(router)
	}
	return router
}

func openAPIValidationMiddleware() gin.HandlerFunc {
	specification, err := generated.GetSpec()
	if err != nil {
		panic("httpserver: invalid embedded OpenAPI specification")
	}
	specification.Servers = nil
	options := ginmiddleware.Options{
		ErrorHandler: openAPIValidationError,
		Options: openapi3filter.Options{
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
		},
	}
	validator := ginmiddleware.OapiRequestValidatorWithOptions(specification, &options)
	uploadOptions := options
	uploadOptions.Options.ExcludeRequestBody = true
	uploadValidator := ginmiddleware.OapiRequestValidatorWithOptions(specification, &uploadOptions)
	return func(ctx *gin.Context) {
		if ctx.Request.Method == http.MethodPost && ctx.Request.URL.Path == "/api/v1/files" {
			uploadValidator(ctx)
			return
		}
		validator(ctx)
	}
}

func openAPIValidationError(ctx *gin.Context, _ string, statusCode int) {
	if statusCode == http.StatusNotFound {
		ctx.AbortWithStatus(statusCode)
		return
	}
	httpapi.WriteError(ctx, apperror.Validation("INVALID_REQUEST", "error.invalid_request", "请求参数无效", nil))
}

type optionalModuleRouter struct {
	gin.IRouter
	files        bool
	systemConfig bool
}

func (r optionalModuleRouter) GET(path string, handlers ...gin.HandlerFunc) gin.IRoutes {
	if !r.files && path == "/files/:ownerId/:fileName" {
		return r
	}
	if !r.systemConfig && strings.HasPrefix(path, "/api/v1/system-configs") {
		return r
	}
	return r.IRouter.GET(path, handlers...)
}

func (r optionalModuleRouter) POST(path string, handlers ...gin.HandlerFunc) gin.IRoutes {
	if !r.files && path == "/api/v1/files" {
		return r
	}
	if !r.systemConfig && strings.HasPrefix(path, "/api/v1/system-configs") {
		return r
	}
	return r.IRouter.POST(path, handlers...)
}

func (r optionalModuleRouter) DELETE(path string, handlers ...gin.HandlerFunc) gin.IRoutes {
	if !r.files && path == "/api/v1/files/:ownerId/:fileName" {
		return r
	}
	if !r.systemConfig && strings.HasPrefix(path, "/api/v1/system-configs") {
		return r
	}
	return r.IRouter.DELETE(path, handlers...)
}

func (r optionalModuleRouter) PUT(path string, handlers ...gin.HandlerFunc) gin.IRoutes {
	if !r.systemConfig && strings.HasPrefix(path, "/api/v1/system-configs") {
		return r
	}
	return r.IRouter.PUT(path, handlers...)
}

func fileUploadBodyLimit(cfg config.Config) int64 {
	if !cfg.Modules.File {
		return defaultMaxBodyBytes
	}
	limit := cfg.File.ImageMaxBytes
	if cfg.File.VideoEnabled && cfg.File.VideoMaxBytes > limit {
		limit = cfg.File.VideoMaxBytes
	}
	return limit + multipartOverheadBytes
}

const swaggerUIHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>GoBA Slim API</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({
      url: "/openapi.json",
      dom_id: "#swagger-ui",
      presets: [SwaggerUIBundle.presets.apis],
      layout: "BaseLayout"
    });
  </script>
</body>
</html>`

func registerDocumentation(router *gin.Engine) {
	router.GET("/openapi.json", func(ctx *gin.Context) {
		spec, err := generated.GetSpecJSON()
		if err != nil {
			httpapi.WriteError(ctx, err)
			return
		}
		ctx.Data(http.StatusOK, "application/json; charset=utf-8", spec)
	})
	router.GET("/docs", func(ctx *gin.Context) {
		ctx.Data(http.StatusOK, "text/html; charset=utf-8", []byte(swaggerUIHTML))
	})
	router.GET("/favicon.ico", func(ctx *gin.Context) {
		ctx.Status(http.StatusNoContent)
	})
}
