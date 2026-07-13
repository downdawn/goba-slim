# 配置参考

配置优先级为内置安全默认值、YAML、显式加载的 `.env`、`GOBA_` 环境变量和 Secret 文件。生产禁止加载源码目录 `.env`。

核心分组：

- `app`：环境、调试和 API 文档。
- `server`：监听地址、超时和可信代理。
- `database`：PostgreSQL 连接、TLS 与连接池。
- `redis`：Redis 连接、TLS、超时与连接池。
- `auth`：JWT、Cookie、登录限流与密钥轮换。
- `cors`：允许的来源、方法、Header 和凭据。
- `file`：可选文件模块的目录、大小与视频开关。
- `systemconfig`：可选动态配置的公共缓存 TTL。
- `modules`：编译内置可选模块的启用状态。

Secret 同时支持明文环境变量和 `_FILE`，两者同时存在会拒绝启动。`goba config print --redact` 只输出脱敏值；`goba doctor` 可在线检查密钥、PostgreSQL、Redis 和启用模块。
