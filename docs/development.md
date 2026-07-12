# 开发说明

## 前置条件

- Go 1.26
- Docker（可选）
- Task（可选）

## 常用命令

```powershell
E:\Program Files\Go\bin\go.exe test ./...
E:\Program Files\Go\bin\go.exe vet ./...
E:\Program Files\Go\bin\go.exe run ./cmd/goba config check --config configs/config.example.yaml
```

修改 `api/openapi/openapi.yaml` 后，执行 `task generate` 重新生成 HTTP 契约代码。
