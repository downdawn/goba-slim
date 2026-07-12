# GoBA Slim

[![CI](https://github.com/downdawn/goba-slim/actions/workflows/ci.yml/badge.svg)](https://github.com/downdawn/goba-slim/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

GoBA Slim 是一个面向 Go HTTP 服务的精简工程内核。它提供显式组合根、强类型配置、模块生命周期、安全 HTTP 边界和可执行的质量门禁，供后续业务模块复用。

项目当前处于 **Phase 1**：基础内核可运行、可测试、可构建为容器，但还不是完整的 RBAC 后端。PostgreSQL、Redis、用户、认证、文件和 systemconfig 等运行时能力将在后续阶段按明确契约加入。

## 核心能力

- 默认值、YAML、`.env` 和 `GOBA_` 环境变量组成的强类型配置链路。
- Secret 明文与 `_FILE` 文件来源互斥，日志和 CLI 输出默认脱敏。
- `slog` 结构化日志、请求 ID、安全响应头、CORS、请求体限制和统一错误响应。
- Gin HTTP Server 与 OpenAPI 契约生成，提供存活、就绪和开发文档端点。
- 模块 Manifest、依赖排序、循环检测、启动、停止和健康检查生命周期。
- Cobra CLI、单元测试、竞态测试、静态检查、漏洞扫描、CI 和非 root 容器镜像。

## 快速开始

### 从源码运行

要求 Go 1.26+；[Task](https://taskfile.dev/) 用于统一跨平台命令。

```bash
git clone https://github.com/downdawn/goba-slim.git
cd goba-slim
task init
task run
```

`task init` 创建 `.env` 和 `configs/config.local.yaml`。任一文件已存在时任务会拒绝覆盖。

没有 Task 时可以直接运行：

```bash
go run ./cmd/goba serve --config configs/config.example.yaml
```

服务默认监听 `http://localhost:8000`：

- `GET /livez`：进程存活检查。
- `GET /readyz`：依赖和模块就绪检查。
- `GET /docs`：开发环境 API 文档。
- `GET /openapi.json`：OpenAPI 契约。

### 使用 Docker

macOS / Linux：

```bash
docker build -f deployments/Dockerfile -t goba-slim:local .
docker run --rm -p 8000:8000 \
  -v "$(pwd)/configs/config.example.yaml:/etc/goba/config.yaml:ro" \
  goba-slim:local
```

Windows PowerShell：

```powershell
docker build -f deployments/Dockerfile -t goba-slim:local .
docker run --rm -p 8000:8000 `
  -v "${PWD}/configs/config.example.yaml:/etc/goba/config.yaml:ro" `
  goba-slim:local
```

镜像以非 root 用户运行；生产 TLS 应由 Nginx、Ingress 或 API Gateway 终止。

## 配置

配置优先级为：内置默认值 → YAML 文件 → `.env`（仅显式启用）→ `GOBA_` 环境变量。列表型环境变量使用英文逗号分隔，例如：

```text
GOBA_CORS_ALLOW_ORIGINS=https://app.example.com,https://admin.example.com
```

`.env` 只用于本地开发，默认不会自动读取。Secret 不得提交、记录或放入镜像；生产环境建议通过挂载文件使用 `_FILE` 配置。生产环境还必须设置 `GOBA_APP_ENVIRONMENT=production`，并保持 debug 和 API 文档关闭。

完整示例见 [`configs/config.example.yaml`](configs/config.example.yaml) 和 [`.env.example`](.env.example)。

## 开发与验证

```bash
task generate:check
task test
task lint
task check
```

`api/openapi/openapi.yaml` 是 HTTP 契约事实来源，生成代码不得手工修改。工程边界、完整命令和测试要求见 [`docs/development.md`](docs/development.md)。

架构边界见 [`docs/architecture.md`](docs/architecture.md)，后续阶段见 [`docs/roadmap.md`](docs/roadmap.md)。

## 项目结构

```text
api/openapi/                 OpenAPI 契约与生成代码
cmd/goba/                    进程入口
configs/                     非敏感示例配置
deployments/                 容器构建定义
internal/app/                应用组合根与生命周期
internal/module/             模块声明与运行时
internal/platform/           配置、日志、HTTP 和健康检查
internal/transport/httpapi/  Gin 与 OpenAPI 传输边界
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
