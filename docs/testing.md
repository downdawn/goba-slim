# 测试说明

测试按耗时和环境依赖分层，日常开发先获得快速反馈，完整验证交给有 Docker 与 Linux 工具链的环境。

- `task test`：快速单元和 HTTP 契约测试，不生成代码、不启动 Docker，也不需要 GCC。
- `task check`：重新生成代码、格式、vet、lint、单元测试和构建。
- `task test:integration`：使用 Testcontainers 验证 PostgreSQL、Redis、迁移和认证 HTTP 流程。
- `task check:full`：`check` 加集成测试与漏洞扫描。
- `task test:race`：Go 竞态检测，由 Linux CI 强制执行。

Windows 本机没有 GCC 时不必为了 Race 安装额外工具链；在 Pull Request 中记录未执行即可，CI 会补验。Docker 不可用时同样跳过 `task check:full`，至少执行 `task check`。

Repository 不使用 SQLite 模拟 PostgreSQL。修复缺陷应先添加回归测试；涉及 goroutine、缓存、事件、迁移并发或共享状态的改动需要补充相应测试。`task check` 会重新生成 OpenAPI 与 sqlc 代码；CI 在干净工作区运行 `task generate:check`，确认生成物已提交且没有漂移。
