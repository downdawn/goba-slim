# SQL 管理

`db/migrations` 是数据库结构演进事实来源，`db/queries` 是查询事实来源。服务启动只检查版本，不执行迁移。

## 新增或修改表

不需要手工登录数据库执行零散 SQL。每次结构变化新增一个有序文件，例如：

```text
db/migrations/000002_create_orders.sql
```

写入向前 SQL 后，执行：

```bash
task generate
task db:migrate
```

`goba db migrate` 使用 `goba_schema_version` 记录已应用版本，并在 PostgreSQL 锁保护下按顺序执行未应用的文件。新建表、加列、索引或约束都是这条流程；查询变化再同步修改 `db/queries`，由 sqlc 生成代码。

已应用的迁移不可修改或重排。表结构变更一律新增迁移：例如加列、回填数据、再收紧 `NOT NULL` 应按实际发布需要拆分为多个可审计步骤。

## 新项目与旧开发库

当前基线只有 `000001_initial.sql`，其中包含当前 `users` 和 `system_configs` 结构。全新空数据库可直接迁移。旧开发数据库若来自本次基线之前，可删除并重新创建；项目处于开发期，不提供旧 Schema 或旧版本表的兼容转换逻辑。

非空且没有 `goba_schema_version` 的数据库会被拒绝，避免误接管现有数据。生产环境的特殊导入或数据转换必须由该项目的部署方案单独设计、备份和审计，不能依赖 API 启动时处理。

## 部署职责

本地 `task run` 与 GoLand `run.go` 会先调用迁移命令。Compose 和生产编排应运行一次性 `goba db migrate` Job，再启动或滚动常驻 API。`goba serve` 没有建表或改表权限。

修改迁移或查询后执行：

```bash
task generate
task generate:check
```

`api/openapi/generated` 与 `db/generated` 都由工具生成，不得手工修改。
