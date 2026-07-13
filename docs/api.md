# API 流程

`api/openapi/openapi.yaml` 是 HTTP 契约事实来源。成功响应使用资源原生结构，错误统一包含稳定 `code`、中文 `message` 和可选 `request_id`。

主要流程：

- 登录：`POST /api/v1/auth/login` 返回 Access Token 并设置 HttpOnly Refresh Cookie。
- 刷新：`POST /api/v1/auth/refresh` 原子轮换 Refresh Token。
- 当前用户：`GET /api/v1/me` 同时验证 JWT、Redis Session 和用户状态。
- 文件：`POST /api/v1/files` 上传，`GET /files/{ownerId}/{fileName}` 公开读取。
- 动态配置：超级管理员管理 `/api/v1/system-configs`，公开端读取 `/api/v1/system-configs/public`。

浏览器 Cookie 接口校验 Origin，生产启用 Secure。Token、Cookie、密码和 Authorization 不写日志。
