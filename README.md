# GoBA Slim

[![CI](https://github.com/downdawn/goba-slim/actions/workflows/ci.yml/badge.svg)](https://github.com/downdawn/goba-slim/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

GoBA Slim 是一个面向 Go HTTP 服务的模块化单体工程内核。它提供显式组合根、强类型配置、PostgreSQL 用户模块、Redis 认证会话、OpenAPI 契约和完整质量门禁，适合作为新业务服务的可靠起点。

当前已完成 Phase 1-3：工程内核、用户模块和认证会话均可运行。后续阶段见[路线图](docs/roadmap.md)。

## 特性

- Gin HTTP 边界与 OpenAPI 生成代码，统一错误、健康检查和安全响应头。
- PostgreSQL 16+、pgx、sqlc、显式 Schema 初始化和版本检查。
- UUIDv7 用户、Argon2id 密码、事务边界与超级管理员保护。
- Redis 会话、EdDSA Access Token、Refresh Token 轮换和重用检测。
- 强类型配置、Secret 与 `_FILE` 双来源、默认脱敏日志和 Cobra CLI。
- 单元测试、真实基础设施集成测试、竞态检查、静态检查和漏洞扫描。

## 快速开始

需要 Go 1.26+ 和 [Task](https://taskfile.dev/)。使用已有或托管的 PostgreSQL/Redis 时不需要 Docker；请先创建目标空数据库，并在本地配置中填写连接信息。

```bash
git clone https://github.com/downdawn/goba-slim.git
cd goba-slim
task setup
task db:init
task run
```

`task setup` 幂等创建缺失的 `.env`、`configs/config.local.yaml` 和 Ed25519 PKCS#8 私钥，不覆盖已有文件，也不输出 Secret。`task db:init` 只初始化当前配置指向的数据库 Schema，不创建数据库实例或数据库。

服务默认监听 `http://localhost:8000`：

- API 文档：`GET /docs`
- 存活检查：`GET /livez`
- 就绪检查：`GET /readyz`
- OpenAPI：`GET /openapi.json`

需要项目同时提供 PostgreSQL、Redis 和 API 时，全新克隆后也可以一行启动完整 Compose 环境；该命令会先补齐缺失的本地配置和 Secret：

```bash
task compose:up
```

## 开发

```bash
task dev:up          # 只启动 PostgreSQL 与 Redis
task db:init         # 初始化当前配置指向的 Schema
task run             # 在宿主机启动 API
task test            # 单元测试
task test:integration
task check           # 完整本地验收
task dev:down
```

独立生成本地认证私钥：

```bash
task auth:keygen
```

完整命令、Schema 管理和工程边界见[开发说明](docs/development.md)。

## 部署

生产交付物是单一非 root API 镜像。PostgreSQL 与 Redis 作为外部依赖，通过环境变量或挂载配置提供地址，通过平台 Secret 或 `_FILE` 变量提供密码和私钥。镜像不创建数据库，也不在启动时修改 Schema。

生产环境要求、容器启动示例和配置清单见[部署说明](docs/deployment.md)。本仓库的 `compose.yaml` 只用于本地开发和验收，不是生产编排模板。

## 文档

- [架构说明](docs/architecture.md)
- [开发说明](docs/development.md)
- [部署说明](docs/deployment.md)
- [阶段路线图](docs/roadmap.md)
- [贡献指南](CONTRIBUTING.md)
- [安全策略](SECURITY.md)

`api/openapi/openapi.yaml` 是 HTTP 契约事实来源，`db/schema` 和 `db/queries` 中的 SQL 是数据库事实来源；生成代码不得手工修改。

## 致谢

项目的产品范围和架构取舍参考了 [fba-slim](https://github.com/fastapi-practices/fba-slim) 与 [fastapi-best-architecture](https://github.com/fastapi-practices/fastapi-best-architecture)。GoBA Slim 是独立实现的非官方项目。

## 许可证

[Apache License 2.0](LICENSE)
