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

请求：

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

## 4. 邮箱账号接口

### 4.1 邮箱账号列表

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
        "enabled": true,
        "pollIntervalMinutes": 10,
        "lastSyncAt": "2026-07-06T10:00:00+08:00",
        "lastSyncStatus": "success"
      }
    ]
  }
}
```

### 4.2 创建邮箱账号

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
  "pollIntervalMinutes": 10,
  "enabled": true
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

### 4.3 更新邮箱账号

`PUT /api/v1/mail-accounts/{id}`

请求字段同创建接口。`imapPassword` 为空字符串或省略时，后端保留原已加密凭据；只有传入非空值时才重新加密并覆盖。

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
    "pollIntervalMinutes": 10,
    "enabled": true
  }
}
```

### 4.4 删除邮箱账号

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

### 4.5 测试邮箱连接

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

### 4.6 手动触发收取

`POST /api/v1/mail-accounts/{id}/sync`

响应：

```json
{
  "success": true,
  "message": "收取完成",
  "httpCode": 200,
  "data": {
    "jobId": "job-id",
    "newMessageCount": 1
  }
}
```

## 5. 邮件接口

### 5.1 邮件列表

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

### 5.2 邮件详情

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

### 5.3 下载附件

`GET /api/v1/messages/{messageId}/attachments/{attachmentId}/content`

该接口返回文件流。错误时仍返回统一 JSON envelope。

### 5.4 邮件列表查询参数

`GET /api/v1/messages` 支持：

- `keyword`：搜索主题、发件人、收件人、抄送和正文索引。
- `from`：按发件人过滤。
- `subject`：按主题过滤。
- `dateFrom` / `dateTo`：按日期范围过滤，格式 `YYYY-MM-DD`。
- `hasAttachments`：是否有附件。
- `accountId`：按邮箱账号过滤。
- `folderId`：按本地文件夹过滤。
- `systemFolder`：系统文件夹，支持 `inbox`、`all`、`attachments`。

### 5.5 邮件放入本地文件夹

`POST /api/v1/messages/{id}/folder`

请求：

```json
{
  "folderId": "folder-id"
}
```

`folderId` 为空字符串时表示移出本地文件夹。

## 6. 本地文件夹接口

### 6.1 文件夹列表

`GET /api/v1/mail-folders`

### 6.2 创建文件夹

`POST /api/v1/mail-folders`

```json
{
  "name": "安全通知",
  "color": "#1f66d1",
  "sortOrder": 10
}
```

### 6.3 删除文件夹

`DELETE /api/v1/mail-folders/{id}`

删除文件夹不会删除邮件，只会清空相关邮件的本地文件夹归属。

## 7. 邮件规则接口

### 7.1 规则列表

`GET /api/v1/mail-rules`

### 7.2 创建规则

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

### 7.3 删除规则

### 7.3 更新规则

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

### 7.4 删除规则

`DELETE /api/v1/mail-rules/{id}`

删除规则会同时删除规则条件，不会改变已经归档邮件的本地文件夹。

### 7.5 手动应用规则

`POST /api/v1/mail-rules/apply`

```json
{
  "scope": "unfiled"
}
```

`scope` 支持：

- `unfiled`：只处理未归档邮件。
- `all`：重新处理全部邮件并覆盖本地文件夹。

## 8. 收取任务接口

### 8.1 任务列表

`GET /api/v1/sync-jobs`

查询参数：

- `accountId`：可选。
- `page`：页码。
- `pageSize`：每页数量。

### 8.2 任务详情

`GET /api/v1/sync-jobs/{id}`

响应包含任务状态、触发方式、开始时间、结束时间、新增邮件数和错误信息。

## 9. OAuth 接口

### 9.1 开始 Microsoft 授权

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

### 9.2 完成 Microsoft 授权

`POST /api/v1/oauth/microsoft/complete`

请求：

```json
{
  "code": "authorization-code",
  "state": "oauth-state"
}
```

响应为创建后的邮箱账号。后端会把 access token 和 refresh token 加密存储。

## 10. 前端处理要求

- 前端 API 客户端必须统一解析 envelope。
- 当 `success=false` 时，优先显示 `message`。
- 当 HTTP 状态码为 `401` 时，跳转登录页。
- 前端业务页面不应直接依赖 HTTP 状态码判断业务数据结构，应读取 envelope 中的 `data`。
