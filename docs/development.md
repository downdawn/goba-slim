# 开发说明

## 前置条件

- Go 1.26
- Task（可选）
- Docker（修改或验证容器链路时需要）

## 常用命令

```bash
task init
task generate:check
task format
task lint
task test
task test:integration
task test:race
task vuln
task check
task compose:up
task compose:down
```

Windows 未将 Go 加入 `PATH` 时，可用完整路径执行 `go` 命令；Taskfile 与 CI 均假设 `go` 已在 `PATH`，以支持 Windows、macOS 和 Linux。

## 本地配置初始化

执行 `task init` 会从 `.env.example` 和 `configs/config.example.yaml` 生成 `.env` 与 `configs/config.local.yaml`。目标文件已存在时任务会失败，不会覆盖本地修改。启动本地服务使用：

```bash
task run
```

本地文件已被 Git 忽略；部署环境必须通过配置文件、Secret 挂载和环境变量提供实际配置。

## OpenAPI 与 HTTP 契约

`api/openapi/openapi.yaml` 是 HTTP 契约事实来源。`db/schema` 和 `db/queries` 是数据库事实来源。修改后执行 `task generate`，OpenAPI 与 sqlc 生成代码必须提交，且 `task generate:check` 与 CI 必须无差异。不得手工修改生成代码。

Gin 仅存在于 `internal/platform/httpserver` 与 `internal/transport/httpapi` 边界；业务模块和共享包只能使用标准 `context.Context`，不得依赖 Gin、pgx、sqlc 或 go-redis 的具体类型。

## 模块与错误处理

模块按实际职责拆分：只有出现明确责任时才建立文件或子目录，不建立无边界的 `utils`、`helpers` 包。模块必须通过 Manifest 声明依赖；启动和停止使用可选生命周期接口。

业务错误统一使用 `internal/shared/apperror` 表达，并只在 HTTP 边界映射为响应。未知错误不得泄露堆栈、数据库信息、路径、Token、Cookie、私钥或配置内容。

## PostgreSQL 与用户模块

数据库变更只在 `db/schema` 提供显式 SQL；`serve` 只检查连接和版本。首次初始化使用：

```bash
task db:init
```

sqlc 只在数据适配器中使用，生成类型不得进入业务服务。用户事务由应用服务通过 Unit of Work 控制，事务对象不得写入 `context.Context`。Repository 测试使用真实 PostgreSQL，不以 SQLite 或 SQL Mock 代替主要保证。

## Redis 与认证

Redis 是认证会话的必要依赖，业务服务只依赖 `SessionStore` 与 `RateLimiter`。Refresh Token 轮换和重用检测由内嵌 Lua 保证原子性；不得使用 `KEYS` 或在 Redis 保存明文 Token。认证集成测试覆盖并发刷新、重放撤销、Cookie、Origin、登出和 Redis 故障时的 fail closed 行为。

## 配置与 Secret

配置按默认值、YAML、`GOBA_` 环境变量覆盖。`.env` 不会自动加载，只允许本地开发显式选择加载；生产部署由平台注入环境变量。Secret 的明文值与 `_FILE` 来源互斥，日志和 CLI 输出必须保持脱敏。

## 测试与提交前检查

功能或行为变更遵循测试先行：先编写能失败的最小测试，再实现代码并运行相关包测试。提交前至少执行：

```bash
task check
task test:race
task lint
git diff --check
```

本地容器开发使用 Compose：

```bash
docker compose up --build --wait
docker compose down
```

Compose 包含应用、PostgreSQL 和 Redis。首次使用前在当前 Shell 设置数据库密码、Redis 密码和 Ed25519 私钥 Secret，再执行 `task compose:init`；后续使用 `task compose:up`。

镜像级构建和排障可直接执行：

```bash
docker build -f deployments/Dockerfile -t goba-slim:foundation .
docker run --rm -p 8000:8000 -v /path/to/config.yaml:/etc/goba/config.yaml:ro goba-slim:foundation
```

镜像以非 root 用户运行，TLS 证书由宿主 Nginx、Ingress 或 API Gateway 管理。本机缺少 Docker 时必须如实记录，并由 CI 或目标环境补验。
