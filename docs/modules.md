# 模块开发

模块按业务能力组织，最小接口只包含 `Manifest` 与 `Register`。有真实生命周期、健康检查或外部资源时再实现对应可选接口，不创建空方法。

## 接入顺序

1. 定义模块业务模型、Service 和必要的窄接口。
2. 数据库变更写入新的显式 SQL；查询写入 `db/queries` 并运行 sqlc。
3. OpenAPI 先定义 HTTP 契约，再生成 Gin Server 和 DTO。
4. Transport 负责 DTO、业务模型和错误码映射，业务层只接收 `context.Context`。
5. 在 `internal/app` 显式构造依赖并注册模块。
6. 更新配置、示例、测试和公开文档。

跨模块协作使用构造期解析的强类型窄接口。禁止字符串 Service Locator、包级可变单例和跨模块直接访问数据表。完整示例见 [`examples/todo`](../examples/todo)。
