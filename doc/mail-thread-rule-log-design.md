# 邮件线程与规则命中记录设计

## 目标

本设计用于补齐 Mail Nest 下一批高频实用能力：

- 邮件线程：把同一主题或回复链中的邮件聚合成会话，降低查找上下文成本。
- 规则命中记录：记录规则为什么处理了某封邮件，便于排查误归档、误标记和垃圾邮件规则误判。

两个能力都必须遵守现有多用户隔离原则：普通用户只能查询自己的线程、邮件、规则和规则日志；管理员用户概览也不直接越权读取邮件正文。

## 一、邮件线程设计

### 1.1 用户场景

- 在邮件列表中用“会话视图”按线程查看往来邮件。
- 阅读邮件时可以看到同一会话内的历史邮件、最新回复和当前邮件位置。
- 回复、回复全部、转发后，本地已发送邮件能自动归入原会话。
- 旧邮件可通过后台任务或手动按钮重建线程。

### 1.2 线程归并策略

线程归并优先级从高到低：

1. `References` 中的任意 Message-ID 已存在本用户邮件中，则加入最靠近根部的已知线程。
2. `In-Reply-To` 指向的 Message-ID 已存在本用户邮件中，则加入该邮件线程。
3. 当前邮件的 `Message-ID` 已被其他邮件引用，则复用已有线程。
4. Mail Nest 内发起的回复或转发优先使用 `source_message_id` 对应邮件的线程。
5. 标准线程头缺失时，使用规范化主题作为弱聚合条件；弱聚合默认限制在同一用户、同一邮箱账号内，避免把不同账号或无关邮件误合并。

主题规范化规则：

- 去除大小写差异和首尾空白。
- 循环剥离 `Re:`、`回复:`、`答复:`、`Fwd:`、`Fw:`、`转发:` 等常见前缀。
- 合并连续空白。
- 主题为空时不进行弱聚合。

### 1.3 数据模型

新增 `mail_threads`：

| 字段 | 说明 |
| --- | --- |
| `id` | 主键 |
| `user_id` | 所属用户 |
| `account_id` | 默认主账号；弱聚合时用于限制范围 |
| `root_message_id` | 根邮件本地 ID，可为空 |
| `subject` | 展示主题 |
| `normalized_subject` | 规范化主题 |
| `message_count` | 线程邮件数量 |
| `unread_count` | 未读数量 |
| `has_attachments` | 线程内是否有附件 |
| `last_message_at` | 最近一封邮件时间 |
| `created_at` / `updated_at` | 创建和更新时间 |

扩展 `mail_messages`：

| 字段 | 说明 |
| --- | --- |
| `thread_id` | 所属线程 ID，可为空 |

建议索引：

- `mail_threads(user_id, last_message_at)`
- `mail_threads(user_id, account_id, normalized_subject)`
- `mail_messages(user_id, thread_id)`
- `mail_messages(user_id, message_id)`

迁移策略：

- 新库由 GORM model 自动建表和补列。
- MySQL 既有生产库沿用安全迁移策略，只显式创建 `mail_threads` 和补充 `mail_messages.thread_id`，避免对历史大表执行可能缩列的自动迁移。
- 首次上线不强制同步重建全部线程；提供后台重建入口，避免启动时长时间阻塞。

### 1.4 后端流程

邮件入库时：

1. 解析并保存 `Message-ID`、`In-Reply-To`、`References`。
2. 调用 `ResolveThreadForMessage(userID, message)` 查找或创建线程。
3. 写回 `mail_messages.thread_id`。
4. 汇总更新线程的 `message_count`、`unread_count`、`has_attachments` 和 `last_message_at`。

发信保存本地已发送邮件时：

1. 若请求包含 `sourceMessageId`，优先复用来源邮件的 `thread_id`。
2. 同时继续写入 `In-Reply-To`、`References`，保证后续 IMAP 同步回来的已发送邮件也能归并。

线程重建：

- 对用户邮件按时间升序扫描。
- 先处理带标准线程头的邮件，再处理弱主题归并。
- 每批处理固定数量，写入同步事件或后台日志，避免一次请求阻塞过久。

### 1.5 接口设计

#### 线程列表

`GET /api/v1/threads`

查询参数：

- `accountId`
- `keyword`
- `systemFolder`
- `folderId`
- `hasAttachment`
- `unread`
- `page`
- `pageSize`

响应 `data`：

```json
{
  "items": [
    {
      "id": "thread-id",
      "subject": "项目通知",
      "messageCount": 4,
      "unreadCount": 1,
      "hasAttachments": true,
      "lastMessageAt": "2026-07-23T16:20:00+08:00",
      "participants": [
        {
          "name": "张三",
          "email": "zhangsan@example.com"
        }
      ],
      "latestMessage": {
        "id": "message-id",
        "from": "张三 <zhangsan@example.com>",
        "preview": "请确认附件..."
      }
    }
  ],
  "total": 12,
  "page": 1,
  "pageSize": 20
}
```

#### 线程详情

`GET /api/v1/threads/{id}`

响应包含线程基础信息和按时间升序排列的邮件摘要。正文仍复用现有 `GET /api/v1/messages/{id}`，避免一次性加载大正文和内嵌图片。

#### 重建线程

`POST /api/v1/threads/rebuild`

请求：

```json
{
  "scope": "all",
  "accountId": "account-id"
}
```

`scope` 支持：

- `empty`：只处理 `thread_id` 为空的邮件。
- `all`：清空并重建当前用户全部线程。

### 1.6 前端交互

邮件页顶部增加视图切换：

- `邮件`：保持现有单封邮件列表。
- `会话`：展示线程列表，未读数和邮件数作为轻量标签。

会话列表项展示：

