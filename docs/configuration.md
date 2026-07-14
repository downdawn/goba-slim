# 配置参考

配置优先级为内置安全默认值、YAML、显式加载的 `.env`、`GOBA_` 环境变量和 Secret 文件。生产禁止加载源码目录 `.env`。YAML 使用严格解码：未知键、错误类型和不合法时长都会拒绝启动，避免配置拼写错误被静默忽略。

核心分组：

- `app`：环境、调试和 API 文档。
- `server`：监听地址、超时和可信代理。
- `database`：PostgreSQL 连接、TLS 与连接池。
- `redis`：Redis 连接、TLS、超时与连接池。
- `auth`：JWT、Cookie、登录限流与密钥轮换。
- `cors`：允许的来源、方法、Header 和凭据。
- `file`：内置可选文件能力的目录、大小与视频开关。
- `systemconfig`：内置可选动态配置能力的公共缓存 TTL。
- `modules`：内置可选能力的启用状态。

Secret 同时支持明文环境变量和 `_FILE`，两者同时存在会拒绝启动。`goba config print --redact` 只输出脱敏值；`goba doctor` 可检查密钥、PostgreSQL、Redis 和已启用能力。

启用内置能力只需配置：

```yaml
modules:
  file: true
  systemconfig: true
```

文件模块还需要 `file.storage_path` 可写。动态配置不能修改 `database.*`、`redis.*`、`auth.*`、`cors.*`、Secret、Token、Cookie 或私钥相关启动配置。
