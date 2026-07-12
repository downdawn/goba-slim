# GoBA Slim

[![CI](https://github.com/downdawn/goba-slim/actions/workflows/ci.yml/badge.svg)](https://github.com/downdawn/goba-slim/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

GoBA Slim 是一个面向 Go HTTP 服务的精简工程内核。它提供显式组合根、强类型配置、PostgreSQL 数据边界、用户模块、安全 HTTP 边界和可执行的质量门禁，供后续业务模块复用。

项目当前已完成 **Phase 3**：PostgreSQL 用户模块、Redis、EdDSA Access Token、不透明 Refresh Token、会话撤销和受保护的用户 API 已形成完整闭环。

## 核心能力

- 默认值、YAML、`.env` 和 `GOBA_` 环境变量组成的强类型配置链路。
- Secret 明文与 `_FILE` 文件来源互斥，日志和 CLI 输出默认脱敏。
- `slog` 结构化日志、请求 ID、安全响应头、CORS、请求体限制和统一错误响应。
- Gin HTTP Server 与 OpenAPI 契约生成，提供存活、就绪和开发文档端点。
- 模块 Manifest、依赖排序、循环检测、启动、停止和健康检查生命周期。
- pgx 连接池、Schema 版本检查、显式初始化 SQL、sqlc 查询和真实 PostgreSQL 集成测试。
- UUIDv7 用户、Argon2id 密码、稳定分页、事务边界与唯一可用超级管理员保护。
- Redis 会话、短期 EdDSA JWT、Refresh Token 原子轮换、重用检测、登录限流和安全 Cookie。
- Cobra CLI、单元测试、竞态测试、静态检查、漏洞扫描、CI 和非 root 容器镜像。

## 快速开始

### 从源码运行

要求 Go 1.26+；[Task](https://taskfile.dev/) 用于统一跨平台命令。

```bash
git clone https://github.com/downdawn/goba-slim.git
cd goba-slim
task init
task db:init
task run
```

`task init` 创建 `.env` 和 `configs/config.local.yaml`。任一文件已存在时任务会拒绝覆盖。运行 `task db:init` 前，需要在本地配置中填写 PostgreSQL 地址、数据库、用户，并通过 `GOBA_DATABASE_PASSWORD` 或 `GOBA_DATABASE_PASSWORD_FILE` 提供密码；目标数据库必须为空。

没有 Task 时可以直接运行：

```bash
go run ./cmd/goba db init --yes --config configs/config.local.yaml --load-dotenv
go run ./cmd/goba user create-admin --username admin --password-file /path/to/password --config configs/config.local.yaml --load-dotenv
go run ./cmd/goba serve --config configs/config.local.yaml --load-dotenv
```

服务默认监听 `http://localhost:8000`：

- `GET /livez`：进程存活检查。
- `GET /readyz`：依赖和模块就绪检查。
- `GET /docs`：开发环境 API 文档。
- `GET /openapi.json`：OpenAPI 契约。

认证和用户 API 使用 `/api/v1` 前缀，主要包括登录、刷新、登出、密码策略、`/me` 和超级管理员用户管理。Refresh Token 仅通过 HttpOnly Cookie 传递，Access Token 使用 Bearer Header；完整契约见 `/docs`。

### 使用 Docker

先在当前 Shell 中安全设置 `GOBA_DATABASE_PASSWORD`、`GOBA_REDIS_PASSWORD` 和 PEM 格式的 `GOBA_AUTH_PRIVATE_KEY`，不要把实际值写入仓库。首次启动显式初始化数据库：

```bash
docker compose up --detach --wait postgres redis
docker compose --profile tools run --build --rm db-init
docker compose up --build --wait
```

服务启动后访问 `http://localhost:8000`。停止并删除容器：

```bash
docker compose down
```

通过 `GOBA_PORT` 可以修改宿主端口，例如 `GOBA_PORT=18000 docker compose up --build --wait`。PowerShell 使用 `$env:GOBA_PORT=18000` 设置当前终端环境变量。

Compose 编排 GoBA Slim、PostgreSQL 与 Redis；`db-init` 只在 `tools` Profile 下由部署方显式执行，`serve` 不会自动修改 Schema。镜像以非 root 用户运行，生产 TLS 应由 Nginx、Ingress 或 API Gateway 终止。

常用数据库命令：

```bash
goba db status --config configs/config.local.yaml --load-dotenv
goba db init --yes --config configs/config.local.yaml --load-dotenv
goba user create-admin --username admin --password-file /path/to/password --config configs/config.local.yaml --load-dotenv
```

管理员密码只能通过安全终端或文件读取，不能作为明文命令行参数。

## 配置

配置优先级为：内置默认值 → YAML 文件 → `.env`（仅显式启用）→ `GOBA_` 环境变量。列表型环境变量使用英文逗号分隔，例如：

```text
GOBA_CORS_ALLOW_ORIGINS=https://app.example.com,https://admin.example.com
```

`.env` 只用于本地开发，默认不会自动读取。Secret 不得提交、记录或放入镜像；生产环境建议通过挂载文件使用 `_FILE` 配置。生产环境还必须设置 `GOBA_APP_ENVIRONMENT=production`，并保持 debug 和 API 文档关闭。

PostgreSQL 支持 16+。development/test 可以显式使用 `ssl_mode: disable` 连接本机数据库；production 必须启用数据库 TLS。应用启动只检查连接和 Schema 版本，版本缺失或不匹配时拒绝启动。

认证使用 Ed25519 PKCS#8 PEM 私钥。Access Token 默认 15 分钟，每次请求同时校验 Redis 会话；Refresh Token 通过 HttpOnly Cookie 保存并在刷新时原子轮换。production 必须启用 Cookie Secure、数据库 TLS 和 Redis TLS。

完整示例见 [`configs/config.example.yaml`](configs/config.example.yaml) 和 [`.env.example`](.env.example)。

## 开发与验证

```bash
task generate:check
task test
task test:integration
task lint
task check
task compose:up
task compose:down
```

`api/openapi/openapi.yaml` 是 HTTP 契约事实来源，`db/schema` 与 `db/queries` 是数据库事实来源；OpenAPI 和 sqlc 生成代码均不得手工修改。工程边界、完整命令和测试要求见 [`docs/development.md`](docs/development.md)。

架构边界见 [`docs/architecture.md`](docs/architecture.md)，后续阶段见 [`docs/roadmap.md`](docs/roadmap.md)。

## 项目结构

```text
api/openapi/                 OpenAPI 契约与生成代码
cmd/goba/                    进程入口
configs/                     非敏感示例配置
db/schema/                   显式 Schema SQL
db/queries/                  sqlc 查询 SQL
db/generated/                sqlc 生成代码
deployments/                 容器构建定义
internal/app/                应用组合根与生命周期
internal/module/             模块声明与运行时
internal/modules/user/        用户领域、应用服务与 PostgreSQL 适配器
internal/modules/auth/        JWT、认证服务与 Redis 会话适配器
internal/platform/           配置、数据库、Redis、日志、HTTP 和健康检查
internal/transport/httpapi/  Gin 与 OpenAPI 传输边界
tests/                       真实基础设施集成测试
```

## 参与项目

提交代码前请阅读 [`CONTRIBUTING.md`](CONTRIBUTING.md)。安全问题请按 [`SECURITY.md`](SECURITY.md) 私下报告，不要在公开 Issue 中披露漏洞细节或 Secret。

## 致谢

GoBA Slim 的产品范围和架构取舍参考了以下开源项目：

- [fastapi-practices/fba-slim](https://github.com/fastapi-practices/fba-slim)
- [fastapi-practices/fastapi-best-architecture](https://github.com/fastapi-practices/fastapi-best-architecture)

GoBA Slim 是独立实现的非官方项目，与上述项目不存在隶属或官方移植关系。

## 许可证

本项目采用 [Apache License 2.0](LICENSE)。
