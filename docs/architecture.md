# 架构说明

GoBA Slim 是面向新项目克隆使用的模块化单体基线。它优先保证代码位置明确、启动链路可预期和新业务能快速接入，不建设运行时插件市场、模块注册表或通用生命周期框架。

## 核心原则

- Composition Root 位于 `internal/app`，集中构造依赖、可选能力和 HTTP 服务。
- Gin 与 OpenAPI 生成类型只存在于 HTTP 平台和传输边界。
- 业务模块使用 `context.Context` 和自身模型，不依赖 Gin、pgx、sqlc 或 go-redis 的具体类型。
- OpenAPI 是 HTTP 契约事实来源；`db/migrations` 是结构演进事实来源；`db/queries` 是查询事实来源。
- 公共能力按稳定职责组织，不建立无边界的 `utils` 或 `helpers` 包。
- 常驻服务不执行数据库迁移；迁移是部署或本地启动包装器中的显式步骤。

## 当前结构

```text
cmd/goba
  -> internal/cli
     -> internal/app                 # 显式组合根
        -> platform/database
        -> platform/redisstore
        -> modules/user
        -> modules/auth
        -> modules/file              # 配置启用
        -> modules/systemconfig      # 配置启用
        -> transport/httpapi
           -> api/openapi/generated
```

`internal/platform` 承载配置、PostgreSQL、Redis、日志、HTTP Server 和健康检查等平台能力。用户与认证业务不依赖具体数据库或 Redis 类型；基础设施适配器在边界处完成转换。`internal/transport/httpapi` 负责 OpenAPI DTO、Bearer、Cookie、Origin 和统一错误响应。

应用的实际启动顺序是 PostgreSQL、Redis、可选文件存储，关闭时反序执行。用户、认证和动态配置是无独立连接生命周期的服务，直接使用已启动的平台依赖。此处使用少量显式代码，而不是所有模块都实现空的 `Start`、`Stop` 或 `Manifest` 方法。

## 内置能力和项目业务

核心能力是 `user` 与 `auth`。`file`、`systemconfig` 是随仓库源码提供的可选能力：配置开关决定是否构造服务和注册路由，不存在下载、发现或注册步骤。它们适合在每个新项目中按需打开。

新项目业务直接建立在 `internal/modules/<name>`，例如 `internal/modules/order`。模块有真实的外部依赖、跨模块协作或测试替换需求时，才定义用途级接口；在 `internal/app` 中明确构造并传入 HTTP Handler。详见 [模块开发](modules.md)。

## 数据库与启动

`goba db migrate` 使用嵌入二进制的顺序迁移文件，并通过 PostgreSQL 锁串行化执行。它在空数据库中创建迁移版本表并应用 `000001_initial.sql`；以后每个结构变化增加一个新的迁移文件。`goba serve` 只校验 PostgreSQL 版本和已应用版本，绝不改表。

本地 `task run` 与 GoLand 的 `run.go` 都是“先迁移、再启动”的便利入口；Compose 使用独立的一次性 `db-migrate` 容器。它们没有把迁移职责放进常驻 API 服务。

## 配置与安全

配置在启动阶段加载为强类型结构，优先级为内置默认值、YAML、显式加载的 `.env`、`GOBA_` 环境变量。未知 YAML 键、错误类型和不合法配置会拒绝启动。Secret 的明文值与 `_FILE` 来源互斥，日志、错误响应、CLI 输出、示例配置和镜像不得泄露 Secret。

生产环境必须关闭 debug 和 API 文档。TLS 由部署侧的 Nginx、Ingress 或 API Gateway 终止，应用镜像以非 root 用户运行。

JWT 使用当前 Ed25519 私钥签发，并按 `kid` 从当前公钥和旧公钥集合中选择验证密钥。每次受保护请求仍校验 Redis Session、用户状态和会话版本，因此改密、停用、会话撤销与 Refresh Token 重用都能立即失效。

文件模块通过 `ObjectStore` 窄接口访问存储，当前 LocalStore 使用受根目录约束的文件系统操作。有效用户可上传，公开路由可读取，只有所有者或超级管理员可删除。动态配置模块只管理非秘密业务参数，管理接口要求超级管理员，公开列表使用 Redis Cache-Aside。
