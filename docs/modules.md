# 模块开发

GoBA Slim 不提供运行时插件机制。它用于克隆成一个新项目，因此模块就是仓库中的普通业务代码，避免为了“可插拔”增加注册表、反射、动态加载和额外配置层。

## 已有内置能力

`file` 与 `systemconfig` 的源码、配置结构、迁移和 OpenAPI 契约已经随仓库提供。使用时只需打开开关：

```yaml
modules:
  file: true
  systemconfig: true
```

文件模块另外要求 `file.storage_path` 指向可写目录；动态配置没有额外安装步骤。关闭开关会取消相关路由注册，但不会删除数据表或要求删除配置。它们是可复用的内置能力，不是跨项目自动发现的“插件包”。

## 新增业务模块

按实际业务建立目录，例如 `internal/modules/order`。通常的接入顺序是：

1. 定义业务模型、Service 和必要的用途级接口。
2. 需要持久化时新增 `db/migrations/000002_create_orders.sql`，查询写入 `db/queries`，然后运行 `task generate`。
3. 先修改 `api/openapi/openapi.yaml`，生成 HTTP 契约代码。
4. 在 `internal/transport/httpapi` 完成 DTO、业务模型和错误映射；业务层只接收 `context.Context`。
5. 在 `internal/app` 显式构造依赖并传给 Handler。
6. 添加单元测试、必要的集成测试、配置示例和公开文档。

如果新能力也需要开关，在 `modules` 增加一个明确布尔配置，并在 `internal/app` 与路由注册处做同一处判断。只有模块确实拥有连接、后台任务或可检查的外部资源时，才加入应用生命周期；纯业务 Service 不需要空生命周期方法。

跨模块协作使用构造期传入的窄接口。禁止字符串 Service Locator、包级可变单例和直接读写其他模块数据表。
