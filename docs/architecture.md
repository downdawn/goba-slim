# 架构说明

GoBA Slim 是一个面向 Go HTTP 服务的模块化单体工程内核。当前架构优先保证边界清晰、显式装配和可验证性，不提前建设尚无实际职责的抽象。

## 核心原则

- Composition Root 位于 `internal/app`，集中构造配置、日志、模块和 HTTP 服务。
- Gin 和 OpenAPI 生成类型只存在于 HTTP 平台与传输边界。
- 业务模块使用 `context.Context` 和自身模型，不依赖 Gin、pgx、sqlc 或 go-redis 的具体类型。
- OpenAPI 是 HTTP 契约事实来源，`db/schema` 与 `db/queries` 中的 SQL 是数据库事实来源。
- 模块通过 Manifest 声明名称和依赖，按拓扑顺序启动并按相反顺序停止。
- 公共能力按稳定职责组织，不建立无边界的 `utils` 或 `helpers` 包。
- 数据库变更只提供显式 SQL，应用启动时不执行自动迁移。

## 当前结构

```text
cmd/goba
  -> internal/cli
     -> internal/app
        -> internal/module
        -> internal/platform/database
        -> internal/platform/redisstore
        -> internal/modules/auth
        -> internal/modules/user
           -> internal/modules/user/postgres
              -> db/generated
        -> internal/modules/file (可选)
        -> internal/modules/systemconfig (可选)
           -> internal/modules/systemconfig/postgres
           -> internal/modules/systemconfig/redis
        -> internal/transport/httpapi
           -> api/openapi/generated
```

`internal/platform` 承载配置、PostgreSQL、Redis、日志、HTTP Server 和健康检查等平台能力。用户业务模型和服务不依赖 pgx/sqlc；认证服务不依赖 go-redis 或 Gin。基础设施适配器负责把具体类型转换到用途级接口。`internal/transport/httpapi` 负责 OpenAPI 映射、Bearer、Cookie、Origin 和统一错误响应。

数据库在核心模块之前启动并检查 PostgreSQL 16+ 与 Schema 版本，关闭时按相反顺序释放。`serve` 不修改 Schema；只有显式 `goba db init` 可在空数据库中执行 `db/schema` 初始化 SQL，当前 Schema 已就绪时该命令幂等成功。用户写入通过模块内 Unit of Work 控制事务，事务对象不进入 Context。

## 配置与安全

配置在启动阶段加载为强类型结构，优先级为内置默认值、YAML、显式加载的 `.env`、`GOBA_` 环境变量。Secret 的明文值与 `_FILE` 来源互斥，日志、错误响应、CLI 输出、示例配置和镜像不得泄露 Secret。

生产环境必须关闭 debug 和 API 文档。TLS 由部署侧的 Nginx、Ingress 或 API Gateway 终止，应用镜像以非 root 用户运行。

JWT 使用当前 Ed25519 私钥签发，并按 `kid` 从当前公钥和旧公钥集合中选择验证密钥。轮换期间只保留旧公钥，不继续加载旧私钥；每次受保护请求仍校验 Redis Session，因此改密、停用和主动撤销可以立即失效，不依赖 Access Token 自然过期。

可选文件模块通过 `ObjectStore` 窄接口访问存储，当前 LocalStore 使用受根目录约束的文件系统操作。对象 Key 只由服务端生成并携带所有者 UUID；有效用户可上传，公开路由可读取，只有所有者或超级管理员可删除。模块关闭时相关路由不会注册。

可选动态配置模块只管理非秘密业务参数，支持 string、integer、boolean、duration 与 string list。管理接口当前复用超级管理员语义，公开读取接口不需要认证。公开列表使用 Redis Cache-Aside，Redis 读取故障时回源 PostgreSQL；写入事务提交后才删除缓存并发布进程内 `ConfigChanged` 事件。启动配置、Secret 和信任边界键会被拒绝。

## 演进约束

Auth 只能通过用户模块提供的窄接口读取身份和安全状态，不得直接访问用户表。每次受保护请求同时验证 EdDSA JWT、Redis Session、用户状态和会话版本；Redis 故障时认证 fail closed。只有出现跨模块协作、多实现或测试替换需求时才增加接口。

阶段规划见 [`roadmap.md`](roadmap.md)，开发规则见 [`development.md`](development.md)。
