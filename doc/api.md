# Mail Nest API 规范

## 1. 基础约定

- API 前缀：`/api/v1`
- 请求和响应编码：`UTF-8`
- JSON 响应统一使用 envelope。
- 需要登录的接口必须携带登录凭据。
- `httpCode` 字段必须与实际 HTTP 状态码一致。

## 2. 统一响应 Envelope

成功示例：

```json
{
  "success": true,
  "message": "操作成功",
  "httpCode": 200,
  "data": {
    "id": "example-id"
  }
}
```

失败示例：

```json
{
  "success": false,
  "message": "未登录或登录已过期",
  "httpCode": 401,
  "data": {}
}
```

没有业务数据时：

```json
{
  "success": true,
  "message": "操作成功",
  "httpCode": 200,
  "data": {}
}
```

## 3. 鉴权接口

### 3.1 注册

`POST /api/v1/auth/register`

JSON 请求：

```json
{
  "username": "demo",
  "email": "demo@example.com",
  "password": "password"
}
```

响应：

```json
{
  "success": true,
  "message": "注册成功",
  "httpCode": 201,
  "data": {
    "user": {
      "id": "user-id",
      "username": "demo",
      "email": "demo@example.com"
    },
    "token": "jwt-token"
  }
}
```

### 3.2 登录

`POST /api/v1/auth/login`

请求：

```json
{
  "account": "demo",
  "password": "password"
}
```

响应：

```json
{
  "success": true,
  "message": "登录成功",
  "httpCode": 200,
  "data": {
    "user": {
      "id": "user-id",
      "username": "demo",
      "email": "demo@example.com"
    },
    "token": "jwt-token"
  }
}
```

### 3.3 当前用户

`GET /api/v1/auth/me`

响应：

```json
{
  "success": true,
  "message": "获取成功",
  "httpCode": 200,
  "data": {
    "id": "user-id",
    "username": "demo",
    "email": "demo@example.com"
  }
}
```

### 3.4 退出登录

`POST /api/v1/auth/logout`

响应：

```json
{
  "success": true,
  "message": "退出成功",
  "httpCode": 200,
  "data": {}
}
```

### 3.5 修改密码

`POST /api/v1/auth/change-password`

需要登录。修改成功后当前 JWT 继续有效，后续登录必须使用新密码。

请求：

```json
{
  "currentPassword": "old-password",
  "newPassword": "new-password",
  "confirmPassword": "new-password"
}
```

响应：

```json
{
  "success": true,
  "message": "密码修改成功",
  "httpCode": 200,
  "data": {}
}
```

校验规则：

- 当前密码必须正确。
- 新密码至少 8 位。
- 新密码和确认新密码必须一致。
- 新密码不能与当前密码相同。
- 数据库只保存新密码的 `bcrypt` 哈希。

### 3.6 获取个人资料

`GET /api/v1/profile`

需要登录。响应：

```json
{
  "success": true,
  "message": "获取成功",
  "httpCode": 200,
  "data": {
    "id": "user-id",
    "username": "demo",
    "email": "demo@example.com",
    "nickname": "信匣用户",
    "bio": "用 Mail Nest 管理邮件",
    "avatarUrl": "/api/v1/profile/avatar/content"
  }
}
```

### 3.7 修改个人资料

`PUT /api/v1/profile`

请求：

```json
{
  "nickname": "信匣用户",
  "bio": "用 Mail Nest 管理邮件"
}
```

校验规则：

- 昵称最多 40 个字符。
- 个人描述最多 200 个字符。
- 用户名和邮箱暂不通过该接口修改。

### 3.8 上传头像

`POST /api/v1/profile/avatar`

请求类型：`multipart/form-data`

字段：

- `avatar`：图片文件，支持 PNG、JPG、WEBP、GIF，最大 2MB。

头像保存到本地数据目录下的用户资料目录，响应返回更新后的个人资料。

### 3.9 读取头像

