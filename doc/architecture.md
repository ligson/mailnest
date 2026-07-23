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
  ├── 数据库：用户、邮箱账号、邮件元数据、任务状态
  └── 本地文件目录：邮件原文、正文缓存、附件文件
```

后端负责鉴权、邮箱配置加密、邮件收取、邮件解析、联系人沉淀、数据隔离和统一 API 响应。前端负责登录注册、邮箱配置管理、通讯录维护、邮件列表和邮件详情展示。

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

API 包内按功能拆分 handler、请求结构、响应组装、鉴权中间件和通用辅助函数，避免所有 HTTP 逻辑集中到单个大文件。详细文件职责见 [后端包结构说明](backend-package-structure.md)。

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

负责用户注册、登录、修改密码、个人资料、图形验证码、密码哈希、登录态校验和当前用户上下文。

登录态一期使用 JWT，便于前后端分离开发。后续如果需要更强的服务端会话控制，可以改为 session 或增加 token 黑名单。

用户登录密码只保存 `bcrypt` 哈希，不允许明文落库。登录后修改密码需要校验当前密码，修改成功后当前 JWT 暂时保持有效；如果后续需要“修改密码后踢掉所有设备”，再引入 token 版本号或黑名单机制。

登录和注册需要先获取图形验证码，提交时携带验证码 ID 和答案。验证码只保存在服务端内存中，短时有效且校验后立即失效，用于降低自动化撞库和批量注册风险。注册开关关闭时仍优先返回注册禁用错误，避免无意义消耗验证码。

用户表包含 `is_admin` 和 `enabled`。首个注册用户自动成为管理员；已有数据库升级时，如果不存在管理员，则把最早创建的用户设为管理员并保持启用。所有受保护接口在解析 JWT 后会再次读取用户状态，停用用户即使持有旧 token 也不能继续访问业务数据。

个人资料保存在 `users` 表中，包括昵称、头像路径、个人描述和界面主题偏好。头像文件保存到本地数据目录的用户资料目录，并通过受鉴权保护的接口读取；前端只拿到头像 URL，不直接接触本地文件路径。界面主题偏好使用枚举值保存，登录、当前用户和个人资料接口都会返回，前端据此设置全局主题变量和 Ant Design 主题 token。主题枚举包括常规办公配色和青花、朱砂、水墨、黛山等中国风配色；顶部栏和左侧导航统一使用框架主题色，避免主题切换后出现割裂。

邮箱认证支持两种模式：

- `password`：邮箱密码或应用专用密码，加密后存入当前配置的数据库。
- `oauth2`：Microsoft OAuth2 授权，access token 和 refresh token 加密后存入当前配置的数据库，IMAP 登录时使用 OAuth bearer 方式。

### 3.4 storage 模块

负责数据库连接、数据库迁移、事务和基础查询。后端通过 `config.yaml` 的 `database.driver` 选择数据库类型：默认 `sqlite`，也支持 `mysql` 和 `postgres`。连接驱动选择、建表、补列和普通索引迁移由 GORM 统一处理。

存储层查询按用户、邮箱账号、邮件、附件、文件夹、联系人、规则和同步任务拆分文件；`database.go` 保留方言连接封装，`schema.go` 保留 GORM 迁移模型。

SQLite 连接使用 WAL、busy timeout 和受控连接池，降低后台同步写入与 Web 读取并发时的锁等待。MySQL/PostgreSQL 通过 DSN 连接。新增表和字段应优先补充 GORM model 标签并让 `AutoMigrate` 迁移；只有邮件列表排序这类表达式索引、插入忽略、联系人 upsert、定时同步到期判断和自增主键返回等数据库差异，才保留在 storage 层集中封装，避免业务查询写死某一种数据库。

邮件列表查询应只读取摘要字段；需要正文、附件、搜索索引等完整数据时由详情、规则或后台任务单独查询，避免列表页为每封邮件加载大字段。

SQLite 数据库默认放在本地数据目录，例如：

```text
data/mailnest.db
```

配置示例：

```yaml
database:
  driver: "sqlite" # sqlite/mysql/postgres
  path: "/data/mailnest.db" # sqlite 使用
  dsn: "" # mysql/postgres 使用
  maxOpenConns: 4
  maxIdleConns: 4
