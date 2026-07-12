# GoBA Slim

GoBA Slim 是面向 Go 服务的工程内核：以显式组合根、强类型配置、模块生命周期和安全 HTTP 边界为基础，供后续业务模块复用。

## 当前 Phase 1 能力

- 强类型配置：默认值、YAML 与 `GOBA_` 环境变量按顺序覆盖；Secret 支持明文或 `_FILE` 文件来源（二者互斥）。
- `slog` 结构化日志、敏感字段脱敏、请求 ID、统一 HTTP 错误响应。
- Gin HTTP Server、OpenAPI 契约与 `/livez`、`/readyz` 健康检查。
- 模块 Manifest、依赖排序、循环检测及启动/停止生命周期。
- Cobra CLI：`version`、`config check`、`config print --redact` 与 `serve`。
- OpenAPI 生成、测试、静态检查、漏洞扫描、CI 与 Docker 构建定义。

## 依赖

- Go 1.26
- [Task](https://taskfile.dev/)（可选，用于统一开发命令）
- Docker（可选，用于镜像构建；生产 TLS 由宿主 Nginx 或网关终止）

## 快速开始

### Windows PowerShell

```powershell
task init
& 'E:\Program Files\Go\bin\go.exe' run ./cmd/goba config check --config configs/config.local.yaml --load-dotenv
& 'E:\Program Files\Go\bin\go.exe' run ./cmd/goba serve --config configs/config.local.yaml --load-dotenv
```

### macOS / Linux

```bash
task init
go run ./cmd/goba config check --config configs/config.local.yaml --load-dotenv
go run ./cmd/goba serve --config configs/config.local.yaml --load-dotenv
```

示例配置监听 `0.0.0.0:8000`。启动后可访问：

- `GET /livez`：进程存活检查。
- `GET /readyz`：依赖和模块就绪检查。
- `GET /openapi.json`、`GET /docs`：仅 development 且 `docs_enabled: true` 时提供。

## 配置

配置优先级为：内置默认值 → YAML 文件 → `GOBA_` 环境变量。列表型环境变量使用英文逗号分隔，例如 `GOBA_CORS_ALLOW_ORIGINS=https://app.example.com,https://admin.example.com`。

执行 `task init` 会从 `.env.example` 和 `configs/config.example.yaml` 生成 `.env` 与 `configs/config.local.yaml`；任一目标文件已存在时任务会失败，绝不覆盖本地修改。`.env` 仅供本地开发，默认不会被程序自动读取；部署时通过编排平台注入环境变量。Secret 不得提交、记录或放入镜像。`GOBA_AUTH_PRIVATE_KEY` 和 `GOBA_AUTH_PRIVATE_KEY_FILE` 只能配置一个，生产部署建议通过挂载文件使用后者。

生产环境必须设置 `GOBA_APP_ENVIRONMENT=production`，并保持 `GOBA_APP_DEBUG=false` 与 `GOBA_APP_DOCS_ENABLED=false`。

## 常用命令

```bash
task init
task generate:check
task test
task lint
task check
```

没有 Task 时可执行等价命令：

```bash
go tool oapi-codegen --package generated --generate gin,types,embedded-spec -o api/openapi/generated/openapi.gen.go api/openapi/openapi.yaml
go test ./...
go vet ./...
go tool govulncheck ./...
go build -o bin/goba ./cmd/goba
```

## 目录说明

- `cmd/goba`：进程入口与信号处理。
- `internal/app`：Composition Root 与应用生命周期。
- `internal/platform`：配置、日志、HTTP Server、健康检查等平台能力。
- `internal/module`：模块声明、解析和生命周期。
- `internal/transport/httpapi`：Gin 与 OpenAPI 的 HTTP 传输边界。
- `api/openapi`：HTTP 契约及生成代码。
- `configs`：非敏感示例配置。
- `deployments`：容器构建定义。

## 设计与后续范围

工程规范见 `AGENTS.md`，开发约定见 `docs/development.md`。当前 Phase 1 不包含 PostgreSQL、Redis、用户、认证、文件或 systemconfig 的运行时实现；这些能力将在后续模块阶段按契约和显式 SQL 逐步加入。