`GET /api/v1/profile/avatar/content`

需要登录。读取当前用户自己的头像文件，不暴露本地文件路径。

## 4. 联系人接口

联系人接口均需要登录，所有查询和写入都按当前用户 `user_id` 隔离。邮箱地址按大小写无关方式去重。

### 4.1 联系人列表

`GET /api/v1/contacts`

查询参数：

- `keyword`：可选，按邮箱、姓名、昵称、电话、公司和备注模糊搜索。
- `page`：页码，默认 `1`。
- `pageSize`：每页数量，默认 `100`，最大 `1000`。

响应：

```json
{
  "success": true,
  "message": "获取成功",
  "httpCode": 200,
  "data": {
    "items": [
      {
        "id": "contact-id",
        "email": "alice@example.com",
        "displayName": "Alice Zhang",
        "nickname": "阿丽",
        "name": "阿丽",
        "phone": "123456",
        "company": "Example Inc.",
        "notes": "重要客户",
        "source": "manual",
        "firstSeenAt": "2026-07-14T10:00:00+08:00",
        "lastSeenAt": "2026-07-14T10:00:00+08:00",
        "createdAt": "2026-07-14T10:00:00+08:00",
        "updatedAt": "2026-07-14T10:00:00+08:00"
      }
    ],
    "page": 1,
    "pageSize": 100,
    "total": 1
  }
}
```

### 4.2 创建联系人

`POST /api/v1/contacts`

请求：

```json
{
  "email": "alice@example.com",
  "displayName": "Alice Zhang",
  "nickname": "阿丽",
  "phone": "123456",
  "company": "Example Inc.",
  "notes": "重要客户"
}
```

校验规则：

- `email` 必填，必须是合法邮箱地址，也可以传入 `Alice <alice@example.com>`，后端会规范化为邮箱地址。
- `displayName` 和 `nickname` 最多 80 个字符。
- `phone` 最多 40 个字符。
- `company` 最多 120 个字符。
- `notes` 最多 500 个字符。

响应：`201 Created`，`data` 为联系人对象。

### 4.3 更新联系人

`PUT /api/v1/contacts/{id}`

请求字段同创建联系人。更新后联系人来源会视为 `manual`，后续自动邮件沉淀不会覆盖用户维护的昵称、姓名、电话、公司和备注。

响应：`200 OK`，`data` 为更新后的联系人对象。

### 4.4 删除联系人

`DELETE /api/v1/contacts/{id}`

删除联系人不会删除任何邮件，只是不再用该联系人信息优化邮件地址显示。

响应：

```json
{
  "success": true,
  "message": "删除成功",
  "httpCode": 200,
  "data": {}
}
```

## 5. 邮箱账号接口

### 5.1 邮箱账号列表

`GET /api/v1/mail-accounts`

响应：

```json
{
  "success": true,
  "message": "获取成功",
  "httpCode": 200,
  "data": {
    "items": [
      {
        "id": "account-id",
        "displayName": "工作邮箱",
        "email": "demo@example.com",
        "imapHost": "imap.example.com",
        "imapPort": 993,
        "imapTls": true,
        "imapUsername": "demo@example.com",
        "smtpHost": "smtp.example.com",
        "smtpPort": 587,
        "smtpTls": false,
        "smtpStartTls": true,
        "smtpUsername": "demo@example.com",
        "smtpConfigured": true,
        "sentFolder": "Sent",
        "enabled": true,
        "pollIntervalMinutes": 10,
        "lastSyncAt": "2026-07-06T10:00:00+08:00",
        "lastSyncStatus": "success",
        "fullSyncStatus": "success",
        "fullSyncTotal": 1200,
        "fullSyncProcessed": 1200,
        "fullSyncNewCount": 1180,
        "fullSyncStartedAt": "2026-07-07T10:00:00+08:00",
        "fullSyncFinishedAt": "2026-07-07T10:12:00+08:00",
        "fullSyncError": null,
        "cleanupEnabled": false,
        "cleanupRetentionDays": 90
      }
    ]
  }
}
```

