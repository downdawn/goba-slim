# 贡献指南

提交改动前，请先阅读当前公开需求、相关代码、配置和 [`AGENTS.md`](AGENTS.md)。

## 环境准备

- Go 1.26.5+
- Git
- Task（推荐）
- Docker（集成测试、Compose 或容器改动时需要）

首次运行：

```bash
task setup
task dev:up
task run
```

本地配置不得提交。`task setup` 只创建缺失文件，不会覆盖已有配置或私钥；`task run` 会在启动前显式执行迁移。

## 开发约定

- 保持改动聚焦，不预建空目录、空抽象或无边界的 `utils`、`helpers` 包。
- Gin 只用于 HTTP 平台与传输边界；业务层不得依赖 Gin、pgx、sqlc 或 go-redis 的具体类型。
- OpenAPI 是 HTTP 契约事实来源，生成代码不得手工修改。
- 数据库变更新增 `db/migrations/NNNNNN_name.sql`，不修改已应用迁移，不使用服务运行时自动迁移。
- 不得在代码、日志、错误响应、示例配置或镜像中加入 Secret。
- 行为变更必须同步测试、公开文档和示例配置。

详细约定见 [`docs/development.md`](docs/development.md)。

## 提交前检查

修改 Go 代码后至少执行：

```bash
task check
git diff --check
```

可使用 Docker 时再执行 `task check:full`。`task test:race` 由 Linux CI 强制执行；Windows 本机没有 GCC 时不要求安装编译器来运行它。修改 OpenAPI 或 SQL 后运行 `task generate` 并提交生成结果。修改 Docker 后还应验证 Compose 配置及 `/livez`、`/readyz`。

本机无法执行的检查必须在 Pull Request 中说明，并由 CI 或目标环境补验。

## 提交与 Pull Request

- 从最新的 `master` 创建短生命周期分支。
- 提交信息使用简洁英文 Conventional Commits，例如 `fix: handle invalid config`。
- 一个 Pull Request 只解决一个明确问题，并说明行为变化、验证结果和兼容性影响。
- 不要提交无关格式化、生成物漂移或本地 IDE 配置。

提交即表示你同意按本项目的 [Apache License 2.0](LICENSE) 许可你的贡献。
