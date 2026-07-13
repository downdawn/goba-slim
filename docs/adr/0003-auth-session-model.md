# ADR 0003：JWT Access Token 与 Redis Session

- 状态：已接受
- 日期：2026-07-13

## 决策

Access Token 使用短期 EdDSA JWT，Refresh Token 使用不透明随机值并在 Redis 原子轮换。每次受保护请求验证 JWT、Redis Session、用户状态和会话版本。

## 原因

该模型兼顾无状态签名验证与即时撤销、改密失效、停用失效和 Token 重用检测。只验证 JWT 无法满足即时安全状态。

## 后果

Redis 是认证必要依赖，故障时 fail closed。密钥通过 `kid` 轮换，旧密钥只保留公钥至少一个 Access Token TTL。
