# 开发说明

## 前置条件

- Go 1.26
- Task（可选）
- Docker（可选；本地无法运行时由 CI 或目标环境补验）

## 常用命令

```bash
task init
task generate:check
task format
task lint
task test
task test:race
task vuln
task check
```

Windows 未将 Go 加入 `PATH` 时，可用完整路径执行 `go` 命令；Taskfile 与 CI 均假设 `go` 已在 `PATH`，以支持 Windows、macOS 和 Linux。

## 本地配置初始化

执行 `task init` 会从 `.env.example` 和 `configs/config.example.yaml` 生成 `.env` 与 `configs/config.local.yaml`。目标文件已存在时任务会失败，不会覆盖本地修改。启动本地服务使用：

```bash
task run
```

本地文件已被 Git 忽略；部署环境必须通过配置文件、Secret 挂载和环境变量提供实际配置。

## OpenAPI 与 HTTP 契约

`api/openapi/openapi.yaml` 是 HTTP 契约事实来源。修改后执行 `task generate`，生成的 `api/openapi/generated/openapi.gen.go` 必须提交，且 `task generate:check` 与 CI 必须无差异。不得手工修改生成代码。

Gin 仅存在于 `internal/platform/httpserver` 与 `internal/transport/httpapi` 边界；业务模块和共享包只能使用标准 `context.Context`，不得依赖 Gin、pgx、sqlc 或 go-redis 的具体类型。

## 模块与错误处理

模块按实际职责拆分：只有出现明确责任时才建立文件或子目录，不建立无边界的 `utils`、`helpers` 包。模块必须通过 Manifest 声明依赖；启动和停止使用可选生命周期接口。

业务错误统一使用 `internal/shared/apperror` 表达，并只在 HTTP 边界映射为响应。未知错误不得泄露堆栈、数据库信息、路径、Token、Cookie、私钥或配置内容。

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

Docker 镜像仅承载 HTTP 服务，TLS 证书由宿主 Nginx、Ingress 或 API Gateway 管理。当前机器若缺少 Docker，不得宣称已完成容器构建或运行验证；应由 CI 或目标环境执行：

```bash
docker build -f deployments/Dockerfile -t goba-slim:foundation .
docker run --rm -p 8000:8000 -v /path/to/config.yaml:/etc/goba/config.yaml:ro goba-slim:foundation
```