### 5.2 创建邮箱账号

`POST /api/v1/mail-accounts`

请求：

```json
{
  "displayName": "工作邮箱",
  "email": "demo@example.com",
  "imapHost": "imap.example.com",
  "imapPort": 993,
  "imapTls": true,
  "imapUsername": "demo@example.com",
  "imapPassword": "mail-password-or-auth-code",
  "smtpHost": "smtp.example.com",
  "smtpPort": 587,
  "smtpTls": false,
  "smtpStartTls": true,
  "smtpUsername": "demo@example.com",
  "smtpPassword": "smtp-password-or-auth-code",
  "smtpUseImapPassword": true,
  "sentFolder": "Sent",
  "pollIntervalMinutes": 10,
  "enabled": true,
  "cleanupEnabled": false,
  "cleanupRetentionDays": 90
}
```

响应：

```json
{
  "success": true,
  "message": "创建成功",
  "httpCode": 201,
  "data": {
    "id": "account-id"
  }
}
```

### 5.3 更新邮箱账号

`PUT /api/v1/mail-accounts/{id}`

请求字段同创建接口。`imapPassword` 和 `smtpPassword` 为空字符串或省略时，后端保留原已加密凭据；只有传入非空值时才重新加密并覆盖。`smtpUseImapPassword=true` 时，后端会把本次 IMAP 密码或原 IMAP 密码作为 SMTP 凭据保存一份加密副本。

响应：

```json
{
  "success": true,
  "message": "更新成功",
  "httpCode": 200,
  "data": {
    "id": "account-id",
    "displayName": "工作邮箱",
    "email": "demo@example.com",
    "imapHost": "imap.example.com",
    "imapPort": 993,
    "imapTls": true,
    "imapUsername": "demo@example.com",
    "smtpHost": "smtp.example.com",
    "smtpPort": 587,
    "smtpTls": false,
    "smtpStartTls": true,
    "smtpUsername": "demo@example.com",
    "smtpConfigured": true,
    "sentFolder": "Sent",
    "pollIntervalMinutes": 10,
    "enabled": true,
    "cleanupEnabled": false,
    "cleanupRetentionDays": 90
  }
}
```

### 5.4 删除邮箱账号

`DELETE /api/v1/mail-accounts/{id}`

响应：

```json
{
  "success": true,
  "message": "删除成功",
  "httpCode": 200,
  "data": {}
}
```

### 5.5 测试邮箱连接

`POST /api/v1/mail-accounts/{id}/test-connection`

响应：

```json
{
  "success": true,
  "message": "连接成功",
  "httpCode": 200,
  "data": {}
}
```

### 5.6 读取邮箱 IMAP 文件夹

`GET /api/v1/mail-accounts/{id}/folders`

该接口使用已保存的邮箱凭据登录 IMAP 服务器并执行文件夹列表读取，用于前端选择真实的发件箱目录。不同邮箱服务商的已发送目录可能叫 `Sent`、`Sent Items`、`Sent Messages`、`已发送邮件` 等，不能只依赖默认值。

响应：

```json
{
  "success": true,
  "message": "获取成功",
  "httpCode": 200,
  "data": {
    "items": [
      {
        "name": "INBOX",
        "delimiter": "/",
        "attributes": ["\\HasNoChildren"],
        "sentCandidate": false
      },
      {
        "name": "已发送邮件",
        "delimiter": "/",
        "attributes": ["\\Sent"],
        "sentCandidate": true
      }
    ]
  }
}
```

### 5.7 手动触发收取

`POST /api/v1/mail-accounts/{id}/sync`

该接口用于普通手动收取，当前实现会拉取 `INBOX` 和账号配置的发件箱文件夹最近一批邮件，适合日常增量更新。需要补齐历史邮件时使用全量同步接口。如果配置的非 `INBOX` 文件夹不存在，后端会跳过该文件夹并在 `warnings` 中返回说明，避免影响收件箱同步。

