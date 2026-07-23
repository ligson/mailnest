# Mail Nest（信匣）

Mail Nest（信匣）是一个计划中的本地化邮件收取与 Web 查看系统。它面向个人或小团队使用，核心目标是：用户可以在 Web 界面注册登录，配置多个邮箱账号，系统负责收取邮件并存储在本地，用户再通过浏览器查看自己的邮件。

## 核心目标

- 支持用户注册、登录和退出登录。
- 支持每个用户配置多个邮箱账号。
- 支持通过 IMAP/IMAPS 收取邮件。
- 支持 Web 界面查看邮件列表、邮件详情和附件信息。
- 支持用户数据隔离，每个用户只能看到自己的邮箱配置和邮件。
- 后端数据存储在本地目录，数据库默认使用 SQLite，也可通过 GORM 配置切换到 MySQL；PostgreSQL 已预留连接和迁移能力。
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
- GORM + SQLite / MySQL / PostgreSQL
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
- [后端包结构说明](doc/backend-package-structure.md)
- [实施计划](doc/implementation-plan.md)
- [邮件批量操作、规则增强、同步日志与附件中心设计](doc/mail-batch-rules-sync-attachments-design.md)
- [邮件线程与规则命中记录设计](doc/mail-thread-rule-log-design.md)
- [垃圾邮件系统文件夹与规则标记设计](doc/mail-spam-design.md)
- [邮件回复与转发功能设计](doc/reply-forward-design.md)
- [更新日志](CHANGELOG.md)

## Docker Compose 部署

仓库提供非敏感部署模板：

- `mailnest-be/docker/Dockerfile`：后端生产镜像。
- `mailnest-fe/Dockerfile`：前端生产镜像。
- `mailnest-fe/docker/nginx.conf`：前端静态资源服务，并将 `/api/` 代理到后端容器。
- `docker/docker-compose.yml`：NAS 单机部署示例。
- `docker/config.example.yaml`：后端生产配置示例。

真实 `docker/config.yaml`、运行数据、镜像包和本地 `.env` 均已加入 `.gitignore`，不要提交。

后端数据库通过 `config.yaml` 的 `database` 节配置，连接和建表迁移由 GORM 统一处理。生产默认仍建议从 SQLite 起步，并确保数据库文件和邮件数据目录都挂载到持久化目录：

```yaml
database:
  driver: "sqlite" # sqlite/mysql/postgres
  path: "/data/mailnest.db" # sqlite 使用
  dsn: "" # mysql/postgres 使用
  maxOpenConns: 4
  maxIdleConns: 4
```

MySQL 示例 DSN：

```text
mailnest:password@tcp(127.0.0.1:3306)/mailnest?parseTime=true&charset=utf8mb4&loc=Local
```

PostgreSQL 示例 DSN：

```text
postgres://mailnest:password@127.0.0.1:5432/mailnest?sslmode=disable
```

部署时可以通过环境变量指定镜像版本：

- `MAILNEST_BE_IMAGE_TAG`：后端镜像标签。
- `MAILNEST_FE_IMAGE_TAG`：前端镜像标签。
- `MAILNEST_IMAGE_TAG`：兼容旧部署方式，同时作为前后端默认标签。

当前后端和前端可以独立升级，避免只发布一侧镜像时被同一个 tag 绑定。

## 当前状态

当前仓库已完成文档初始化，并创建了后端与前端基础骨架：

- 后端已包含配置读取、可配置数据库初始化、统一响应封装、JWT 注册登录基础接口、邮箱账号配置接口、邮箱凭据加密存储、IMAP 连接测试、手动收取、邮件列表/详情、联系人、SMTP 发信、回复和转发接口。
- 前端已包含 Vue 3/Vite/TypeScript/Ant Design Vue 基础应用、登录注册页面、邮箱账号配置页面、连接测试、手动收取、邮件列表/详情、联系人维护、写邮件抽屉、回复/回复全部/转发入口和 envelope API 客户端。

下一步建议按设计推进规则命中记录和邮件会话视图，并继续完善 Outlook/Gmail 等常见邮箱的配置提示。
