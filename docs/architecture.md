# 架构说明

GoBA Slim 是一个面向 Go HTTP 服务的模块化单体工程内核。当前架构优先保证边界清晰、显式装配和可验证性，不提前建设尚无实际职责的抽象。

## 核心原则

- Composition Root 位于 `internal/app`，集中构造配置、日志、模块和 HTTP 服务。
- Gin 和 OpenAPI 生成类型只存在于 HTTP 平台与传输边界。
- 业务模块使用 `context.Context` 和自身模型，不依赖 Gin、pgx、sqlc 或 go-redis 的具体类型。
- OpenAPI 是 HTTP 契约事实来源；未来引入数据库后，SQL 是查询事实来源。
- 模块通过 Manifest 声明名称和依赖，按拓扑顺序启动并按相反顺序停止。
- 公共能力按稳定职责组织，不建立无边界的 `utils` 或 `helpers` 包。
- 数据库变更只提供显式 SQL，应用启动时不执行自动迁移。

## 当前结构

```text
cmd/goba
  -> internal/cli
     -> internal/app
        -> internal/module
        -> internal/platform
        -> internal/transport/httpapi
           -> api/openapi/generated
```

`internal/platform` 承载配置、日志、HTTP Server 和健康检查等平台能力。`internal/transport/httpapi` 负责将 OpenAPI HTTP 请求映射到应用能力，并在边界统一处理错误响应。

## 配置与安全

配置在启动阶段加载为强类型结构，优先级为内置默认值、YAML、显式加载的 `.env`、`GOBA_` 环境变量。Secret 的明文值与 `_FILE` 来源互斥，日志、错误响应、CLI 输出、示例配置和镜像不得泄露 Secret。

生产环境必须关闭 debug 和 API 文档。TLS 由部署侧的 Nginx、Ingress 或 API Gateway 终止，应用镜像以非 root 用户运行。

## 演进约束

后续 PostgreSQL、Redis、用户和认证能力应作为有明确边界的模块加入，不改变 HTTP、数据和基础设施之间的依赖方向。只有出现跨模块协作、多实现或测试替换需求时才增加接口。

阶段规划见 [`roadmap.md`](roadmap.md)，开发规则见 [`development.md`](development.md)。