响应：

```json
{
  "success": true,
  "message": "收取完成",
  "httpCode": 200,
  "data": {
    "jobId": "job-id",
    "newMessageCount": 1,
    "warnings": []
  }
}
```

### 5.8 启动全量历史同步

`POST /api/v1/mail-accounts/{id}/full-sync/start`

该接口启动后台任务，按 IMAP UID 分批同步 `INBOX` 和账号配置的发件箱文件夹全部历史邮件。接口立即返回 `202`，前端应通过同步状态接口轮询进度。重复启动正在运行的全量同步时，后端返回当前任务状态，不再启动第二个任务。非 `INBOX` 文件夹不存在时会跳过该文件夹，避免全量同步完全失败。

响应：

```json
{
  "success": true,
  "message": "已开始全量同步",
  "httpCode": 202,
  "data": {
    "fullSyncStatus": "running",
    "fullSyncTotal": 0,
    "fullSyncProcessed": 0,
    "fullSyncNewCount": 0,
    "fullSyncStartedAt": "2026-07-07T10:00:00+08:00",
    "fullSyncFinishedAt": null,
    "fullSyncError": null,
    "cleanupEnabled": false,
    "cleanupRetentionDays": 90
  }
}
```

### 5.9 停止全量历史同步

`POST /api/v1/mail-accounts/{id}/full-sync/stop`

该接口用于请求停止正在运行的全量同步。停止后，已经成功保存到本地的邮件会保留，后续可以重新启动全量同步继续补齐。由于 IMAP 请求本身可能正在等待服务端响应，后端会先把状态标记为 `cancelled`，后台任务在当前批次返回后退出。

响应：

```json
{
  "success": true,
  "message": "已停止全量同步",
  "httpCode": 200,
  "data": {
    "fullSyncStatus": "cancelled",
    "fullSyncTotal": 1200,
    "fullSyncProcessed": 50,
    "fullSyncNewCount": 50,
    "fullSyncStartedAt": "2026-07-08T10:00:00+08:00",
    "fullSyncFinishedAt": "2026-07-08T10:03:00+08:00",
    "fullSyncError": "用户停止了全量同步",
    "cleanupEnabled": false,
    "cleanupRetentionDays": 90
  }
}
```

### 5.10 查询同步状态

`GET /api/v1/mail-accounts/{id}/sync-status`

响应：

```json
{
  "success": true,
  "message": "获取成功",
  "httpCode": 200,
  "data": {
    "fullSyncStatus": "success",
    "fullSyncTotal": 1200,
    "fullSyncProcessed": 1200,
    "fullSyncNewCount": 1180,
    "fullSyncStartedAt": "2026-07-07T10:00:00+08:00",
    "fullSyncFinishedAt": "2026-07-07T10:12:00+08:00",
    "fullSyncError": null,
    "cleanupEnabled": false,
    "cleanupRetentionDays": 90
  }
}
```

### 5.11 同步后清理服务器旧邮件

邮箱账号可配置：

- `cleanupEnabled`：是否在全量同步成功后清理服务器旧邮件，默认 `false`。
- `cleanupRetentionDays`：服务器保留天数，默认 `90`。

清理只在全量同步成功后执行，只删除已经保存到本地数据库、位于 `INBOX`、并且早于保留天数的邮件 UID。普通手动收取和定时增量收取不会触发服务器删除。该动作会通过 IMAP 标记 `\Deleted` 并执行 `EXPUNGE`，属于真实删除，前端必须给出明确风险提示。

## 6. 邮件接口

### 6.1 邮件列表

`GET /api/v1/messages`

查询参数：

- `accountId`：可选，按邮箱账号过滤。
- `keyword`：可选，按主题或发件人简单过滤。
- `page`：页码。
- `pageSize`：每页数量。

响应：

