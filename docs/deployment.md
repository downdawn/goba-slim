# 部署说明

## 部署模型

GoBA Slim 生产环境只部署 API 镜像。PostgreSQL 与 Redis 是外部依赖，可以是独立主机、集群或云托管服务。应用镜像不内置数据库，不负责创建实例，也不会在启动时修改 Schema。

推荐由 Kubernetes、Nomad、云容器平台或自有编排系统管理容器。仓库根目录的 `compose.yaml` 面向本地开发和验收，不作为生产模板。

## 构建镜像

```bash
docker build -f deployments/Dockerfile -t goba-slim:latest .
```

镜像默认执行 `goba serve`，以非 root 用户运行，监听容器内 `8000` 端口。TLS 应由 Ingress、负载均衡器或 API Gateway 终止。

## 配置外部服务

普通配置可以通过 `GOBA_` 环境变量注入。生产环境至少需要根据实际服务设置：

```text
GOBA_APP_ENVIRONMENT=production
GOBA_APP_DOCS_ENABLED=false
GOBA_DATABASE_HOST=<postgres-host>
GOBA_DATABASE_PORT=5432
GOBA_DATABASE_NAME=goba
GOBA_DATABASE_USER=<postgres-user>
GOBA_DATABASE_SSL_MODE=verify-full
GOBA_REDIS_HOST=<redis-host>
GOBA_REDIS_PORT=6379
GOBA_REDIS_TLS=true
GOBA_AUTH_COOKIE_SECURE=true
```

数据库密码、Redis 密码和 Ed25519 PKCS#8 私钥是 Secret。每项可以使用明文环境变量或 `_FILE` 文件路径，不能同时配置两种来源：

```text
GOBA_DATABASE_PASSWORD_FILE=/run/secrets/database_password
GOBA_REDIS_PASSWORD_FILE=/run/secrets/redis_password
GOBA_AUTH_PRIVATE_KEY_FILE=/run/secrets/auth_private_key
```

推荐由部署平台把 Secret 挂载为只读文件。不要把实际 Secret 写入镜像、Compose 文件、仓库或日志。

## 启动容器

以下命令只展示镜像契约；实际 Secret 文件应由部署平台提供：

```bash
docker run --rm -p 8000:8000 \
  --read-only \
  --tmpfs /tmp \
  --env-file /path/to/non-secret.env \
  -v /path/to/secrets:/run/secrets:ro \
  goba-slim:latest
```

也可以挂载 YAML 并覆盖默认命令：

```bash
docker run --rm -p 8000:8000 \
  -v /path/to/config.yaml:/etc/goba/config.yaml:ro \
  goba-slim:latest serve --config /etc/goba/config.yaml
```

## 数据库初始化

首次部署前，由受控的一次性 Job 对目标空数据库执行：

```bash
goba db init --yes --config /etc/goba/config.yaml
```

后续 Schema 变更由部署流程按 `db/schema` 中的显式 SQL 执行。常驻 API 容器只检查连接和 Schema 版本，不获得自动修改数据库的职责。

## 健康检查

- `/livez`：进程存活；
- `/readyz`：PostgreSQL、Redis 和模块就绪；
- `goba db status`：检查 PostgreSQL 版本与 Schema 版本。

平台应使用 `/readyz` 控制流量接入，并在更新期间保留足够的优雅退出时间。生产配置校验失败时应用会拒绝启动，包括文档未关闭、数据库或 Redis 未启用安全传输、Cookie 未启用 Secure 等情况。
