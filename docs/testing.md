# 测试说明

## 执行策略

`task test:race` 用于检测并发访问中的数据竞态。日常小范围修改可不在本地跑全量 Race；涉及 goroutine、缓存、事件或共享状态的变更，以及合并前验收，应执行。CI 会始终覆盖 Race。

通常最耗时的检查是集成测试、Race、Docker 镜像构建与完整 Compose 启动、漏洞扫描。首次运行还会受 Go 依赖下载、Docker 镜像拉取和缓存预热影响。

- `task test`：单元与 HTTP 契约测试。
- `task test:integration`：Testcontainers PostgreSQL、Redis 与端到端认证测试。
- `task test:race`：竞态检测。
- `task lint`、`task vet`、`task vuln`：静态检查与漏洞扫描。
- `task check`：本地完整验收。

Repository 不使用 SQLite 模拟 PostgreSQL。修复缺陷应先添加回归测试；文件名、路径、MIME、Token 和配置值等不可信输入应有 Fuzz 覆盖。本机无法运行 Docker 或 Race 时必须在变更说明中记录，由 CI 补验。