```json
{
  "success": true,
  "message": "获取成功",
  "httpCode": 200,
  "data": {
    "items": [
      {
        "id": "message-id",
        "accountId": "account-id",
        "subject": "欢迎使用 Mail Nest",
        "from": "sender@example.com",
        "to": ["demo@example.com"],
        "sentAt": "2026-07-06T10:00:00+08:00",
        "hasAttachments": true
      }
    ],
    "page": 1,
    "pageSize": 20,
    "total": 1
  }
}
```

### 6.2 邮件详情

`GET /api/v1/messages/{id}`

响应：

```json
{
  "success": true,
  "message": "获取成功",
  "httpCode": 200,
  "data": {
    "id": "message-id",
    "accountId": "account-id",
    "subject": "欢迎使用 Mail Nest",
    "from": "sender@example.com",
    "to": ["demo@example.com"],
    "cc": [],
    "sentAt": "2026-07-06T10:00:00+08:00",
    "textBody": "纯文本正文",
    "htmlBody": "<p>HTML 正文</p>",
    "attachments": [
      {
        "id": "attachment-id",
        "filename": "hello.txt",
        "contentType": "text/plain",
        "size": 128
      }
    ]
  }
}
```

### 6.3 发送邮件

`POST /api/v1/messages/send`

请求：

```json
{
  "accountId": "account-id",
  "to": ["好友 <friend@example.com>"],
  "cc": ["copy@example.com"],
  "bcc": [],
  "subject": "会议纪要",
  "textBody": "这是纯文本正文",
  "htmlBody": ""
}
```

带附件时使用 `multipart/form-data`：

- `accountId`：发件账号 ID。
- `to` / `cc` / `bcc`：JSON 字符串数组，例如 `["好友 <friend@example.com>"]`。
- `subject`：主题。
- `textBody`：纯文本正文。
- `htmlBody`：HTML 富文本正文。
- `attachments`：可重复文件字段。

说明：

- 需要登录，且 `accountId` 必须属于当前用户。
- 发信使用邮箱账号中的 SMTP 配置；SMTP 密码或授权码加密保存在数据库中，响应不会返回明文。
- `to`、`cc`、`bcc` 至少一个非空；后端按标准邮件地址解析并校验。
- 当前实现支持纯文本正文、HTML 正文和普通附件；附件数量最多 20 个，总大小最多 25MB。
- SMTP 发送成功后，后端会把该邮件保存到本地已发送目录，目录名使用账号配置的 `sentFolder`，邮件列表可通过 `systemFolder=sent` 查询。
- 发送成功后会自动沉淀收件人、抄送人和密送人为联系人；本地保存的邮件不会在邮件头中展示密送人。

响应：

```json
{
  "success": true,
  "message": "发送成功",
  "httpCode": 200,
  "data": {
    "id": "message-id",
    "accountId": "account-id",
    "subject": "会议纪要",
    "from": "发件人 <sender@example.com>",
    "to": ["好友 <friend@example.com>"],
    "sentAt": "2026-07-16T10:00:00+08:00",
    "hasAttachments": false
  }
}
```

### 6.4 下载附件

`GET /api/v1/messages/{messageId}/attachments/{attachmentId}/content`

该接口返回文件流。错误时仍返回统一 JSON envelope。

邮件详情中的 `cid:` 内嵌图片会被后端改写为短期签名图片地址：

`GET /api/v1/messages/{messageId}/attachments/{attachmentId}/inline-content?uid={userId}&exp={timestamp}&sig={signature}`

该地址只用于 HTML 正文内嵌图片展示，不要求额外传 `Authorization` 请求头，但必须携带有效签名和未过期时间戳。普通附件下载仍使用受登录保护的 `content` 接口。

### 6.5 邮件列表查询参数

`GET /api/v1/messages` 支持：

