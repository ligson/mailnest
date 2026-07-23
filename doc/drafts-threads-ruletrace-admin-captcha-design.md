# 草稿箱、邮件线程、规则命中、验证码与系统管理设计

## 目标

本设计覆盖两个阶段：

- 第一优先级设计：草稿箱、邮件线程、规则命中记录。
- 本次落地能力：登录/注册图形验证码、管理员用户管理与用户存储统计。

所有能力继续遵守 Mail Nest 的多用户隔离原则。除管理员查看系统概览外，普通用户只能访问自己的邮箱账号、邮件、附件、规则和联系人。

## 草稿箱设计

### 场景

- 写新邮件、回复、回复全部、转发过程中关闭弹窗或页面刷新，不丢正文。
- 用户可在草稿箱继续编辑、发送或删除草稿。
- 附件草稿需要能保存普通附件元数据和临时文件。

### 数据模型建议

新增 `mail_drafts`：

- `id`
- `user_id`
- `account_id`
- `compose_mode`：`new`、`reply`、`replyAll`、`forward`
- `source_message_id`
- `to_addrs`、`cc_addrs`、`bcc_addrs`
- `subject`
- `text_body`
- `html_body`
- `forward_attachment_ids`
- `last_saved_at`
- `created_at`、`updated_at`

新增 `mail_draft_attachments`：

- `id`
- `user_id`
- `draft_id`
- `filename`
- `content_type`
- `size`
- `file_path`
- `created_at`

### 接口建议

- `GET /api/v1/drafts`
- `POST /api/v1/drafts`
- `PUT /api/v1/drafts/{id}`
- `DELETE /api/v1/drafts/{id}`
- `POST /api/v1/drafts/{id}/send`

### 前端交互

- 写信弹窗每 10 秒自动保存一次；关闭前如果有内容，提示保存草稿。
- 草稿箱作为系统文件夹展示。
- 发送成功后自动删除对应草稿。

## 邮件线程设计

### 场景

- 同一封邮件的回复链在阅读区聚合展示。
- 回复/转发时能回到原始上下文。

### 线程归并规则

优先级从高到低：

1. `References` 中包含的 Message-ID。
2. `In-Reply-To` 指向的 Message-ID。
3. 当前邮件自身 `Message-ID`。
4. 对没有标准头的邮件，使用规范化主题作为弱聚合条件，但默认不跨账号合并。

### 数据模型建议

新增 `mail_threads`：

- `id`
- `user_id`
- `root_message_id`
- `subject`
- `message_count`
- `last_message_at`
- `created_at`、`updated_at`

在 `mail_messages` 增加：

- `thread_id`

### 接口建议

- `GET /api/v1/threads`
- `GET /api/v1/threads/{id}`
- `POST /api/v1/messages/{id}/rebuild-thread`

### 前端交互

- 邮件列表可切换“邮件视图/会话视图”。
- 阅读区顶部展示线程中全部邮件，当前邮件高亮。

## 规则命中记录设计

### 场景

- 用户知道某封邮件为什么被移动、标记已读、加星或进垃圾邮件。
- 调试规则时能看到命中条件和执行动作。

### 数据模型建议

新增 `mail_rule_logs`：

- `id`
- `user_id`
- `rule_id`
- `message_id`
- `matched`
- `action_type`
- `target_folder_id`
- `condition_snapshot`
- `result_message`
- `created_at`

### 接口建议

- `GET /api/v1/rule-logs?messageId=&ruleId=`
- `GET /api/v1/messages/{id}/rule-logs`

### 前端交互

- 邮件详情增加“规则记录”入口。
- 规则页增加最近命中数量和最近命中时间。

## 图形验证码设计

### 本次实现

- 新增 `GET /api/v1/auth/captcha`。
- 返回 `id`、`imageData`、`expireSeconds`。
- 登录和注册请求必须提交 `captchaId`、`captchaAnswer`。
- 验证码保存在后端内存中，5 分钟过期，一次性使用。

### 安全边界

- 验证码只降低简单脚本爆破风险，不替代登录限流。
- 后续建议增加按 IP 和账号的失败次数限流。

## 系统管理设计

### 本次实现

- 用户表增加 `is_admin`、`enabled`。
- 旧库迁移时默认所有用户启用，最早创建的用户自动成为管理员。
- 首个注册用户自动成为管理员。
- 管理员可查看用户列表、启用/停用用户。
- 管理员可查看用户级统计：邮箱账号数、邮件数、附件数、附件大小、联系人、文件夹、规则、最近邮件/同步时间。

### 接口

- `GET /api/v1/admin/users`
- `PUT /api/v1/admin/users/{id}/enabled`

### 权限

- 前端只对管理员显示“系统管理”菜单。
- 后端通过 `adminMiddleware` 强制校验管理员权限。
- 停用用户不能继续登录，已有 token 请求也会被鉴权中间件拒绝。
- 管理员不能停用当前登录账号。

## 验收

- 登录和注册页必须显示验证码，验证码错误时不能登录或注册。
- 获取新验证码后旧验证码不能重复使用。
- 第一个用户具备管理员权限。
- 普通用户访问管理接口返回 403。
- 停用用户登录返回明确错误。
- 管理员页可查看用户数据概览并启停其他用户。
