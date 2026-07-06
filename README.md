# Mail Nest（信匣）

Mail Nest（信匣）是一个计划中的本地化邮件收取与 Web 查看系统。它面向个人或小团队使用，核心目标是：用户可以在 Web 界面注册登录，配置多个邮箱账号，系统负责收取邮件并存储在本地，用户再通过浏览器查看自己的邮件。

## 核心目标

- 支持用户注册、登录和退出登录。
- 支持每个用户配置多个邮箱账号。
- 支持通过 IMAP/IMAPS 收取邮件。
- 支持 Web 界面查看邮件列表、邮件详情和附件信息。
- 支持用户数据隔离，每个用户只能看到自己的邮箱配置和邮件。
- 后端数据存储在本地目录，数据库优先使用 SQLite。
- 所有对外 JSON 接口统一使用标准 envelope。

## 推荐目录结构

```text
mailnest/
├── mailnest-be/              # Go 后端服务
├── mailnest-fe/              # Vue 3 + Vite + TypeScript 前端应用
├── doc/                      # 中文设计与实施文档
├── AGENTS.md                 # 工程协作约定
├── CHANGELOG.md              # 变更记录
└── README.md                 # 项目说明
```

## 技术栈建议

### 后端

- Golang
- SQLite
- 本地文件存储
- IMAP/IMAPS 邮件收取
- JWT 或服务端 session 登录态

### 前端

- Vue 3
- Vite
- TypeScript
- Ant Design Vue
- Vue Router
- Pinia

## 统一接口响应格式

所有对外 JSON 接口必须统一使用以下 envelope：

```json
{
  "success": true,
  "message": "",
  "httpCode": 200,
  "data": {}
}
```

- `success` 表示接口处理是否成功。
- `message` 表示给前端显示的说明信息。
- `httpCode` 必须与 HTTP 状态码保持一致。
- `data` 表示业务数据主体，没有数据时返回空对象。

新增接口时应优先复用后端统一响应封装，避免每个 handler 手写不同格式。

## 文档入口

- [工程协作约定](AGENTS.md)
- [产品需求说明](doc/requirements.md)
- [架构设计](doc/architecture.md)
- [接口规范](doc/api.md)
- [实施计划](doc/implementation-plan.md)
- [更新日志](CHANGELOG.md)

## 当前状态

当前仓库已完成文档初始化，并创建了后端与前端基础骨架：

- 后端已包含配置读取、SQLite 初始化、统一响应封装、JWT 注册登录基础接口、邮箱账号配置接口、邮箱凭据加密存储、IMAP 连接测试、手动收取、邮件列表和邮件详情接口。
- 前端已包含 Vue 3/Vite/TypeScript/Ant Design Vue 基础应用、登录注册页面、邮箱账号配置页面、连接测试、手动收取、邮件列表/详情和 envelope API 客户端。

下一步建议完善 Outlook/Gmail 等常见邮箱的配置提示、附件下载、收取任务后台调度和更完整的 MIME 解析。
