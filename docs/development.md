# 开发说明

## 环境要求

- Go 1.26+
- Task
- Docker 与 Docker Compose

Go 和 Task 需要位于 `PATH`。项目工具通过 Go `tool` directive 固定版本，无需全局安装 sqlc、oapi-codegen、golangci-lint 或 govulncheck。

## 首次启动

```bash
task setup
task dev:init
task run
```

`task setup` 是幂等操作，只创建缺失文件：

- `.env`：随机生成本地 PostgreSQL、Redis 密码，并引用本地私钥文件；
- `configs/config.local.yaml`：从非敏感示例配置复制；
- `configs/auth-private.local.pem`：Ed25519 PKCS#8 PEM 私钥。

这些文件均被 Git 忽略。已有文件不会被覆盖；需要轮换私钥时先自行移走旧文件，再执行 `task auth:keygen`。本地开发可以直接使用 `GOBA_DATABASE_PASSWORD` 和 `GOBA_REDIS_PASSWORD`，无需使用 `_FILE`。

`task dev:init` 会启动 PostgreSQL 与 Redis，并对空数据库执行显式初始化 SQL。它不是迁移命令，已初始化数据库不应重复执行。

## 日常开发

推荐让依赖运行在 Docker 中，API 运行在宿主机：

```bash
task dev:up
task run
```

常用命令：

```bash
task generate
task generate:check
task format
task format:check
task lint
task test
task test:integration
task test:race
task test:coverage
task vet
task vuln
task build
task check
task dev:down
```

`task run`、`task dev:init` 会显式加载仓库根目录的 `.env`。直接调用 CLI 时需要同时传入本地配置参数：

```bash
go run ./cmd/goba config check --config configs/config.local.yaml --load-dotenv
go run ./cmd/goba db status --config configs/config.local.yaml --load-dotenv
go run ./cmd/goba user create-admin --username admin --config configs/config.local.yaml --load-dotenv
```

管理员密码通过交互式终端读取；非交互环境必须使用权限受控的 `--password-file`，不能把密码写入命令行参数。

## 完整 Compose 验收

`compose.yaml` 默认只启动 PostgreSQL 与 Redis。`app` profile 增加 API，`tools` profile 提供显式数据库初始化：

```bash
task setup
task compose:init
task compose:up
task compose:down
```

该编排复用本地 `.env` 的随机密码和本地私钥文件，只用于开发及验收。修改宿主端口可设置 `GOBA_PORT`。
PostgreSQL 与 Redis 默认分别发布到宿主机 `5432` 和 `6379`，与本地应用配置保持一致；端口已被占用时，可在 `.env` 同时调整 `GOBA_DATABASE_PORT` 或 `GOBA_REDIS_PORT`。

## Schema 与生成代码

`db/schema` 是 Schema SQL 事实来源，`db/queries` 是查询事实来源。服务启动只检查 PostgreSQL 连接和 Schema 版本，不执行自动迁移。数据库变更必须提供显式 SQL，并由部署方按顺序执行。

`api/openapi/openapi.yaml` 是 HTTP 契约事实来源。修改 OpenAPI 或 SQL 后执行：

```bash
task generate
task generate:check
```

`api/openapi/generated` 与 `db/generated` 中的代码由工具生成，不得手工修改。

## 工程边界

- Gin 只存在于 `internal/platform/httpserver` 和 `internal/transport/httpapi`。
- 业务模块不依赖 Gin、pgx、sqlc 或 go-redis 的具体类型。
- 业务错误通过 `internal/shared/apperror` 表达，只在 HTTP 边界映射响应。
- 模块通过 Manifest 声明依赖，不建立无职责的空目录或通用工具包。
- Secret、Token、Cookie、私钥和 Authorization 信息不得进入日志或错误响应。

提交前至少执行：

```bash
task check
task test:race
task lint
git diff --check
```
