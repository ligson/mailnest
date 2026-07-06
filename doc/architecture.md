# Mail Nest 架构设计

## 1. 总体架构

Mail Nest 采用前后端分离架构：

```text
浏览器
  │
  ▼
mailnest-fe（Vue 3 / Vite / TypeScript / Ant Design Vue）
  │ HTTP JSON
  ▼
mailnest-be（Go API 服务）
  │
  ├── SQLite：用户、邮箱账号、邮件元数据、任务状态
  └── 本地文件目录：邮件原文、正文缓存、附件文件
```

后端负责鉴权、邮箱配置加密、邮件收取、邮件解析、数据隔离和统一 API 响应。前端负责登录注册、邮箱配置管理、邮件列表和邮件详情展示。

## 2. 推荐目录结构

```text
mailnest/
├── mailnest-be/
│   ├── cmd/mailnest/
│   ├── internal/
│   │   ├── api/
│   │   ├── auth/
│   │   ├── config/
│   │   ├── mail/
│   │   ├── storage/
│   │   ├── worker/
│   │   └── response/
│   ├── migrations/
│   ├── go.mod
│   └── README.md
├── mailnest-fe/
│   ├── src/
│   │   ├── api/
│   │   ├── components/
│   │   ├── layouts/
│   │   ├── router/
│   │   ├── stores/
│   │   └── views/
│   ├── package.json
│   └── README.md
└── doc/
```

## 3. 后端模块

### 3.1 API 模块

负责 HTTP 路由、请求参数校验、调用业务服务和返回统一 envelope。

建议接口统一以 `/api/v1` 开头。

### 3.2 response 模块

集中封装 JSON 响应格式：

```json
{
  "success": true,
  "message": "",
  "httpCode": 200,
  "data": {}
}
```

所有 handler 应复用该模块，避免出现多个响应结构。

### 3.3 auth 模块

负责用户注册、登录、密码哈希、登录态校验和当前用户上下文。

登录态一期使用 JWT，便于前后端分离开发。后续如果需要更强的服务端会话控制，可以改为 session 或增加 token 黑名单。

邮箱认证支持两种模式：

- `password`：邮箱密码或应用专用密码，加密后存入 SQLite。
- `oauth2`：Microsoft OAuth2 授权，access token 和 refresh token 加密后存入 SQLite，IMAP 登录时使用 OAuth bearer 方式。

### 3.4 storage 模块

负责 SQLite 连接、数据库迁移、事务和基础查询。

数据库默认放在本地数据目录，例如：

```text
data/mailnest.db
```

### 3.5 mail 模块

负责邮箱账号配置、IMAP 连接测试、邮件拉取、MIME 解析、附件落盘和邮件去重。一期只支持 IMAP/IMAPS，只收取 `INBOX`。Outlook 二步验证场景优先使用 Microsoft OAuth2 授权。

### 3.6 worker 模块

负责后台收取任务：

- 按邮箱账号收取间隔调度。
- 支持手动触发收取。
- 记录任务执行结果。
- 避免同一个邮箱账号同时运行多个收取任务。

## 4. 前端模块

### 4.1 登录注册

页面：

- `/login`
- `/register`

功能：

- 表单校验。
- 登录成功后保存 token 或 session 状态。
- 未登录访问业务页面时跳转登录页。

### 4.2 主界面

建议主界面包含：

- 左侧导航：邮件、邮箱账号、设置。
- 顶部用户区：当前用户、退出登录。
- 内容区：邮件列表、邮件详情或配置表单。

### 4.3 邮箱账号管理

页面：

- 邮箱账号列表。
- 新增/编辑邮箱账号弹窗或页面。
- 连接测试按钮。
- 手动收取按钮。

### 4.4 邮件查看

页面：

- 邮件列表。
- 邮件详情。
- 附件下载入口。

前端需要集中封装 API 客户端：

- 自动附带登录凭据。
- 统一解析 envelope。
- 当 `success=false` 时给出统一错误提示。
- 当 HTTP 401 时跳转登录页。

## 5. 数据模型草案

### 5.1 users

- `id`
- `username`
- `email`
- `password_hash`
- `created_at`
- `updated_at`

### 5.2 mail_accounts

- `id`
- `user_id`
- `display_name`
- `email`
- `imap_host`
- `imap_port`
- `imap_tls`
- `imap_username`
- `imap_password_encrypted`
- `poll_interval_minutes`
- `enabled`
- `last_sync_at`
- `last_sync_status`
- `last_sync_error`
- `created_at`
- `updated_at`

### 5.3 mail_messages

- `id`
- `user_id`
- `account_id`
- `local_folder_id`
- `folder`
- `imap_uid`
- `message_id`
- `subject`
- `from_addr`
- `to_addrs`
- `cc_addrs`
- `sent_at`
- `received_at`
- `has_attachments`
- `text_body_path`
- `html_body_path`
- `raw_path`
- `search_text`
- `created_at`
- `updated_at`

建议为 `account_id + folder + imap_uid` 建唯一约束，同时尽量保存 `message_id` 作为辅助去重依据。

### 5.4 mail_attachments

- `id`
- `user_id`
- `message_id`
- `filename`
- `content_type`
- `size`
- `file_path`
- `created_at`

### 5.5 mail_folders

- `id`
- `user_id`
- `name`
- `color`
- `sort_order`
- `created_at`
- `updated_at`

本地文件夹只影响 Mail Nest 展示，不移动远端 IMAP 邮件。

### 5.6 mail_rules

- `id`
- `user_id`
- `name`
- `enabled`
- `match_mode`
- `target_folder_id`
- `sort_order`
- `created_at`
- `updated_at`

### 5.7 mail_rule_conditions

- `id`
- `rule_id`
- `field`
- `operator`
- `value`

### 5.8 mail_sync_jobs

- `id`
- `user_id`
- `account_id`
- `trigger_type`
- `status`
- `started_at`
- `finished_at`
- `new_message_count`
- `error_message`

## 6. 本地文件存储

建议结构：

```text
data/
├── mailnest.db
└── users/
    └── <user_id>/
        └── accounts/
            └── <account_id>/
                └── messages/
                    └── <message_id>/
                        ├── raw.eml
                        ├── body.txt
                        ├── body.html
                        └── attachments/
```

数据库保存相对路径，运行时根据数据目录拼接绝对路径。

## 7. 安全设计

- 用户密码必须哈希保存。
- 邮箱密码或授权码必须加密后存入 SQLite。
- 邮箱凭据加密密钥通过 `config.yaml` 或环境变量提供，不应提交真实密钥到仓库。
- 是否允许用户注册由 `config.yaml` 中的配置项控制。
- 所有用户数据接口必须校验登录态。
- 所有查询必须按当前用户过滤。
- 日志中禁止输出密码、授权码、token 等敏感内容。

## 8. 错误处理

后端错误响应仍然使用统一 envelope，例如：

```json
{
  "success": false,
  "message": "邮箱连接失败，请检查服务器地址、端口或授权码",
  "httpCode": 400,
  "data": {}
}
```

常见状态码：

- `200`：成功。
- `201`：创建成功。
- `400`：请求参数错误。
- `401`：未登录或登录态过期。
- `403`：无权访问。
- `404`：资源不存在。
- `409`：资源冲突，例如用户名已存在。
- `500`：服务器内部错误。

## 9. 测试重点

- 注册登录流程。
- 密码哈希和登录态校验。
- 用户数据隔离。
- 邮箱账号凭据加密。
- IMAP 连接测试。
- 邮件去重。
- 邮件 MIME 解析。
- 统一响应 envelope。
