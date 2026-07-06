# 邮件三栏界面、搜索过滤与规则归档实施计划

## 1. 目标

实现 `doc/mail-ui-and-rules-design.md` 中的 MVP：

- 邮件页改为三栏邮箱客户端。
- 邮件列表支持关键词、发件人、主题、日期、附件、账号和文件夹过滤。
- 增加本地文件夹。
- 增加规则归档，规则命中后把邮件放入本地文件夹。
- 新邮件收取后自动执行规则，支持手动对历史邮件应用规则。

## 2. 文件变更范围

### 后端

- `mailnest-be/internal/storage/storage.go`
  - 增加 `mail_folders`、`mail_rules`、`mail_rule_conditions` 迁移。
  - `mail_messages` 增加 `local_folder_id`、`search_text`。
  - 增加邮件搜索参数结构、文件夹 CRUD、规则 CRUD、规则条件查询与邮件归档更新方法。
- `mailnest-be/internal/mail/service.go`
  - 保存邮件时写入 `search_text`。
  - 插入新邮件或回填附件后应用规则。
- `mailnest-be/internal/mail/rules.go`
  - 新增规则匹配和应用逻辑。
- `mailnest-be/internal/api/app.go`
  - 扩展 `GET /api/v1/messages` 查询参数。
  - 增加 `/api/v1/mail-folders` 接口。
  - 增加 `/api/v1/mail-rules` 与 `/api/v1/mail-rules/apply` 接口。
- `mailnest-be/internal/api/auth_test.go`
  - 增加搜索过滤、用户隔离、文件夹和规则归档集成测试。

### 前端

- `mailnest-fe/src/api/client.ts`
  - 扩展邮件查询参数类型。
  - 增加文件夹和规则 API 类型与方法。
- `mailnest-fe/src/components/AppLayout.vue`
  - 增加规则管理入口。
- `mailnest-fe/src/router/index.ts`
  - 增加 `/rules` 路由。
- `mailnest-fe/src/views/DashboardView.vue`
  - 改造为三栏邮箱客户端。
- `mailnest-fe/src/views/MailRulesView.vue`
  - 新增规则管理页面。
- `mailnest-fe/src/styles.css`
  - 补充全局壳样式和三栏页面基础样式。
- `CHANGELOG.md`
  - 记录实现变更。

## 3. 实施步骤

### 阶段 A：后端搜索过滤

1. 在 `auth_test.go` 新增失败测试：
   - 同一用户收取多封邮件。
   - `keyword` 能匹配主题、发件人、正文。
   - `from`、`subject`、`dateFrom/dateTo`、`hasAttachments` 能过滤。
   - 第二个用户无法搜索到第一个用户的邮件。
2. 扩展 `storage.MailMessage` 和迁移，新增 `search_text`。
3. 保存邮件时生成 `search_text`。
4. 新增 `ListMailMessagesQuery`，重写 `ListMailMessages` 支持过滤。
5. 扩展 `handleListMessages` 解析查询参数。
6. 跑 `go test ./...`。

### 阶段 B：本地文件夹

1. 新增失败测试：
   - 用户可创建文件夹。
   - 删除文件夹不会删除邮件。
   - 按 `folderId` 过滤只返回当前用户邮件。
2. 增加 `mail_folders` 迁移和 `local_folder_id`。
3. 实现文件夹 CRUD 存储方法。
4. 实现 `/api/v1/mail-folders` 接口。
5. 跑 `go test ./...`。

### 阶段 C：规则归档

1. 新增失败测试：
   - 创建规则：主题包含“网络安全”且有附件，放入“安全通知”文件夹。
   - 同步新邮件后自动归档。
   - 手动应用规则可以处理历史邮件。
   - 多条规则命中时按 `sortOrder` 使用第一条。
2. 增加 `mail_rules` 和 `mail_rule_conditions` 迁移。
3. 实现规则 CRUD 存储方法。
4. 新增 `internal/mail/rules.go`，实现字段匹配、操作符匹配和规则应用。
5. 在 `Service.saveMessage` 新邮件插入成功后应用规则。
6. 实现 `/api/v1/mail-rules` 与 `/api/v1/mail-rules/apply` 接口。
7. 跑 `go test ./...`。

### 阶段 D：前端三栏邮件页

1. 扩展 `client.ts` 类型和 API。
2. `DashboardView.vue` 改为：
   - 左侧文件夹导航。
   - 中间搜索过滤和邮件列表。
   - 右侧常驻阅读区。
3. 搜索条件变化后刷新列表。
4. 选中邮件后右侧加载详情。
5. 保留附件下载。
6. 跑 `npm run build`。

### 阶段 E：前端规则管理页

1. 新增 `/rules` 路由和侧边栏入口。
2. 新增 `MailRulesView.vue`：
   - 规则列表。
   - 新建规则表单。
   - 条件编辑。
   - 目标文件夹选择。
   - 手动应用规则。
3. 跑 `npm run build`。

### 阶段 F：文档、验证和本地页面检查

1. 更新 `doc/api.md`。
2. 更新 `doc/architecture.md` 数据模型。
3. 更新 `CHANGELOG.md`。
4. 跑：
   - `go test ./...`
   - `npm run build`
5. 重启本地后端，刷新 Chrome 页面，检查：
   - 三栏布局显示正常。
   - 搜索过滤可用。
   - 附件邮件详情和下载仍可用。
   - 规则创建和手动应用可用。
