# GoBA Slim

GoBA Slim 是一个以模块化组合根、强类型配置和安全 HTTP 边界为基础的 Go 服务骨架。

## 快速开始

```powershell
Copy-Item .env.example .env
E:\Program Files\Go\bin\go.exe run ./cmd/goba config check --config configs/config.example.yaml
E:\Program Files\Go\bin\go.exe run ./cmd/goba serve --config configs/config.example.yaml
```

直接执行 `goba` 而不带参数会显示帮助。这是多命令 CLI 的标准行为；使用 `serve` 子命令启动服务。

如使用 GoLand，打开共享的 `.run/Goba Serve` 配置即可一键运行，配置与上述命令一致。

开发服务默认监听 `127.0.0.1:8000`，启动后可访问：

- `GET /docs`：交互式 Swagger UI 文档（仅 development 且 `docs_enabled: true` 时提供）
- `GET /openapi.json`：OpenAPI JSON 契约（仅 development 且 `docs_enabled: true` 时提供）
- `GET /livez`：进程存活检查
- `GET /readyz`：依赖就绪检查

## 配置

配置按默认值、YAML 文件、`GOBA_` 环境变量的顺序覆盖。敏感值不应提交到仓库，可参考 `.env.example` 和 `configs/config.example.yaml`。

## 验证

```powershell
E:\Program Files\Go\bin\go.exe test ./...
E:\Program Files\Go\bin\go.exe vet ./...
E:\Program Files\Go\bin\go.exe build ./cmd/goba
```