```

### 3.5 mail 模块

负责邮箱账号配置、IMAP 连接测试、IMAP 文件夹列表读取、邮件拉取、SMTP 发信、MIME 解析、附件落盘和邮件去重。一期收信支持 IMAP/IMAPS，默认同步 `INBOX` 和账号配置的发件箱文件夹，必须覆盖邮箱服务器上的收件邮件和已发送邮件；发信支持 SMTP/SMTPS/STARTTLS。Outlook 二步验证场景优先使用 Microsoft OAuth2 授权。

邮件同步分为两类：

- 发件箱目录：不同邮箱服务商的已发送目录可能不是 `Sent`，邮箱账号页支持读取 IMAP 文件夹列表并选择真实发件箱目录。
- 日常收取：手动或定时触发，拉取 `INBOX` 和发件箱最近一批邮件，用于保持收件箱和发件箱更新。
- 全量历史同步：用户主动触发，先列出 `INBOX` 和发件箱全部 UID，再按批次从新到旧拉取并保存，用于首次导入或补齐历史邮件。
- 容错策略：`INBOX` 不存在或不可访问时同步失败；配置的发件箱目录不存在时跳过该目录并返回/记录提示，避免影响收件箱同步。
- 同步边界：Mail Nest 通过 IMAP 读取服务器数据。第三方客户端发送但仅保存在本机、没有上传到服务器发件箱目录的邮件，后端无法拉取；用户需要在客户端开启保存已发送邮件到服务器。

邮件入库仍以 `account_id + folder + imap_uid` 去重，因此全量同步可以重复执行，不会重复写入同一封邮件。全量同步进度记录在邮箱账号上，包括状态、总数、已处理数、新增数、开始时间、结束时间和错误信息。

用户从 Web 界面发送邮件时，后端先按当前用户校验邮箱账号归属，再使用账号的 SMTP 主机、端口、加密方式、用户名和加密凭据发信。SMTP 发送成功后，后端将生成的 RFC822 原文、纯文本正文、HTML 正文、附件文件和元数据写入本地数据目录，并在 `mail_messages` 中保存到账号配置的 `sent_folder`。当前发信支持普通附件和账号签名模板，暂不向 IMAP 服务器执行 append；后续可在同一发送服务上扩展服务器已发送追加和草稿链路。

邮件回复与转发复用同一套发信服务，但发送前应由后端基于来源邮件生成写信上下文。回复填写原发件人并生成 `In-Reply-To`、`References`；回复全部需要合并原发件人、收件人和抄送人，并排除当前用户自己的所有邮箱地址；转发默认不填收件人，正文中追加原邮件信息和引用内容。转发原附件时，前端只传附件 ID，后端按当前用户和来源邮件校验后从本地附件目录读取并重新组包，避免前端重复下载再上传。

回复和转发发送成功后，应在本地已发送邮件中保存来源邮件 ID、回复/转发模式和线程头，方便后续扩展会话聚合、发信追踪和草稿箱。

邮件保存成功或重复同步命中已有邮件时，后端会从发件人、收件人和抄送人中提取邮箱地址，写入 `contacts` 表。自动沉淀联系人按 `user_id + email_key` 去重，只补充空缺显示名并更新最近出现时间；如果用户已经手工维护过昵称、姓名、电话、公司或备注，自动同步不能覆盖这些字段。

全量同步支持用户主动停止。停止后状态标记为 `cancelled`，已同步到本地的邮件继续保留，后台任务会在当前 IMAP 批次返回后退出。服务启动时会把上次进程中遗留的 `running` 状态重置为失败，避免容器重启后页面长期显示假同步中。

同步后清理服务器旧邮件是高风险能力，默认关闭。只有全量同步成功后才允许执行，且只会删除已经本地保存、早于保留天数的 `INBOX` 邮件 UID；发件箱不会被清理。普通日常收取和定时增量任务不会删除服务器邮件。

### 3.6 worker 模块

负责后台收取任务：

- 服务启动后启动轻量后台调度器，默认每 1 分钟扫描一次启用的邮箱账号。
- 按邮箱账号 `poll_interval_minutes` 判断是否到期，只查询账号表，不扫描邮件表。
- 自动任务只执行日常增量收取，复用最近一批邮件拉取逻辑；全量历史同步必须由用户手动触发。
- 自动收取默认最多并发 2 个邮箱账号，避免 IMAP 拉取和 MIME 解析占用过多资源。
- 支持手动触发收取。
- 支持手动触发全量历史同步，并可查询同步进度。
- 支持停止全量历史同步。
- 记录任务执行结果。
- 避免同一个邮箱账号同时运行多个日常收取任务；手动收取与自动收取冲突时只允许一个执行。
- 升级和迁移必须优先保护本地用户数据，新增字段使用兼容默认值，不能用重建数据库或清空数据的方式完成升级。

## 4. 前端模块

### 4.1 登录注册

页面：

- `/login`
- `/register`

功能：

- 表单校验。
- 登录成功后保存 token 或 session 状态。
- 未登录访问业务页面时跳转登录页。

### 4.1.1 个人设置

页面：

- `/settings/profile`

功能：

- 展示用户名和邮箱。
- 修改昵称和个人描述。
- 上传头像并更新顶部用户区展示。
- 选择个人界面主题，保存后立即应用，并在下次登录后自动恢复。

### 4.2 主界面

建议主界面包含：

- 左侧导航：邮件、邮箱账号、联系人、规则、设置。
- 顶部用户区：头像、昵称或用户名、个人设置、修改密码、退出登录。
- 内容区：邮件列表、邮件详情、系统管理或配置表单。
- 管理员用户会看到“系统管理”入口，普通用户不展示该入口；后端仍以管理员鉴权为准，不能只依赖前端隐藏菜单。

### 4.3 邮箱账号管理

页面：

- 邮箱账号列表。
- 新增/编辑邮箱账号弹窗或页面。
- 连接测试按钮。
- 手动收取按钮。
- 全量同步按钮和同步进度展示。
- SMTP 发信配置，包括主机、端口、SSL/TLS、STARTTLS、用户名和密码或授权码。
- 同步后清理服务器旧邮件的开关与保留天数配置，默认关闭，并展示风险提示。

### 4.4 邮件查看

页面：

- 邮件列表。
- 邮件详情。
- 写邮件抽屉，可选择发件账号并填写收件人、抄送、密送、主题和正文。
- 邮件详情动作区，支持回复、回复全部和转发。
- 附件下载入口。

邮件列表和详情需要基于邮箱地址匹配通讯录，显示优先级为：联系人昵称、联系人姓名、邮件头显示名、邮箱用户名部分。详情中的联系人标签使用点击弹出层展示真实邮箱地址、电话、公司和备注，既减少列表噪音，也保留可追溯的地址信息。

回复、回复全部和转发应复用写邮件抽屉。前端只负责展示后端返回的写信上下文和收集用户编辑结果，不在浏览器里自行推导邮件线程头。阅读区宽度不足时，回复、回复全部和转发按钮应收进更多菜单，避免标题、联系人和按钮互相挤压。

### 4.5 联系人通讯录

页面：

- 联系人列表。
- 新增/编辑联系人弹窗。
- 搜索和分页。

功能：

- 支持按姓名、昵称、邮箱、电话、公司和备注搜索。
- 支持维护昵称、姓名、电话、公司和备注。
- 自动沉淀的联系人显示来源为“邮件发现”，用户编辑后按手工维护资料使用。

### 4.6 系统管理

页面：

- 用户概览仪表盘。
- 用户列表。
- 启用/停用用户操作。

功能：

- 展示用户总数、启用用户数、邮件总数和附件占用。
- 按用户展示邮箱账号数、邮件数、附件数、附件已知大小、联系人、文件夹、规则、最近邮件时间和最近同步时间。
- 管理员可停用或重新启用其他用户，不能停用当前登录的自己。
- 停用用户后，该用户再次登录会被拒绝，已有 token 访问邮箱、联系人、规则等页面也会被后端拒绝。

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
- `nickname`
- `avatar_path`
- `bio`
- `ui_theme`
- `is_admin`
- `enabled`
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
- `smtp_host`
- `smtp_port`
- `smtp_tls`
- `smtp_starttls`
- `smtp_username`
- `smtp_password_encrypted`
- `sent_folder`
- `signature_html`
- `poll_interval_minutes`
- `enabled`
- `last_sync_at`
- `last_sync_status`
- `last_sync_error`
- `full_sync_status`
- `full_sync_total`
- `full_sync_processed`
- `full_sync_new_count`
- `full_sync_started_at`
- `full_sync_finished_at`
- `full_sync_error`
- `cleanup_enabled`
- `cleanup_retention_days`
- `created_at`
- `updated_at`

### 5.3 mail_messages

- `id`
- `user_id`
- `account_id`
- `thread_id`
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
- `in_reply_to`
- `references_header`
- `source_message_id`
- `compose_mode`
- `created_at`
- `updated_at`

建议为 `account_id + folder + imap_uid` 建唯一约束，同时尽量保存 `message_id` 作为辅助去重依据。回复与转发阶段新增的线程字段必须使用兼容默认值迁移，避免影响已有邮件数据。
`thread_id` 指向邮件会话，历史邮件可通过重建接口补齐；字段允许为空，保证旧数据升级时不阻塞服务启动。

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

### 5.6 contacts

- `id`
- `user_id`
- `email`
- `email_key`
- `display_name`
- `nickname`
- `phone`
- `company`
- `notes`
- `source`
- `first_seen_at`
- `last_seen_at`
- `created_at`
- `updated_at`

`email_key` 使用规范化小写邮箱地址，建议对 `user_id + email_key` 建唯一约束。`source` 用于区分 `manual` 和 `auto`，自动联系人后续被用户编辑后视为手工维护。

### 5.7 mail_rules

- `id`
- `user_id`
- `name`
- `enabled`
- `match_mode`
- `priority`
- `stop_on_match`
- `action_type`
- `target_folder_id`
- `sort_order`
- `created_at`
- `updated_at`

### 5.8 mail_rule_conditions

- `id`
- `rule_id`
- `field`
- `operator`
- `value`

### 5.9 mail_threads

- `id`
- `user_id`
- `account_id`
- `root_message_id`
- `subject`
- `normalized_subject`
- `message_count`
- `unread_count`
- `has_attachments`
- `last_message_at`
- `created_at`
- `updated_at`

`mail_threads` 以 `user_id` 做硬隔离；标准邮件头优先归并，同主题弱聚合限制在同一邮箱账号内，避免不同账号的相同主题被误合并。建议索引 `user_id + last_message_at`、`user_id + account_id + normalized_subject`。

### 5.10 mail_rule_logs

- `id`
- `user_id`
- `rule_id`
- `rule_name`
- `message_id`
- `matched`
- `action_type`
- `target_folder_id`
- `trigger_type`
- `condition_snapshot_json`
- `result_status`
- `result_message`
- `created_at`

规则日志记录同步或手动应用规则时的命中、跳过和失败结果。`rule_name` 和 `condition_snapshot_json` 保存当时快照，即使后续删除或修改规则，历史记录仍可用于排障。建议索引 `user_id + created_at`、`user_id + message_id + created_at`、`user_id + rule_id + created_at`。

### 5.11 mail_sync_jobs

- `id`
- `user_id`
- `account_id`
- `trigger_type`
- `status`
- `started_at`
- `finished_at`
- `new_message_count`
- `error_message`

### 5.12 mail_message_states

- `id`
- `user_id`
- `message_id`
- `is_read`
- `read_at`
- `starred`
- `is_spam`
- `spam_at`
- `archived_at`
- `deleted_at`
- `created_at`
- `updated_at`

`mail_message_states` 保存 Mail Nest 本地阅读和整理状态，不回写远端 IMAP。建议对 `user_id + message_id` 建唯一约束，批量操作时使用 upsert 保证幂等。

### 5.13 mail_sync_job_events

- `id`
- `job_id`
- `level`
- `phase`
- `message`
- `detail_json`
- `created_at`

同步事件日志用于排障，`phase` 建议覆盖连接、目录列表、拉取、解析、入库、规则执行和服务器清理等阶段。日志内容必须脱敏。

### 5.14 附件中心索引

附件中心复用 `mail_attachments` 表，不单独复制附件文件。建议增加以下索引：

- `mail_attachments(user_id, filename)`
- `mail_attachments(user_id, content_type)`
- `mail_attachments(user_id, created_at)`
- `mail_attachments(user_id, inline, id)`

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
- 邮箱密码或授权码必须加密后存入当前配置的数据库。
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