- 主题、参与人摘要、最新邮件预览。
- 邮件数量、未读数、附件标识、最新时间。

阅读区：

- 进入会话后展示时间线。
- 默认展开最新邮件和当前点击邮件，其余邮件折叠。
- 回复、回复全部、转发仍复用现有写信弹窗，并带入当前邮件上下文。

## 二、规则命中记录设计

### 2.1 用户场景

- 邮件详情中查看“这封邮件被哪条规则处理过”。
- 规则管理页看到最近命中时间、命中次数和最近处理样例。
- 调试垃圾邮件规则时，可以看到命中条件快照和执行结果。
- 手动应用历史邮件规则后，知道处理了多少封、失败多少封。

### 2.2 数据模型

新增 `mail_rule_logs`：

| 字段 | 说明 |
| --- | --- |
| `id` | 主键 |
| `user_id` | 所属用户 |
| `rule_id` | 规则 ID |
| `message_id` | 邮件 ID |
| `matched` | 是否命中 |
| `action_type` | 执行动作 |
| `target_folder_id` | 目标文件夹，可为空 |
| `trigger_type` | `sync`、`manual`、`preview` |
| `condition_snapshot_json` | 当时规则条件快照 |
| `result_status` | `applied`、`skipped`、`failed` |
| `result_message` | 简短结果说明 |
| `created_at` | 记录时间 |

建议索引：

- `mail_rule_logs(user_id, created_at)`
- `mail_rule_logs(user_id, message_id, created_at)`
- `mail_rule_logs(user_id, rule_id, created_at)`

保留策略：

- 首期默认保留最近 90 天或每用户最近 20000 条。
- 后续在系统管理中增加清理策略配置。

### 2.3 日志记录时机

同步新邮件应用规则：

- 只记录命中的规则。
- 对 `stop_on_match` 导致后续规则未执行的，不写未命中日志，避免日志噪音。

手动应用规则：

- 记录命中并执行的规则。
- 如果规则命中但因为 `overwrite=false` 且邮件已有本地文件夹而跳过，记录 `result_status=skipped`。
- 如果动作执行失败，记录 `result_status=failed` 和脱敏错误信息。

规则预览：

- 默认不写日志。
- 后续如果需要审计预览，可通过 `recordPreview=true` 参数打开。

### 2.4 接口设计

#### 日志列表

`GET /api/v1/rule-logs`

查询参数：

- `messageId`
- `ruleId`
- `resultStatus`
- `triggerType`
- `page`
- `pageSize`

#### 邮件规则日志

`GET /api/v1/messages/{id}/rule-logs`

用于邮件详情侧查询当前邮件的规则处理历史。

#### 规则列表增强

`GET /api/v1/mail-rules`

每条规则额外返回：

- `hitCount`
- `lastHitAt`
- `lastResultStatus`

### 2.5 前端交互

邮件详情：

- 在状态/附件区域增加“规则记录”入口。
- 抽屉或弹窗展示规则名称、动作、结果、时间和条件快照。

规则管理页：

- 表格增加“最近命中”“命中次数”“最近结果”列。
- 每条规则增加“查看记录”操作。
- 手动应用规则完成后展示应用数量和失败数量，并提供查看日志入口。

垃圾邮件规则调试：

- 对 `mark_spam` 动作展示“已标记垃圾邮件”。
- 支持从垃圾邮件视图进入规则记录，快速判断是哪条规则导致。

## 三、实现拆分

### 阶段 A：规则命中记录

优先做规则日志，因为数据结构独立，风险低，且能直接服务垃圾邮件规则排查。

任务：

1. 新增 `mail_rule_logs` model、迁移和 storage CRUD。
2. 修改规则执行链路，记录命中、跳过和失败。
3. 新增日志查询接口和 presenter。
4. 规则列表补充命中统计。
5. 前端规则页增加命中统计和日志入口。
6. 邮件详情增加规则记录入口。

验收：

- 用户只能看到自己的规则日志。
- 收取新邮件触发规则后能查询到命中记录。
- 手动应用规则能记录 applied、skipped、failed。
- 删除规则后历史日志保留规则名称快照，前端仍能展示。

### 阶段 B：邮件线程

任务：

1. 新增 `mail_threads` 和 `mail_messages.thread_id`。
2. 实现主题规范化和线程归并服务。
3. 在邮件入库、发信保存、历史重建中写入线程关系。
4. 新增线程列表、详情和重建接口。
5. 前端邮件页增加“邮件/会话”视图切换和会话时间线。

验收：

- 标准 `References` / `In-Reply-To` 邮件能归入同一线程。
- Mail Nest 内回复发送后的本地已发送邮件能归入来源线程。
- 无标准头但主题相同的邮件只在同账号内弱聚合。
- 用户不能访问其他用户线程。
- 大量历史邮件重建不会阻塞服务启动。

## 四、测试计划

后端测试：

- 线程主题规范化测试。
- 标准线程头归并测试。
- `source_message_id` 优先归并测试。
- 弱主题同账号聚合和跨账号不聚合测试。
- 规则日志写入、查询、用户隔离测试。
- 规则执行 `applied`、`skipped`、`failed` 状态测试。
- MySQL 既有表安全迁移测试。

前端测试：

- `rtk npm run build` 类型检查。
- 邮件/会话视图切换不破坏现有邮件列表。
- 规则页日志弹窗和邮件详情规则记录入口可用。

生产验证：

- 部署前备份 `docker-compose.yml`、`config.yaml` 和数据目录。
- 部署后验证健康检查、登录、邮件列表、规则列表、垃圾邮件视图和草稿箱不回退。
- 先对单个账号执行 `empty` 线程重建，再视情况执行全量重建。
