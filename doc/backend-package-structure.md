# 后端包结构说明

本文档记录 `mailnest-be/internal` 目前的职责拆分。后续新增后端功能时，优先放入对应文件或新增同职责文件，避免继续把大量 handler、SQL 或业务流程堆到单个大文件中。

## 总体原则

- 保持包边界稳定：API、业务服务、存储层、认证、配置和响应封装各自负责自己的职责。
- 同一个包内可以继续按功能拆文件；只有当职责边界清晰、依赖方向稳定时，再考虑新建 package。
- 新增接口必须继续复用统一 JSON envelope 和鉴权中间件。
- 新增数据库表、字段或索引优先维护 GORM model 和集中迁移逻辑，生产升级必须先备份并确保不丢用户数据。
- 涉及邮件解析、同步、附件、联系人、规则等逻辑时，应优先补充对应测试。

## `internal/api`

API 包负责 HTTP 路由、参数校验、权限上下文、调用业务服务和响应 DTO 组装。当前按功能拆分如下：

- `app.go`：`App` 装配、路由注册、后台任务启动和健康检查。
- `requests.go`：所有 HTTP 请求体结构。
- `auth_handlers.go`：注册、登录、当前用户、退出和修改密码。
- `profile_handlers.go`：个人资料、头像上传和头像读取。
- `account_handlers.go`：邮箱账号增删改查、连接测试、目录读取、手动同步和全量同步状态。
- `message_handlers.go`：邮件列表、邮件详情、批量操作、移动文件夹、写信上下文和发送入口。
- `attachment_handlers.go`：附件中心列表、附件下载和内嵌附件读取。
- `contact_handlers.go`：联系人通讯录维护。
- `folder_rule_handlers.go`：本地文件夹和邮件规则维护、预览、应用。
- `sync_handlers.go`：同步任务和同步事件日志查询。
- `oauth_handlers.go`：Microsoft OAuth 授权入口和回调完成。
- `middleware.go`：鉴权中间件、JWT 签发和当前用户读取。
- `request_decode.go`：JSON、multipart 写信请求和附件读取。
- `presenters.go`：面向前端的响应 DTO 组装。
- `inline_images.go`：邮件详情中 CID 内嵌图片重写和图片兼容处理。
- `contacts_presenter_helpers.go`：联系人请求参数归一化。
- `helpers.go`：路由 ID、查询参数、日期、布尔值、空值等通用辅助函数。

## `internal/storage`

Storage 包负责数据库连接之上的数据读写方法，当前仍保留 `database.go` 的数据库方言封装和 `schema.go` 的 GORM 迁移模型，业务查询按数据域拆分：

- `storage.go`：`Store` 打开、关闭和迁移入口。
- `models.go`：业务实体、查询参数和返回结果结构。
- `errors.go`：存储层公共错误。
- `users.go`：用户账号、个人资料和密码哈希更新。
- `accounts.go`：邮箱账号配置、同步状态、全量同步状态和到期账号查询。
- `sync_jobs.go`：同步任务和同步事件日志。
- `messages.go`：邮件入库、列表查询、详情查询、状态更新、批量操作和文件夹归属。
- `attachments.go`：附件元数据、附件中心查询和邮件附件查询。
- `folders.go`：本地邮件文件夹维护。
- `contacts.go`：联系人维护和自动沉淀联系人。
- `rules.go`：邮件规则及规则条件维护。
- `helpers.go`：跨查询复用的归一化、空值和布尔转换辅助函数。

## `internal/mail`

Mail 包负责邮件相关业务流程，包括 SMTP 发信、IMAP 收取、全量同步、自动同步、规则应用、联系人沉淀和邮件内容落盘：

- `service.go`：`Service` 装配和依赖注入。
- `compose.go`：发送邮件、回复/回复全部/转发上下文、联系人展示名和引用正文。
- `account.go`：邮箱连接测试、目录列表、IMAP/SMTP 配置和 OAuth token 解密刷新。
- `autosync.go`：定时自动收取调度。
- `sync.go`：手动收取、全量同步、停止同步、同步状态和服务器旧邮件清理。
- `persist.go`：邮件正文、原文、附件落盘，邮件元数据入库和联系人沉淀。
- `repair.go`：历史邮件解析内容修复。
- `content_helpers.go`：搜索文本生成、HTML 清理、路径安全和时间解析。
- `rules.go`：邮件规则匹配和应用。
- `fetcher.go`、`imap_fetcher.go`：IMAP 拉取接口与实现。
- `sender.go`：SMTP 发信接口与实现。
