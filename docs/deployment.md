# 部署说明

## 部署模型

GoBA Slim 支持两种部署拓扑：完整 Compose 适合开发、验收和小规模自托管；API 镜像连接外部 PostgreSQL/Redis 适合托管数据库、集群和平台化部署。无论哪种拓扑，API、PostgreSQL 与 Redis 都保持独立容器或独立服务，应用镜像不内置数据库，也不会在启动时修改 Schema。

生产环境可由 Docker Compose、Kubernetes、Nomad、云容器平台或自有编排系统管理。仓库根目录的 `compose.yaml` 提供可运行的自托管基线，但上线前必须按风险补齐 TLS、备份、监控、资源限制和恢复策略。

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

旧验证公钥通过 `kid=文件路径` 映射提供，多个值使用英文逗号分隔：

```text
GOBA_AUTH_KEY_ID=2026-07
GOBA_AUTH_VERIFICATION_KEY_FILES=2026-06=/run/secrets/auth_public_2026_06.pem
```

推荐由部署平台把 Secret 挂载为只读文件。不要把实际 Secret 写入镜像、Compose 文件、仓库或日志。

启用文件模块时，还需要为 `file.storage_path` 提供独立可写持久卷。应用根文件系统仍可保持只读，例如：

```text
GOBA_MODULES_FILE=true
GOBA_FILE_STORAGE_PATH=/var/lib/goba/uploads
GOBA_FILE_IMAGE_MAX_BYTES=5242880
```

公开文件 URL 不提供访问控制，适合头像、封面和普通业务图片，不用于合同、证件或其他私密附件。需要私有文件时应接入私有对象存储和限时签名 URL，而不是复用公开路由。

仓库 Compose 在 Linux 和 macOS 上通过 `GOBA_CONTAINER_UID`、`GOBA_CONTAINER_GID` 让非 root API 进程与宿主私钥文件所有者对齐；`task compose:up` 会自动设置这两个值。其他部署平台应直接把 Secret 文件的读取权限授予容器运行用户，不应放宽宿主私钥权限。

## JWT 密钥轮换

Access Token 使用当前 Ed25519 私钥签发，并根据 JWT `kid` 从当前公钥和旧公钥集合中选择验证密钥。轮换不会延长 Token 生命周期，也不会绕过 Redis Session 校验。

1. 从当前私钥导出旧公钥：

   ```bash
   task auth:public-key -- --private /run/secrets/auth_private_key --output /secure/auth_public_2026_06.pem
   ```

2. 生成带配套公钥的新密钥：

   ```bash
   task auth:keygen -- --output /secure/auth_private_2026_07.pem --public-output /secure/auth_public_2026_07.pem
   ```

3. 将新私钥设为 `GOBA_AUTH_PRIVATE_KEY_FILE`，更新 `GOBA_AUTH_KEY_ID`，并把旧 `kid` 与旧公钥加入 `GOBA_AUTH_VERIFICATION_KEY_FILES` 后滚动部署。
4. 保留旧公钥至少一个 `GOBA_AUTH_ACCESS_TOKEN_TTL`，确认旧 Token 全部自然过期后再移除。旧私钥不再用于签发，应按密钥管理策略销毁或离线归档。

当前 `key_id` 不能同时出现在旧公钥映射中。重复 `kid`、未知 `kid`、无效 PEM 或非 Ed25519 PKIX 公钥都会导致配置或认证失败。
容器部署时，映射中的路径必须是容器内只读挂载路径；宿主机路径不会自动进入 API 容器。仓库默认 Compose 不挂载旧公钥，轮换部署应由实际编排平台增加对应 Secret/Volume。

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

数据库实例和目标空数据库由部署方或托管平台创建。首次部署前，由受控的一次性 Job 初始化 GoBA Schema：

```bash
goba db init --yes --config /etc/goba/config.yaml
```

该命令在当前 Schema 已就绪时直接成功，但会拒绝未知表、缺失表或版本不匹配的数据库。后续 Schema 变更由部署流程按 `db/schema` 中的显式 SQL 执行。常驻 API 容器只检查连接和 Schema 版本，不获得自动修改数据库的职责。

从 Schema 版本 1 升级到版本 2 时，在停止旧版本写入后由受控部署任务执行：

```bash
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f db/schema/000002_systemconfig.sql
```

执行前应完成备份，并在目标环境验证回滚方案。升级 SQL 创建 `system_configs` 表并记录版本 2；API 启动时只验证版本，不自动执行该文件。

仓库提供的 `task compose:up` 可一行启动 PostgreSQL、Redis、一次性初始化任务和 API，适合开发、验收与小规模自托管。它仍然使用职责独立的多个容器；正式部署应按风险补充持久化备份、TLS、监控和恢复方案。

Compose 中的常驻 API 容器以非 root 用户、`no-new-privileges` 和去除全部 Linux capabilities 运行，文件模块只使用独立数据卷。Docker Desktop 无法将 Compose 的环境变量型 Secret 挂载到只读根文件系统，因此默认 Compose 不设置 `read_only`，以保证 Windows 开箱可用。生产部署应使用镜像支持的 `--read-only --tmpfs /tmp` 策略，并将 Secret 以只读文件和为文件存储提供的独立可写卷挂载。`db-init` 是一次性、受控的 Schema 初始化任务，生产环境应使用部署平台提供的一次性 Job 运行相同命令，并按平台能力收紧其文件系统权限。

## 健康检查

- `/livez`：进程存活；
- `/readyz`：PostgreSQL、Redis 和模块就绪；
- `goba db status`：检查 PostgreSQL 版本与 Schema 版本。

平台应使用 `/readyz` 控制流量接入，并在更新期间保留足够的优雅退出时间。生产配置校验失败时应用会拒绝启动，包括文档未关闭、数据库或 Redis 未启用安全传输、Cookie 未启用 Secure 等情况。
