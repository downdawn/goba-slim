# Todo 示例模块

该目录演示 GoBA Slim 的最小模块边界，不默认装配进生产二进制：

- `Todo` 与 `Service` 不依赖 Gin、OpenAPI、pgx 或 sqlc 类型；
- Repository 是模块自身的窄接口；
- `Module.Manifest` 声明稳定名称和直接依赖；
- 测试使用内存适配器和固定 ID，保持确定性。

真实业务模块应继续完成以下接入：

1. 在 `api/openapi/openapi.yaml` 增加带明确 tag 的契约并重新生成；
2. 在 `db/schema` 提供显式增量 SQL，在 `db/queries` 提供查询并运行 sqlc；
3. 在模块内实现 PostgreSQL Repository 映射，生成类型不离开数据层；
4. 在 `internal/app` 显式构造并注册模块；
5. 同步单元、HTTP 契约、PostgreSQL 集成测试和中文文档。
