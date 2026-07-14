# GoBA Slim

[![CI](https://github.com/downdawn/goba-slim/actions/workflows/ci.yml/badge.svg)](https://github.com/downdawn/goba-slim/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.26.5%2B-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

GoBA Slim 是用于克隆创建新 Go HTTP 服务的模块化单体基线。它提供显式组合根、类型安全配置、PostgreSQL、Redis 认证会话、OpenAPI 契约和基础质量检查，目标是让新业务可以直接开始开发，而不是先搭建插件平台。

## 特性

- Gin HTTP 边界与 OpenAPI 生成代码，统一错误、健康检查、安全响应头和运行时请求校验。
- PostgreSQL 16+、pgx、sqlc 与按序 SQL 迁移。
- UUIDv7 用户、Argon2id 密码、事务边界、超级管理员保护和登录后密码参数升级。
- Redis 会话、EdDSA Access Token、Refresh Token 轮换、重用检测和会话管理接口。
- 严格强类型配置、Secret 与 `_FILE` 双来源、默认脱敏日志和 Cobra CLI。
- 内置但默认关闭的文件与动态配置能力，不需要插件注册表。

## 快速开始

需要 Go 1.26.5+、[Task](https://taskfile.dev/) 和 Docker Desktop。克隆后直接运行：

```bash
git clone https://github.com/downdawn/goba-slim.git
cd goba-slim
task setup
task dev:up
task run
```

`task setup` 只创建缺失的 `.env`、`configs/config.local.yaml` 和 Ed25519 私钥，不覆盖已有文件，也不输出 Secret。`task run` 会先显式执行 `goba db migrate`，再启动服务；`goba serve` 本身永远不会修改数据库。

服务默认监听 `http://localhost:8000`：

- API 文档：`GET /docs`
- 存活检查：`GET /livez`
- 就绪检查：`GET /readyz`
- OpenAPI：`GET /openapi.json`

已有或托管 PostgreSQL、Redis 时不需要 Docker。先创建一个空数据库，配置连接信息后执行：

```bash
task setup
task db:migrate
task run
```

现有的非 GoBA 数据库不会被接管。开发中的旧数据库可删除后重新创建；从现在开始的数据库结构变化一律新增迁移文件，详见 [SQL 管理](docs/sql.md)。

完整 Compose 环境可直接启动：

```bash
task compose:up
```

其中一次性 `db-migrate` 服务完成迁移，API 只负责运行。

## 内置可选能力

克隆出的项目已经包含 `file` 和 `systemconfig` 源码。它们不是需要下载安装的插件：打开配置开关即可使用，不需要删除或新增其他配置。

```yaml
modules:
  file: true
  systemconfig: true
file:
  storage_path: var/uploads
systemconfig:
  cache_ttl: 5m
```

文件模块还需要 `file.storage_path` 可写；动态配置使用已经包含在基线迁移中的表。关闭开关时相关 HTTP 路由不会注册。项目业务模块直接放在 `internal/modules/<name>`，并在 `internal/app` 显式装配，见 [模块开发](docs/modules.md)。

## 开发

```bash
task generate        # 重新生成 OpenAPI 与 sqlc 代码
task test            # 快速单元测试，不需要 Docker 或 GCC
task check           # 重新生成契约代码、格式、vet、lint、单测和构建
task check:full      # check 加 Testcontainers 集成测试与漏洞扫描
task test:race       # Linux CI 执行；Windows 无 GCC 时无需本地运行
```

根目录的 [`run.go`](run.go) 是 GoLand 入口，固定执行“迁移后启动”，与 `task run` 保持一致。

## 文档

- [架构说明](docs/architecture.md)
- [开发说明](docs/development.md)
- [部署说明](docs/deployment.md)
- [配置参考](docs/configuration.md)
- [模块开发](docs/modules.md)
- [API 流程](docs/api.md)
- [SQL 管理](docs/sql.md)
- [测试说明](docs/testing.md)
- [性能与 PGO](docs/performance.md)
- [常见问题](docs/faq.md)
- [阶段路线图](docs/roadmap.md)
- [贡献指南](CONTRIBUTING.md)
- [安全策略](SECURITY.md)

`api/openapi/openapi.yaml` 是 HTTP 契约事实来源；`db/migrations` 中按序 SQL 是数据库结构演进事实来源；`db/queries` 是查询事实来源。生成代码不得手工修改。

## 致谢

项目的产品范围和架构取舍参考了 [fba-slim](https://github.com/fastapi-practices/fba-slim) 与 [fastapi-best-architecture](https://github.com/fastapi-practices/fastapi-best-architecture)。GoBA Slim 是独立实现的非官方项目。

## 许可证

[Apache License 2.0](LICENSE)
