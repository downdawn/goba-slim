# 常见问题

## 新增表还要手动执行 SQL 吗

需要编写 SQL，但不需要手工登录数据库逐条执行。新增 `db/migrations/NNNNNN_name.sql` 后运行 `task db:migrate`；它按版本记录与 PostgreSQL 锁执行未应用迁移。`task run` 和 GoLand `run.go` 会先调用同一命令再启动服务。

## 表结构改了怎么办

不要修改已应用迁移。新增一个迁移文件，例如加列、建索引或数据回填；在本地执行 `task db:migrate`，部署时运行一次性 `goba db migrate` Job。常驻 `goba serve` 永远不会改表。

## 为什么旧数据库报“不受 GoBA 管理”

迁移只接管空库或已有 `goba_schema_version` 的 GoBA 数据库，防止误操作其他项目。当前仍在开发期，旧开发库可以删除后重新创建；不会提供旧 Schema 的兼容、猜测或自动转换。

## file、systemconfig 要做很多配置吗

不需要。源码和数据库结构已随仓库提供，在 `modules.file` 或 `modules.systemconfig` 设为 `true` 即可。文件模块额外需要一个可写 `file.storage_path`；动态配置使用已有 PostgreSQL 与 Redis。关闭开关仅取消路由，不要求删除表或源码。

## 后续模块能不能拿来就用

同一基线中的内置能力可以通过开关使用。新业务模块是直接复制项目后在 `internal/modules/<name>` 开发的普通代码，由 `internal/app` 显式装配；它不是跨任意项目自动安装的二进制插件。这样更少隐藏依赖，也更适合快速迭代。

## 为什么 Redis 故障后不能认证

Redis Session 是撤销和安全状态事实来源，认证故障默认 fail closed。普通动态配置缓存故障则回源 PostgreSQL。

## 为什么本机不跑 Race

`go test -race` 在 Windows 通常需要 GCC 工具链。日常本机使用 `task test` 或 `task check`，Linux CI 强制运行 Race。涉及并发改动时应在 Pull Request 中注明本机未跑 Race。

## 为什么文件 URL 是公开的

首版文件模块面向头像、封面等公开素材。私密附件应使用私有对象存储和限时签名 URL。

## 如何定位启动失败

先运行 `goba config check`，再运行 `goba db status` 或 `goba doctor`。输出只包含安全诊断结论，不会打印 Secret。
