# 开发说明

## 环境要求

- Go 1.26.5+
- Task
- Docker Desktop（推荐，用于本地 PostgreSQL、Redis 与集成测试）

项目工具通过 Go `tool` directive 固定版本，无需全局安装 sqlc、oapi-codegen、golangci-lint 或 govulncheck。Windows 不需要安装 GCC；Race 在 Linux CI 运行。

## 首次启动

推荐让依赖在 Docker 中运行，API 在宿主机运行：

```bash
task setup
task dev:up
task run
```

`task setup` 幂等创建 `.env`、`configs/config.local.yaml` 和 `configs/auth-private.local.pem`。已有文件不会被覆盖，且这些本地文件均被 Git 忽略。

`task run` 等价于先执行：

```bash
task db:migrate
go run ./cmd/goba serve --config configs/config.local.yaml --load-dotenv
```

这是本地便利流程，不是服务自动迁移：单独执行的 `goba serve` 只检查 Schema，遇到未迁移或版本不符会拒绝启动。已有或托管基础设施时，创建一个空数据库、填写连接配置后直接执行 `task db:migrate`。

## 常用命令

```bash
task generate          # OpenAPI 与 sqlc 生成
task generate:check    # 检查生成物漂移
task db:migrate        # 显式执行数据库迁移
task test              # 快速测试
task check             # 快速完整检查
task check:full        # 加集成测试与漏洞扫描
task test:race         # Linux CI 运行
task dev:down
```

直接调用 CLI 时使用同一份本地配置：

```bash
go run ./cmd/goba db status --config configs/config.local.yaml --load-dotenv
go run ./cmd/goba user create-admin --username admin --config configs/config.local.yaml --load-dotenv
```

管理员密码通过交互式终端读取；非交互环境使用权限受控的 `--password-file`，不要把密码放在命令行参数中。

## GoLand

根目录的 `run.go` 是 IDE 专用入口。右键运行或调试它会按“迁移后启动”的顺序调用同一套 CLI 与组合根。Working directory 必须是仓库根目录。

## 内置可选能力

文件与动态配置模块的代码已在仓库中。只需在 `configs/config.local.yaml` 中修改开关：

```yaml
modules:
  file: true
  systemconfig: true
```

文件模块需要可写的 `file.storage_path`；动态配置使用 PostgreSQL 与 Redis。关闭模块时不注册对应路由，不需要删除表、代码或其他配置。详情见 [模块开发](modules.md)。

## Compose

完整 Compose 环境：

```bash
task compose:up
task compose:down
```

Compose 启动 PostgreSQL、Redis、一次性 `db-migrate` 和 API。`db-migrate` 成功后 API 才启动。Windows Docker Desktop 直接可用；Linux 和 macOS 的 `task compose:up` 会传入当前用户 UID/GID，以便非 root 容器读取本地私钥文件。

## 数据库与生成代码

新表或改表时，新增 `db/migrations/NNNNNN_name.sql`，然后运行 `task generate` 和 `task db:migrate`。不要修改已经应用的迁移，也不要在 Handler 或服务启动代码中建表。迁移版本由数据库中的 `goba_schema_version` 保存。具体规则见 [SQL 管理](sql.md)。

`api/openapi/openapi.yaml` 是 HTTP 契约事实来源，`db/queries` 是查询事实来源。修改契约后重新生成 `api/openapi/generated` 和 `db/generated`，生成代码不得手工编辑。

## 工程边界

- Gin 只存在于 `internal/platform/httpserver` 和 `internal/transport/httpapi`。
- 业务模块不依赖 Gin、pgx、sqlc 或 go-redis 的具体类型。
- 业务错误通过 `internal/shared/apperror` 表达，只在 HTTP 边界映射响应。
- 新模块在 `internal/app` 显式装配，不通过 Manifest、注册表或字符串 Service Locator。
- Secret、Token、Cookie、私钥和 Authorization 信息不得进入日志或错误响应。