- `keyword`：搜索主题、发件人、收件人、抄送和正文索引。
- `from`：按发件人过滤。
- `subject`：按主题过滤。
- `dateFrom` / `dateTo`：按日期范围过滤，格式 `YYYY-MM-DD`。
- `hasAttachments`：是否有附件。
- `accountId`：按邮箱账号过滤。
- `folderId`：按本地文件夹过滤。
- `systemFolder`：系统文件夹，支持 `inbox`、`sent`、`all`、`attachments`。

### 6.6 邮件放入本地文件夹

`POST /api/v1/messages/{id}/folder`

请求：

```json
{
  "folderId": "folder-id"
}
```

`folderId` 为空字符串时表示移出本地文件夹。

## 7. 本地文件夹接口

### 7.1 文件夹列表

`GET /api/v1/mail-folders`

### 7.2 创建文件夹

`POST /api/v1/mail-folders`

```json
{
  "name": "安全通知",
  "color": "#1f66d1",
  "sortOrder": 10
}
```

### 7.3 删除文件夹

`DELETE /api/v1/mail-folders/{id}`

删除文件夹不会删除邮件，只会清空相关邮件的本地文件夹归属。

## 8. 邮件规则接口

### 8.1 规则列表

`GET /api/v1/mail-rules`

### 8.2 创建规则

`POST /api/v1/mail-rules`

```json
{
  "name": "安全通知归档",
  "enabled": true,
  "matchMode": "all",
  "targetFolderId": "folder-id",
  "sortOrder": 10,
  "conditions": [
    {
      "field": "subject",
      "operator": "contains",
      "value": "网络安全"
    }
  ]
}
```

### 8.3 更新规则

`PUT /api/v1/mail-rules/{id}`

请求字段同创建接口。更新时会整体替换规则条件，避免旧条件残留。

响应：

```json
{
  "success": true,
  "message": "更新成功",
  "httpCode": 200,
  "data": {
    "id": "rule-id",
    "name": "安全通知归档",
    "enabled": true,
    "matchMode": "all",
    "targetFolderId": "folder-id",
    "sortOrder": 10,
    "conditions": [
      {
        "id": "condition-id",
        "field": "subject",
        "operator": "contains",
        "value": "网络安全"
      }
    ]
  }
}
```

### 8.4 删除规则

`DELETE /api/v1/mail-rules/{id}`

删除规则会同时删除规则条件，不会改变已经归档邮件的本地文件夹。

### 8.5 手动应用规则

`POST /api/v1/mail-rules/apply`

```json
{
  "scope": "unfiled"
}
```

`scope` 支持：

- `unfiled`：只处理未归档邮件。
- `all`：重新处理全部邮件并覆盖本地文件夹。

## 9. 收取任务接口

### 9.1 任务列表

`GET /api/v1/sync-jobs`

查询参数：

- `accountId`：可选。
- `page`：页码。
- `pageSize`：每页数量。

### 9.2 任务详情

`GET /api/v1/sync-jobs/{id}`

响应包含任务状态、触发方式、开始时间、结束时间、新增邮件数和错误信息。

## 10. OAuth 接口

### 10.1 开始 Microsoft 授权

`POST /api/v1/oauth/microsoft/start`

响应：

```json
{
  "success": true,
  "message": "获取成功",
  "httpCode": 200,
  "data": {
    "state": "oauth-state",
    "authUrl": "https://login.microsoftonline.com/..."
  }
}
```

前端拿到 `authUrl` 后跳转到 Microsoft 授权页。

### 10.2 完成 Microsoft 授权

`POST /api/v1/oauth/microsoft/complete`

请求：

```json
{
  "code": "authorization-code",
  "state": "oauth-state"
}
```

响应为创建后的邮箱账号。后端会把 access token 和 refresh token 加密存储。

## 11. 前端处理要求

- 前端 API 客户端必须统一解析 envelope。
- 当 `success=false` 时，优先显示 `message`。
- 当 HTTP 状态码为 `401` 时，跳转登录页。
- 前端业务页面不应直接依赖 HTTP 状态码判断业务数据结构，应读取 envelope 中的 `data`。
