# 部署说明

## 部署模型

GoBA Slim 的常驻 API 连接外部 PostgreSQL 与 Redis，不内置数据库，也不在启动时修改表结构。完整 Compose 适合开发、验收和小规模自托管；Kubernetes、Nomad 或云平台应把迁移作为一次性 Job，把 API 作为常驻工作负载。

## 配置与 Secret

生产至少需要设置：

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

数据库密码、Redis 密码和 Ed25519 PKCS#8 私钥使用环境变量或 `_FILE` 文件路径，不能同时配置两种来源：

```text
GOBA_DATABASE_PASSWORD_FILE=/run/secrets/database_password
GOBA_REDIS_PASSWORD_FILE=/run/secrets/redis_password
GOBA_AUTH_PRIVATE_KEY_FILE=/run/secrets/auth_private_key
```

推荐由部署平台只读挂载 Secret。不要把实际 Secret 写入镜像、Compose 文件、仓库或日志。启用文件模块时，为 `GOBA_FILE_STORAGE_PATH` 提供独立可写持久卷；公开文件路由不适合私密附件。

## 数据库迁移

在部署 API 前，使用受控的一次性任务执行：

```bash
goba db migrate --config /etc/goba/config.yaml
```

该命令在空数据库中应用全部嵌入迁移，在已迁移数据库中只执行新增版本。它通过 PostgreSQL 锁避免多副本并发重复执行。API 使用：

```bash
goba serve --config /etc/goba/config.yaml
```

后者只校验数据库版本。非空但未受 GoBA 管理的数据库会被迁移命令拒绝；开发期的旧库可以删除重建，生产数据转换必须有单独的备份、审计和回滚计划。

仓库的 `task compose:up` 使用同一逻辑：一次性 `db-migrate` 容器成功后，`goba` API 容器才启动。

## JWT 密钥轮换

Access Token 使用当前 Ed25519 私钥签发，并根据 JWT `kid` 从当前公钥和旧公钥集合中选择验证密钥。轮换期间只保留旧公钥至少一个 `GOBA_AUTH_ACCESS_TOKEN_TTL`：

1. 从当前私钥导出旧公钥：

   ```bash
   task auth:public-key -- --private /run/secrets/auth_private_key --output /secure/auth_public_2026_06.pem
   ```

2. 生成新密钥：

   ```bash
   task auth:keygen -- --output /secure/auth_private_2026_07.pem --public-output /secure/auth_public_2026_07.pem
   ```

3. 将新私钥设为 `GOBA_AUTH_PRIVATE_KEY_FILE`，更新 `GOBA_AUTH_KEY_ID`，并在 `GOBA_AUTH_VERIFICATION_KEY_FILES` 中保留旧 `kid` 的公钥。

## 容器和健康检查

镜像以非 root 用户运行，TLS 应由 Ingress、负载均衡器或 API Gateway 终止。生产可使用只读根文件系统与 `/tmp` tmpfs，并给文件存储提供单独可写卷。

- `/livez`：进程存活。
- `/readyz`：PostgreSQL、Redis 和已启用存储就绪。
- `goba db status`：检查 PostgreSQL 与迁移版本。

平台应使用 `/readyz` 控制流量接入，并在更新期间保留足够的优雅退出时间。
