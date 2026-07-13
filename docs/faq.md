# 常见问题

## 为什么服务不自动迁移数据库

生产数据库变更需要受控执行、备份和回滚。GoBA Slim 只校验 Schema 版本，避免服务启动时隐式修改数据。

## 为什么 Redis 故障后不能认证

Redis Session 是撤销和安全状态事实来源，认证故障默认 fail closed。普通动态配置缓存故障则回源 PostgreSQL。

## 为什么文件 URL 是公开的

首版文件模块面向头像、封面等公开素材。私密附件应使用私有对象存储和限时签名 URL。

## 为什么没有 RBAC

Slim 使用超级管理员和资源所有者满足首版闭环。RBAC、组织和数据权限属于 GoBA Full 范围。

## 如何定位启动失败

先运行 `goba config check`，再运行 `goba doctor`。输出只包含安全诊断结论，不会打印 Secret。
