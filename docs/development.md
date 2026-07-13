# 开发说明

## 环境要求

- Go 1.26+
- Task
- Docker 与 Docker Compose

Go 和 Task 需要位于 `PATH`。项目工具通过 Go `tool` directive 固定版本，无需全局安装 sqlc、oapi-codegen、golangci-lint 或 govulncheck。

## 首次启动

```bash
task setup
task db:init
task run
```

`task setup` 是幂等操作，只创建缺失文件：

- `.env`：随机生成本地 PostgreSQL、Redis 密码，并引用本地私钥文件；
- `configs/config.local.yaml`：从非敏感示例配置复制；
- `configs/auth-private.local.pem`：Ed25519 PKCS#8 PEM 私钥。

这些文件均被 Git 忽略。已有文件不会被覆盖；需要轮换私钥时先自行移走旧文件，再执行 `task auth:keygen`。本地开发可以直接使用 `GOBA_DATABASE_PASSWORD` 和 `GOBA_REDIS_PASSWORD`，无需使用 `_FILE`。

PostgreSQL 实例和目标空数据库由开发者、运维或托管平台创建。`task db:init` 不启动 Docker，也不创建数据库；它只对当前配置指向的数据库执行 GoBA Schema 初始化。当前版本已经就绪时命令直接成功；存在未知表、缺失表或版本不匹配时拒绝执行。

## 日常开发

推荐让依赖运行在 Docker 中，API 运行在宿主机：

```bash
task dev:up
task db:init
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

`task run`、`task db:init` 会显式加载仓库根目录的 `.env`。直接调用 CLI 时需要同时传入本地配置参数：

```bash
go run ./cmd/goba config check --config configs/config.local.yaml --load-dotenv
go run ./cmd/goba db status --config configs/config.local.yaml --load-dotenv
go run ./cmd/goba user create-admin --username admin --config configs/config.local.yaml --load-dotenv
```

### GoLand 运行与调试

完成 `task setup` 和 `task db:init` 后，可以直接在 GoLand 中右键运行或调试仓库根目录的 `run.go`。该文件等价于：

```bash
goba serve --config configs/config.local.yaml --load-dotenv
```

它只提供 IDE 入口，实际启动仍复用 `internal/cli` 和应用组合根。GoLand 的 Working directory 必须是仓库根目录；默认配置通常已满足这一条件。

管理员密码通过交互式终端读取；非交互环境必须使用权限受控的 `--password-file`，不能把密码写入命令行参数。

## 完整 Compose 验收

`compose.yaml` 编排 PostgreSQL、Redis、一次性 `db-init` 和 API。API 只在 PostgreSQL 健康、Schema 初始化成功且 Redis 健康后启动：

```bash
task setup
task compose:up
task compose:down
```

`task compose:up` 会先执行幂等的 `task setup`，是全新克隆后完整环境的一键入口；重复执行不会重复建表。该编排复用本地 `.env` 的随机密码和本地私钥文件，适合开发、验收和小规模自托管；正式生产仍需按部署环境补齐 TLS、备份、监控和恢复策略。修改宿主端口可设置 `GOBA_PORT`。
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
