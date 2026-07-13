# SQL 管理

`db/schema` 是 Schema 事实来源，`db/queries` 是查询事实来源。应用启动只检查版本，不执行迁移。

- 空数据库使用 `goba db init` 顺序执行初始化 SQL。
- 已有数据库按版本显式执行后续 SQL，例如 `000002_systemconfig.sql`。
- SQL 使用显式列名、稳定排序、参数绑定和数据库约束。
- 修改 SQL 后运行 `task generate:sqlc` 和 `task generate:check`。

部署升级前必须备份、验证目标版本和回滚方式。`schema_migrations` 记录当前已应用版本，但不是运行时自动迁移框架。
