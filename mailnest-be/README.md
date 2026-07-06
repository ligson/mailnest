# Mail Nest 后端

后端使用 Go 开发，当前已实现：

- `config.yaml` 配置读取，缺失配置文件时使用默认值。
- SQLite 本地数据库初始化和基础迁移。
- 统一 JSON envelope 响应封装。
- 用户注册、登录、当前用户、退出登录接口。
- JWT 登录态。
- `config.yaml` 控制是否开放注册。
- 邮箱账号创建、列表和删除接口。
- 邮箱密码或授权码加密后存入 SQLite。
- IMAP 连接测试接口。
- 手动收取 INBOX 邮件接口。
- 邮件列表和邮件详情接口。
- Microsoft OAuth2 授权接入，支持二步验证和禁用基础密码登录的 Outlook 账号。

## Outlook 说明

如果 Outlook 账号开启二步验证或禁用了基础密码登录，推荐使用 Microsoft OAuth2 授权。

需要先在 Microsoft Entra 管理中心注册应用，并在 `config.yaml` 中配置：

```yaml
oauth:
  microsoft:
    tenant: "consumers"
    clientId: "你的应用 client id"
    clientSecret: "你的应用 client secret"
    redirectUrl: "http://127.0.0.1:5173/oauth/microsoft/callback"
```

应用重定向地址需要包含：

```text
http://127.0.0.1:5173/oauth/microsoft/callback
```

授权 scope 包含：

```text
offline_access
https://outlook.office.com/IMAP.AccessAsUser.All
https://graph.microsoft.com/User.Read
```

如果仍使用密码模式且 Outlook 返回 `AUTHENTICATE failed.`，通常需要检查：

- Outlook 是否允许 IMAP。
- 当前账号是否禁用了基础密码登录。
- 是否需要使用应用专用密码或 OAuth 授权。
- 邮箱安全策略是否阻止第三方客户端登录。

## 启动

```bash
cp config.example.yaml config.yaml
go run ./cmd/mailnest
```

默认监听：

```text
http://127.0.0.1:8080
```

健康检查：

```text
GET /api/v1/health
```

## 测试

```bash
go test ./...
```
