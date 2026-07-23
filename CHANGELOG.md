# 更新日志

所有重要变更都应记录在本文件中。本文档尽量使用中文，便于后续回忆需求和设计决策。

## 2026-07-23

### 变更

- 后端数据库连接和 schema 迁移替换为 GORM：通过 `gorm.io/driver/sqlite`、`gorm.io/driver/mysql`、`gorm.io/driver/postgres` 统一选择数据库驱动，底层仍暴露 `*sql.DB` 兼容现有复杂查询。
- 数据库建表、补列和普通索引迁移改为 GORM model + `AutoMigrate`，新增表和字段优先维护 model 标签，不再为 SQLite/MySQL/PostgreSQL 分别手写整套 DDL。
- 邮件列表表达式排序索引保留少量集中补充 SQL，兼顾跨库维护成本和列表性能。

### 文档

- 更新 README、架构和实施计划文档，明确数据库迁移由 GORM 处理，默认 SQLite，可配置 MySQL/PostgreSQL。

### 测试

- 后端 `rtk go test ./...` 通过，覆盖 GORM AutoMigrate 后的现有 storage、API 和 mail 服务行为。

## 2026-07-22

### 变更

- 后端数据库层从写死 SQLite 重构为可配置方言：`config.yaml` 新增 `database.driver`、`dsn`、连接池配置，默认继续使用 SQLite，同时支持 MySQL，并预留 PostgreSQL 方言层。
- storage 层保留统一数据库封装，集中处理 PostgreSQL 占位符转换、自增 ID 返回、插入忽略、联系人 upsert 和定时同步到期 SQL，避免业务 handler 或存储方法手写数据库专属语法。
- MySQL/PostgreSQL 新库初始化使用独立 portable schema，补齐邮件列表、附件中心、同步日志和联系人常用索引；SQLite 旧迁移保持兼容，避免影响现有本地数据库。
- 文件夹列表的规则数量统计改为子查询，移除 SQLite 宽松 `GROUP BY` 依赖，兼容 MySQL `ONLY_FULL_GROUP_BY` 和 PostgreSQL。

### 文档

- 更新 README、架构、需求、实施计划和邮件界面设计文档，说明数据库默认 SQLite、可配置 MySQL、预留 PostgreSQL，以及不同数据库的 DSN 配置方式。

### 测试

- 新增 storage 方言层单元测试，覆盖数据库配置归一化、SQLite 路径处理、PostgreSQL 占位符改写、插入忽略 SQL、联系人 upsert SQL 和定时同步到期条件。

## 2026-07-21

### 新增

- 写邮件富文本编辑器新增字体、字号、删除线、清除格式、字体颜色、背景色、正文图片插入和粘贴图片能力。

### 修复

- 修复回复、回复全部和转发上下文中中文联系人展示名被显示为 `=?utf-8?q?...?=` 编码词的问题；后端返回给前端的收件人、抄送人和转发正文头信息统一使用可读展示格式，发信时仍由 SMTP 组包逻辑编码为标准邮件头。
- 修复回复、回复全部和转发上下文未优先使用通讯录名称的问题；收件人、抄送人和转发引用头现在按“联系人昵称、联系人姓名、邮件头姓名、邮箱地址”的顺序生成展示名。
- 修复邮件页在较窄视口下三栏布局横向溢出、右侧详情被截断、搜索按钮和高级筛选控件错位的问题。
- 修复邮件列表项多选框缺少定位导致发件人、主题区域被顶乱的问题。
- 修复写邮件富文本编辑器选中文字后调整字号、字体、文字颜色和背景色不生效的问题；格式应用改为基于已保存选区包裹内联样式，避免下拉框抢焦点导致选区丢失。
- 修复写邮件背景色无法取消的问题，背景色面板新增“无背景”操作，只清理背景样式并保留其他文字格式。

### 优化

- 简化邮件列表顶部搜索和批量操作区域：默认只保留主搜索框和轻量筛选入口，日期、状态、附件和星标过滤折叠到高级筛选面板；批量操作未选中邮件时只显示选择状态，选中后展示常用动作并将低频动作收进“更多”菜单。
- 优化邮件页响应式宽度策略：窗口变窄或拖拽调整三栏宽度时，会自动给右侧阅读区保留可用宽度；搜索范围、输入框、搜索按钮保持一体化显示，高级筛选在窄栏下自动换行。
- 优化写邮件编辑器选区保存逻辑，先选中文字再点击工具栏时能正确应用样式；图片正文支持最大 3MB 内联插入，超过时提示改用附件。
- 统一个人设置页内容区框架，去除独立限宽外壳，让页面面板宽度、留白和滚动行为与联系人、规则、邮箱账号等页面保持一致。
- 邮件页左侧自定义文件夹恢复为只显示文件夹名称，不再直接展示 `规则 N`；规则关联数量继续保留在接口中，用于删除保护和规则管理提示。

### 测试

- 更新后端写信上下文测试，覆盖带 RFC 2047 编码词的发件人、收件人和抄送人在回复、回复全部、转发场景下正确解码展示。
- 更新后端写信上下文测试，覆盖通讯录昵称/姓名优先覆盖邮件头原始显示名。
- 前端 `rtk npm run build` 通过。

### 部署

- 将后端版本 `20260721104800-191cc5e-addrdecode` 部署到 生产环境 的 Mail Nest Docker Compose 服务；本地 Docker 构建 amd64 镜像后通过 `docker save | ssh docker load` 导入远端，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260721104800-addrdecode.tgz`；线上健康检查、后端容器镜像标签和启动日志验证通过。
- 将后端版本 `20260721110938-191cc5e-contactname` 部署到 生产环境 的 Mail Nest Docker Compose 服务；本地 Docker 构建 amd64 镜像后通过 `docker save | ssh docker load` 导入远端，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260721110938-contactname.tgz`；线上健康检查、后端容器镜像标签和启动日志验证通过。
- 将前端版本 `20260721114654-191cc5e-mailtoolbar` 部署到 生产环境 的 Mail Nest Docker Compose 服务；本地 Docker 构建 amd64 镜像后通过 `docker save | ssh docker load` 导入远端，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260721114654-mailtoolbar.tgz`；线上健康检查、`/mail` 静态资源、前端容器镜像标签和启动日志验证通过。
- 将前端版本 `20260721165650-191cc5e-layoutfix` 部署到 生产环境 的 Mail Nest Docker Compose 服务；本地 Docker 构建 amd64 镜像后通过 `docker save | ssh docker load` 导入远端，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260721165650-layoutfix.tgz`；线上健康检查、`/mail` 静态资源和前端容器镜像标签验证通过。
- 将前端版本 `20260721194834-191cc5e-profilelayout` 部署到 生产环境 的 Mail Nest Docker Compose 服务；本地 Docker 构建 amd64 镜像后通过 `docker save | ssh docker load` 导入远端，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260721194848-profilelayout.tgz`；线上健康检查、`/settings/profile` 静态资源、前端容器镜像标签和个人设置页 CSS 验证通过。
- 将富文本编辑器前端镜像 `20260721194920-191cc5e-rich-editor` 导入 生产环境，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260721194920-rich-editor.tgz`；并行前端任务使用 `20260721194834-191cc5e-profilelayout` 标签运行，但两个标签指向同一镜像 ID，线上 `/mail` 已加载包含富文本编辑器的新静态资源，健康检查通过。
- 将前端版本 `20260721194907-191cc5e-foldernav` 部署到 生产环境 的 Mail Nest Docker Compose 服务；本地 Docker 构建 amd64 镜像后通过 `docker save | ssh docker load` 导入远端，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260721194907-foldernav.tgz`；线上健康检查、前端容器镜像标签和 Chrome 实页验证通过，邮件页自定义文件夹仅显示名称与编辑/删除操作，不再展示 `规则 1`。
- 将前端版本 `20260721201605-191cc5e-editor-selection` 部署到 生产环境 的 Mail Nest Docker Compose 服务；本地 Docker 构建 amd64 镜像后通过 `docker save | ssh docker load` 导入远端，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260721201605-editor-selection.tgz`；线上健康检查、`/mail` 静态资源和前端容器镜像标签验证通过。

## 2026-07-20

### 优化

- 写邮件、回复、回复全部和转发统一改为居中大弹窗；回复/回复全部/转发点击后会立即打开弹窗并显示“正在准备邮件”加载状态，避免等待写信上下文接口时无反馈。
- 富文本编辑器工具栏改为自动换行，正文编辑区禁止横向滚动并强制长文本换行，减少水平滚动条带来的割裂感。
- 邮箱账号页新增启用/停用快捷开关；停用账号仍保留在账号管理页便于重新启用。
- 邮件页左侧账号筛选和写邮件发件账号只展示启用账号，停用账号不会再出现在邮件页账号列表中。
- 修复写邮件、回复和转发弹窗在大屏下过宽、表单控件溢出和原生文件选择控件露出的问题，改用稳定的收件人网格布局并限制弹窗内容宽度。
- 优化邮件详情加载性能：打开详情时不再等待已读状态写库，特殊格式内嵌图片转换增加大小上限，避免大附件被 base64 塞入详情响应导致前端 15 秒超时。
- 邮件详情请求改为支持取消旧请求并单独使用更长超时，快速切换邮件时不再让已取消的旧请求弹出超时提示或覆盖当前详情。

### 测试

- 前端 `rtk npm run build` 通过。
- 新增后端测试覆盖大尺寸特殊格式内嵌图片不会进入邮件详情 JSON，防止大邮件详情响应过慢。

### 部署

- 将前端版本 `20260720104739-191cc5e-account-enable` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260720104750-account-enable.tgz`；线上健康检查、首页静态资源和容器镜像标签验证通过。
- 将前端版本 `20260720111428-191cc5e-composefix` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260720111445-composefix.tgz`；线上健康检查、首页静态资源、容器镜像标签和写邮件弹窗视觉检查通过。
- 将前后端版本 `20260720145724-191cc5e-detailfast` 部署到 生产环境 的 Mail Nest Docker Compose 服务；本地 Docker 构建 amd64 镜像后通过 `docker save | ssh docker load` 导入远端，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260720145724-detailfast.tgz`；线上健康检查、`/mail` 静态资源、容器镜像标签和启动日志验证通过。

## 2026-07-19

### 新增

- 新增垃圾邮件系统文件夹与规则动作，支持通过 `mark_spam` / `unmark_spam` 批量操作和 `systemFolder=spam` 查看。
- 新增《垃圾邮件系统文件夹与规则标记设计》文档，明确垃圾邮件仅作为本地状态，不回写远端 IMAP。
- 本地邮件文件夹新增编辑能力，支持修改名称和颜色，后端新增 `PUT /api/v1/mail-folders/{id}`。
- 新增《邮件回复与转发功能设计》文档，明确回复、回复全部、转发、引用正文、线程头、转发附件和联系人沉淀规则。
- 后端落地回复、回复全部和转发能力：新增写信上下文接口，发送接口支持 `composeMode`、`sourceMessageId` 和 `forwardAttachmentIds`，SMTP 原文写入 `In-Reply-To` 与 `References`，并支持从本地安全读取原邮件普通附件重新组包转发。
- 前端邮件详情新增“回复”“回复全部”“转发”按钮，复用写邮件抽屉预填收件人、主题、引用正文和转发附件勾选列表。
- 邮件页新增批量操作能力，支持批量标记已读/未读、加星/取消星标、移动文件夹、删除和回收站恢复，并新增星标邮件、回收站、未读/星标筛选。
- 新增附件中心页面 `/attachments`，支持按附件名、内容类型、账号、文件夹、日期范围集中检索附件，并可直接下载或跳回原始邮件。
- 邮箱账号页新增同步日志中心，支持查看收取/全量同步任务列表、任务状态、事件流和阶段性错误信息。
- 规则能力增强，支持优先级、匹配后停止、动作类型、附件类条件、已读/星标条件和规则预览接口。

### 优化

- 文件夹列表返回关联规则数量 `ruleCount`，邮件页左侧显示规则数量；仍被规则引用的文件夹不允许直接删除，需要先调整或删除相关规则。
- 邮件页将邮箱账号筛选从顶部筛选栏移动到左侧栏，支持直接切换全部账号或单个账号，并保留与邮箱目录、本地文件夹和其他搜索条件的组合筛选。
- 规则页保存参数补齐 `priority`、`stopOnMatch` 和 `actionType`，保持现有“移动到文件夹”交互不变并满足后端规则模型。
- Docker Compose 部署模板支持 `MAILNEST_BE_IMAGE_TAG` 和 `MAILNEST_FE_IMAGE_TAG` 分别指定前后端镜像标签，避免只升级一侧时被全局标签绑定。
- 邮件详情接口和列表返回补齐 `isRead`、`starred`、`deletedAt`，打开详情时会自动回写已读状态。
- 同步任务写入阶段事件日志，手动收取和全量同步都可通过 `/api/v1/sync-jobs` 与 `/api/v1/sync-jobs/{id}/events` 追踪执行过程。

### 文档

- 新增《邮件批量操作、规则增强、同步日志与附件中心设计》，补充四个高频实用功能的目标场景、数据模型、接口草案、前端交互和实施顺序。
- 完善 API、架构、需求和实施计划文档，补充批量操作、规则预览、同步事件日志和附件中心的落地约定。
- 更新 API 与邮件界面设计文档，明确自定义文件夹是规则归档目标，不是孤立目录。
- 更新 README、需求、架构、API 和实施计划文档，补充回复/转发规划接口、数据模型扩展、前端交互和测试验收点。

### 测试

- 新增后端测试，覆盖规则标记垃圾邮件、历史邮件重新应用规则、批量标记/取消垃圾邮件和 `systemFolder=spam` 过滤。
- 新增后端 API 测试，覆盖文件夹编辑、规则关联计数和有关联规则时禁止删除。
- 新增后端 API 测试，覆盖批量操作、附件中心、同步任务事件和增强规则预览；本地验证 `rtk go test ./...` 54 条通过、`rtk npm run build` 通过。
- 新增后端服务层测试，覆盖回复全部排除当前用户自己的邮箱地址、写信上下文生成、回复线程头和转发原附件重新组包；后端 `rtk go test ./...` 通过，前端 `rtk npm run build` 通过。

### 部署

- 将后端版本 `20260719153500-191cc5e-spamfix` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260719152523-spam.tgz` 与 `backups/pre-20260719152523-spam.tgz` 之外的补充备份；修复旧库迁移顺序后，线上健康检查与后端容器状态验证通过。
- 将前后端版本 `20260719134547-191cc5e-fourfeatures` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260719142700-fourfeatures.tgz`；因远端直接拉取 Docker Hub 超时，最终采用本地构建镜像后通过 `docker save | ssh sudo /usr/local/bin/docker load` 导入远端，再执行 `sudo /usr/local/bin/docker compose up -d` 完成切换；线上健康检查、首页静态资源版本和容器镜像标签验证通过。
- 将后端版本 `20260719124449-191cc5e-folderedit` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml` 到 `backups/docker-compose.yml.pre-folderedit-2026071913*`；线上容器运行镜像已切换为 `ligson/mailnest-be:20260719124449-191cc5e-folderedit`，健康检查正常，`PUT /api/v1/mail-folders/{id}` 已从 `405` 修复为受鉴权保护的接口，浏览器文件夹编辑保存验证通过。
- 将前端账号筛选调整部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260719121138-191cc5e-account-left-20260719121157.tgz`；最终线上前端运行标签为 `20260719122623-191cc5e-roundshell`，与本次 `20260719121138-191cc5e-account-left` 构建同镜像 ID，远端镜像架构为 `linux/amd64`，容器内首页静态资源和后端健康接口访问验证通过。
- 将前后端版本 `20260719134159-191cc5e-replyforward` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260719134159-191cc5e-replyforward.tgz`；线上后端健康接口 `https://mailnest.ligson.xyz/api/v1/health` 返回正常，前端容器首页静态资源访问验证通过。

## 2026-07-18

### 新增

- 写邮件功能新增普通附件发送：前端支持选择多个附件，后端支持 `multipart/form-data` 发信请求，SMTP 原文使用 `multipart/mixed` 组包，并在发送成功后把附件保存到本地已发送邮件。
- 写邮件抽屉新增轻量富文本编辑器，支持添加附件、插入签名、加粗、斜体、下划线、项目/编号列表、对齐和插入链接。
- 邮箱账号新增 `signature_html` 签名模板字段；账号编辑弹窗新增“签名”标签页，可维护 HTML 签名并预览，写邮件时按发件账号自动插入或手动插入签名。
- 新增用户级界面主题偏好，个人设置页可在松林、晴空、葡萄、暖木、石墨、青花、朱砂、水墨和黛山 9 套主题中选择，保存后按用户持久化并在登录后自动应用。

### 优化

- 统一前端滚动条视觉样式，覆盖页面内容区、邮件三栏、写邮件编辑器、弹窗、抽屉、表格和下拉菜单等滚动容器，减少系统默认滚动条造成的割裂感。
- 前端主框架、邮件阅读区、账号编辑弹窗和个人设置页接入主题变量，主题会影响背景、边框、选中态、侧边栏和关键操作色。
- 优化主题切换后的主框架一致性，顶部栏和左侧导航统一使用主题框架色，右上角用户信息按钮改为低对比半透明样式，减少白色胶囊按钮的突兀感。
- 新增 Mail Nest 定制图标，并同步用于浏览器标签栏 favicon 和左侧品牌区，替换默认线性邮件图标。
- 优化右侧内容工作区外框，四个外角改为主题框架色露出的圆角画布，弱化内容区和导航框架之间的硬切割。
- 移除顶部栏底部分割线，使内容工作区上方圆角与主题框架色衔接得更自然。
- 优化邮件列表接口 `GET /api/v1/messages`：列表页改为只读取展示所需的摘要字段，避免无意义加载正文搜索索引、正文路径和原文路径等详情字段。
- 优化 SQLite 连接配置：将 WAL、busy timeout、同步模式和临时表内存设置写入连接 DSN，并限制连接池规模，降低自动收取写入和页面读取并发时的偶发等待。
- 为附件表新增 `(user_id, message_id, inline, id)` 索引，减少邮件详情读取附件列表时的扫描和临时排序。
- 后端为邮件列表和邮件详情增加慢接口分段日志，超过 500ms 时记录查询、正文读取、CID 重写等耗时，便于线上继续定位真实瓶颈。
- 将历史邮件正文修复任务改为后台执行，避免服务启动时先扫描和修复旧邮件导致网关短暂 502。
- 优化邮件详情 `cid:` 内嵌图片输出：JPEG/PNG/WebP 等浏览器可直接显示的图片改为短期签名 URL，不再 base64 塞入详情 JSON，修复多张内嵌截图邮件详情响应体膨胀到数 MB 导致浏览器打开慢的问题。

### 测试

- 新增后端服务层测试，覆盖 SMTP 附件组包、本地已发送附件保存，以及邮箱账号签名模板持久化。
- 新增后端存储层测试，覆盖邮件列表摘要查询不会加载 `search_text`，同时邮件详情查询仍保留完整字段。
- 更新后端详情测试，覆盖 `cid:` 图片改写为签名内嵌图片 URL、无登录态可凭签名读取图片，以及篡改签名会被拒绝。

### 部署

- 将后端版本 `20260718115642-4e0e307-perf2` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260718-perf-20260718114825.tgz`，线上健康检查、附件索引迁移和邮件列表/详情接口耗时验证通过。
- 将后端版本 `20260718123318-4e0e307-inlineurl` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260718-inlineurl-20260718123319.tgz`；线上验证 `messages/134879` 详情响应从约 5.7MB 降至约 107KB，公网请求耗时从约 4-5 秒降至约 0.3 秒，签名内嵌图片访问返回正常。
- 将前后端版本 `20260718130805-4e0e307-compose-rich` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260718-compose-rich-20260718130806.tgz`；线上健康检查、前端首页访问、`mail_accounts.signature_html` 迁移和账号接口签名字段验证通过。
- 将前后端版本 `20260718163145-191cc5e-themes` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260718163145-191cc5e-themes-20260718163207.tgz`；线上健康检查、前端静态资源访问和 `users.ui_theme` 迁移验证通过。
- 将前后端版本 `20260718165148-191cc5e-themefix` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260718165148-191cc5e-themefix-20260718165212.tgz`；线上健康检查、前端新版静态资源访问、容器镜像标签和 `users.ui_theme` 字段验证通过。
- 将前端版本 `20260719121147-191cc5e-brandicon` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260719121147-191cc5e-brandicon-20260719121211.tgz`；线上健康检查、首页 favicon 引用、`mailnest-icon.svg` 资源和前端容器镜像标签验证通过。
- 将前端版本 `20260719122623-191cc5e-roundshell` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260719122623-191cc5e-roundshell-20260719122659.tgz`；线上健康检查、前端新版静态资源访问、容器镜像标签和内容区圆角 CSS 验证通过。
- 将前端版本 `20260719123712-191cc5e-headerline` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260719123712-191cc5e-headerline-20260719123732.tgz`；线上健康检查、前端新版静态资源访问、容器镜像标签和顶部栏无底部分割线 CSS 验证通过。

## 2026-07-16

### 新增

- 后端新增 SMTP 发信能力：邮箱账号增加 SMTP 主机、端口、SSL/TLS、STARTTLS、用户名和加密密码字段，并新增 `POST /api/v1/messages/send` 发信接口。
- 发信成功后会生成 RFC822 原文并保存到本地已发送目录，邮件页“发件箱”可立即查看从 Mail Nest 发出的邮件。
- 发信链路会自动沉淀收件人、抄送人和密送人为联系人，继续遵守不覆盖手工维护资料的规则。
- 前端邮箱账号编辑弹框新增 SMTP 发信配置区；邮件页新增“写邮件”抽屉，支持选择发件账号、填写收件人、抄送、密送、主题和正文。

### 测试

- 新增后端服务层测试，覆盖 SMTP 凭据解密、发信后保存已发送邮件以及密送联系人沉淀。

### 文档

- 更新需求、架构、API 和实施计划文档，补充 SMTP 账号字段、发信接口、发送后本地保存行为，以及当前不支持发信附件的边界。

### 部署

- 将版本 `20260716154232-4e0e307-sendmail` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260716154232-4e0e307-sendmail-20260716155948.tgz`，线上健康检查、前后端容器互通和 `mail_accounts` SMTP 字段迁移验证通过。
- 将前端版本 `20260717131227-4e0e307-accountform` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260717131227-4e0e307-accountform-20260717134200.tgz`，线上健康检查、前端容器启动和静态资源版本验证通过。
- 将前端版本 `20260717164344-4e0e307-accounttabs` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/` 到 `backups/pre-20260717164344-4e0e307-accounttabs-20260717164636.tgz`，线上健康检查、前端容器启动和静态资源版本验证通过。

### 优化

- 重新整理邮箱账号新增/编辑弹窗布局，改为“基础信息 / 收信 IMAP / 发信 SMTP / 同步”四个标签页；字段改为稳定双列布局，并将发件箱目录选择收进收信页高级设置，修复“读取目录”按钮错位和主界面信息过载的问题。

## 2026-07-14

### 新增

- 后端新增联系人通讯录能力：新增 `contacts` 表和 `/api/v1/contacts` 列表、新增、更新、删除接口，按当前登录用户隔离，并按邮箱地址大小写无关去重。
- 邮件收取链路会从发件人、收件人和抄送人自动沉淀联系人，自动联系人只更新最近出现时间和空缺显示名，不覆盖用户手工维护的昵称、姓名、电话、公司和备注。
- 前端新增“联系人”页面，支持搜索、分页、新增、编辑和删除联系人，可维护昵称、姓名、电话、公司和备注。
- 邮件列表和邮件详情接入通讯录显示逻辑，优先显示联系人昵称或姓名；邮件详情联系人标签可点击查看真实邮箱地址和联系信息。

### 优化

- 邮件详情联系人弹窗新增简洁编辑图标，可一键跳转到联系人页并自动打开对应联系人编辑弹窗；联系人不存在时会预填邮箱进入新增联系人。
- 优化应用整体滚动布局：左侧导航和顶部用户栏固定，右侧页面视口在所有页面保持统一尺寸，滚动只发生在页面视口内部；邮件页三栏内部各自承担滚动。

### 测试

- 新增后端测试覆盖联系人 CRUD 用户隔离，以及自动沉淀联系人不会覆盖手工维护资料。

### 文档

- 更新需求、架构、API 和实施计划文档，补充联系人通讯录设计、接口字段、自动沉淀规则和显示优先级。

### 部署

- 将版本 `20260714144753-4e0e307-contacts` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/`，线上健康检查、联系人路由、联系人接口鉴权和 `contacts` 表迁移验证通过。
- 将前端版本 `20260714152300-4e0e307-contactedit` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/`，线上健康检查和联系人编辑跳转路由验证通过。
- 将前端版本 `20260714154611-4e0e307-layoutfix` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/`，线上健康检查以及邮件、联系人路由验证通过。
- 将前端版本 `20260714161220-4e0e307-viewport` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/`，线上健康检查以及邮件、联系人路由验证通过。

## 2026-07-10

### 优化

- 优化邮件页首屏加载体验：邮箱账号、文件夹和邮件列表改为并行加载，首次加载期间显示“加载中”和骨架屏，避免先显示 `0 封邮件` 与空结果造成误解。
- 邮件列表加载完成后自动打开当前页第一封邮件，减少右侧阅读区长时间空白。
- 后端为邮件列表常用筛选和时间排序增加 SQLite 表达式索引，减少 `ORDER BY COALESCE(sent_at, received_at, created_at)` 触发临时排序导致的列表打开延迟。

### 新增

- 新增发件箱查看能力：邮箱账号支持配置发件箱 IMAP 文件夹名，手动收取和全量同步会同步 `INBOX` 与发件箱，邮件页左侧新增“发件箱”系统文件夹。
- 后端新增邮箱账号 IMAP 文件夹列表接口 `GET /api/v1/mail-accounts/{id}/folders`，用于读取真实服务器目录并识别可能的发件箱目录。
- 前端邮箱账号编辑弹框新增“读取目录”能力，发件箱文件夹名改为下拉选择，避免不同邮箱服务商目录名不一致时靠手工猜。

### 修复

- 修复 Foxmail 等客户端发送的邮件中，正文 `cid:` 引用的图片附件未标记为 inline 时详情页显示破图的问题；详情接口会根据 HTML 引用动态识别内嵌图片并隐藏对应普通附件。
- 修复内嵌 TIFF 图片在浏览器中显示为破图或文件名的问题；详情接口会将 `image/tiff` 内嵌图片临时转换为 PNG data URL 返回，原始附件文件不变。
- 增强邮件详情 `cid:` 图片处理：兼容大写 `CID:`、尖括号和 URL 编码的 Content-ID；当原始邮件缺少对应内嵌附件时，使用“内嵌图片缺失”占位图替代浏览器破图。
- 修复配置的发件箱目录不存在时，自动/手动同步因 `SELECT Folder not exist` 失败并影响 `INBOX` 同步的问题；现在非 `INBOX` 目录不存在会跳过并返回提示。
- 优化邮件详情发件人、收件人和抄送人展示，将 `名字 <邮箱>` 拆成联系人标签，避免多人地址直接拼接导致换行和阅读体验别扭。

### 测试

- 新增后端测试覆盖 `inline=false` 但被 HTML `cid:` 引用的图片附件，确保详情响应会改写为可展示的 data URL。
- 新增后端测试覆盖内嵌 TIFF 图片转换为 PNG data URL，避免浏览器不支持 TIFF 导致正文破图。
- 新增后端测试覆盖 URL 编码 `cid:` 匹配和缺失内嵌附件的占位图兜底。
- 新增后端测试覆盖发件箱同步、账号发件箱配置回显，以及 `systemFolder=sent` 过滤。
- 新增后端测试覆盖 IMAP 文件夹列表接口和发件箱候选识别。
- 新增后端测试覆盖发件箱目录不存在时仍可同步 `INBOX`，并向前端返回 warning。

### 文档

- 更新 API、架构、需求和邮件界面设计文档，补充发件箱文件夹配置、同步范围和 `systemFolder=sent` 查询说明。
- 更新 API、架构和需求文档，补充读取 IMAP 文件夹列表、选择真实发件箱目录以及非 `INBOX` 目录不存在时的容错策略。
- 明确长期规则：Mail Nest 必须保存邮箱服务器上的收件邮件和已发送邮件，不能只保存收件箱；本地客户端未上传到服务器发件箱的邮件不属于 IMAP 可同步范围。

### 部署

- 将版本 `20260710111406-4e0e307-mailfast` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/`，线上健康检查、邮件列表 SQL 查询计划和浏览器首屏验证通过。
- 将后端版本 `20260710142458-4e0e307-inlinecid` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/`，线上健康检查和目标邮件详情内嵌图片显示验证通过。
- 将后端版本 `20260710153900-4e0e307-tiffpng` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/`，线上健康检查和目标 TIFF 内嵌图片转换显示验证通过。
- 将后端版本 `20260710155414-4e0e307-cidfallback` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/`，线上健康检查验证通过。
- 将版本 `20260710161834-4e0e307-sentbox` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/`，线上健康检查和浏览器左侧“发件箱”入口验证通过。
- 将版本 `20260710172735-4e0e307-folderlist` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/`，线上健康检查、用友邮箱 IMAP 目录读取、发件箱目录修正和全量同步验证通过。
- 线上将用友邮箱账号的发件箱目录从不存在的 `Sent` 修正为服务器真实目录 `Sent Items`，并关闭该账号“全量同步成功后清理服务器旧邮件”开关，避免排查期间误删远端邮件。
- 将前端版本 `20260711104936-4e0e307-addressui` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/`，线上健康检查和邮件详情联系人标签展示验证通过。

## 2026-07-08

### 修复

- 修复 GBK/GB2312 等中文邮件头未解码导致主题、发件人、收件人显示 `=?gbk?...?=` 乱码的问题，并兼容单段非 multipart 邮件正文的字符集解析。
- 后端启动时会尝试修复已入库的 RFC2047 乱码邮件元数据，基于已保存的原始邮件重新解析标题、联系人、正文和搜索索引。
- 修复部分非标准邮件缺少头部与正文空行时，正文被保存成 MIME boundary、`Content-Type` 和 base64 原文的问题；启动修复会重新扫描有原文的邮件并重写异常正文。
- 后端新增真正的邮箱自动收取调度器：服务启动后每分钟轻量扫描到期且启用的邮箱账号，按账号收取间隔触发日常增量收取，默认最多并发 2 个账号，并避免同一账号手动收取与自动收取重复执行。

### 测试

- 新增后端测试覆盖 GBK 编码邮件主题、联系人、正文和附件文件名解析。
- 新增后端测试覆盖缺少头部/正文分隔空行的嵌套 multipart 邮件正文解析。
- 新增后端测试覆盖自动收取到期账号筛选，确保停用账号、最近已同步账号和全量同步中的账号不会被定时任务选中。

### 部署

- 将版本 `20260708101038-4e0e307-actions` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/`，线上健康检查验证通过。
- 将版本 `20260708102431-4e0e307-maildecode` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/`，线上健康检查、后端日志和历史乱码邮件修复结果验证通过。
- 将版本 `20260708104116-4e0e307-bodyparse` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/`，线上健康检查、正文文件修复结果和浏览器邮件详情验证通过。
- 将版本 `20260708153226-4e0e307-autosync` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/`，线上健康检查、自动收取调度器启动日志和首轮自动收取任务记录验证通过。

## 2026-07-07

### 文档

- 新增升级与部署安全硬规则：以后任何升级、部署、迁移、重建容器、调整挂载目录或修改数据库结构，都必须以“不丢用户数据、可备份、可回滚、平滑升级”为最高优先级。
- 更新架构和 API 文档，补充登录后修改密码、个人资料、头像上传接口、校验规则和密码哈希存储说明。

### 修复

- 修复邮件页日期范围选择器显示英文月份和星期的问题，统一 Ant Design Vue 与 dayjs 使用中文 locale。
- 修复左侧“暂无文件夹”空状态显示破图片图标的问题，改为轻量文本空状态。
- 优化邮件页日期范围选择器中文占位符，并将左侧空文件夹状态改为稳定的线性图标加中文文案。
- 优化顶部用户区视觉清晰度，改为头像、昵称和下拉箭头组合展示，避免深色背景下用户名不清晰。

### 新增

- 后端新增登录后修改密码接口 `POST /api/v1/auth/change-password`，要求校验当前密码，新密码使用 `bcrypt` 哈希后保存。
- 前端右上角用户菜单新增“修改密码”入口，弹框支持输入当前密码、新密码和确认新密码，修改成功后保持当前登录态。
- 后端新增个人资料接口 `GET/PUT /api/v1/profile`，支持维护昵称和个人描述，并在当前用户响应中返回资料字段。
- 后端新增头像上传和读取接口 `POST /api/v1/profile/avatar`、`GET /api/v1/profile/avatar/content`，头像保存到本地用户数据目录且读取需要登录。
- 前端新增个人设置页 `/settings/profile`，支持查看用户名/邮箱、修改昵称/描述、上传头像，并同步刷新顶部用户区。
- 将版本 `20260708081342-4e0e307-profile` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/`，线上健康检查验证通过。
- 后端新增停止全量同步接口 `POST /api/v1/mail-accounts/{id}/full-sync/stop`，停止后状态标记为 `cancelled`，已同步到本地的邮件继续保留。
- 后端启动时会把遗留的 `running` 全量同步状态重置为失败，避免容器重启后页面长期显示假同步中。
- SQLite 启用 WAL 和 busy timeout，降低全量同步写入期间列表和状态读取被锁住的概率。
- 前端邮箱账号页调整全量同步轮询逻辑：同步中只轮询单账号状态，不再整表 loading，不再反复弹出超时提示。
- 前端邮箱账号页新增“停止同步”和“同步日志”入口，同步中仍可点击其他账号操作。
- 将版本 `20260708085022-4e0e307-syncfix` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/`，线上健康检查与未登录接口响应验证通过。
- 优化邮箱账号页操作列，将低频操作收纳到“更多”下拉菜单，保留“收取”和“同步全部/停止同步”为主按钮，避免按钮过多导致表格横向溢出。
- 后端新增邮箱账号全量历史同步能力：通过 `POST /api/v1/mail-accounts/{id}/full-sync/start` 后台分批同步 `INBOX` 全部 UID，并通过 `GET /api/v1/mail-accounts/{id}/sync-status` 查询进度。
- 邮箱账号新增全量同步状态字段，记录状态、总数、已处理数、新增数、开始/结束时间和错误信息，账号列表接口同步返回这些字段。
- 邮箱账号新增同步后清理服务器旧邮件策略，默认关闭；开启后仅在全量同步成功后删除已本地保存且早于保留天数的服务器 `INBOX` 邮件。
- 前端邮箱账号页新增“同步全部”操作、全量同步进度展示、服务器旧邮件清理开关和保留天数配置。
- 前端左侧主菜单新增锁起/展开按钮，锁起后侧栏收窄并仅展示菜单图标，状态会保存在本地浏览器。
- 新增文件夹弹框的颜色字段改为预设色块选择，不再要求手动输入颜色值。
- 邮件搜索框新增搜索范围下拉，支持按全部、发件人、主题、正文搜索，并移除下方重复的发件人和主题输入框。
- 优化邮件搜索栏布局，让搜索范围下拉、输入框和搜索图标保持在同一行组合显示。
- 重新设计邮件搜索区为两行紧凑筛选布局，统一搜索范围、输入框、搜索按钮、邮箱、日期和附件控件的高度与对齐。
- 优化邮件搜索筛选区窄宽度响应式布局，避免邮箱账号、日期范围和附件筛选在列表栏较窄时挤压错位。
- 调整左侧主菜单锁起按钮位置，将其从顶部品牌区移到底部工具区，避免和系统图标挤在一起。
- 修复邮件页内容较高时左侧主菜单底部“锁起菜单”按钮可能被挤出视口的问题，侧栏改为固定视口高度并让菜单区域内部滚动。
- 将版本 `20260707114614-4e0e307-ui` 部署到 生产环境 的 Mail Nest Docker Compose 服务，更新前已备份远端 `docker-compose.yml`、`config.yaml` 和 `data/`，线上首页与健康检查验证通过。

## 2026-07-06

### 文档

- 初始化 Mail Nest（信匣）项目文档。
- 明确后端目录为 `mailnest-be/`，前端目录为 `mailnest-fe/`。
- 明确后端建议使用 Go，数据默认存储到本地目录，数据库优先使用 SQLite。
- 明确前端建议使用 Vue 3、Vite、TypeScript、Ant Design Vue。
- 明确系统需要支持用户注册登录、多用户数据隔离、多个邮箱账号配置、邮件收取和 Web 查看。
- 明确所有对外 JSON 接口必须使用统一 envelope：`success`、`message`、`httpCode`、`data`。
- 新增 `AGENTS.md`、`doc/requirements.md`、`doc/architecture.md`、`doc/api.md`、`doc/implementation-plan.md`。
- 确认一期实现决策：登录态使用 JWT；邮件收取只支持 IMAP/IMAPS；一期只收取 `INBOX`；邮箱密码或授权码加密后存入 SQLite；是否开放注册由后端 `config.yaml` 控制。
- 新增后端说明文档 `mailnest-be/README.md` 和前端说明文档 `mailnest-fe/README.md`。
- 新增 `doc/mail-ui-and-rules-design.md`，设计三栏邮箱客户端界面、邮件搜索过滤、本地文件夹和规则归档能力。

### 新增

- 新增 `.gitignore`，排除本地配置、运行数据、前端依赖和构建产物。
- 创建 `mailnest-be/` Go 后端项目骨架。
- 后端实现 `config.yaml` 配置读取，缺失配置文件时使用默认值。
- 后端实现 SQLite 本地数据库初始化和用户、邮箱账号、邮件、附件、收取任务基础表结构。
- 后端实现统一 JSON envelope 响应封装。
- 后端实现用户注册、登录、当前用户、退出登录接口。
- 后端实现 JWT 生成和鉴权中间件。
- 后端实现注册开关，允许通过 `config.yaml` 控制是否开放注册。
- 后端实现邮箱账号创建、列表和删除接口，并按当前登录用户隔离数据。
- 后端实现邮箱密码或授权码 AES-GCM 加密后落库。
- 后端实现 IMAP 连接测试接口、手动收取接口、邮件列表接口和邮件详情接口。
- 后端实现 INBOX 邮件拉取、正文落本地文件、邮件元数据入 SQLite 和基于 `account_id + folder + imap_uid` 的去重。
- 后端实现 Microsoft OAuth2 授权入口、授权完成接口、token 加密存储、refresh token 刷新和 IMAP OAuth bearer 登录。
- 创建 `mailnest-fe/` Vue 3 + Vite + TypeScript + Ant Design Vue 前端项目骨架。
- 前端实现登录页、注册页、基础应用壳、Pinia 登录态管理和统一 envelope API 客户端。
- 前端实现邮箱账号列表、新增邮箱账号弹窗和删除操作。
- 前端实现邮箱连接测试、手动收取、邮件列表和邮件详情抽屉。
- 前端实现 Microsoft OAuth2 授权按钮和回调页面，用于兼容 Outlook 二步验证或禁用基础密码登录的账号。
- 后端扩展邮件列表搜索过滤，支持关键词、发件人、主题、日期范围、附件、邮箱账号、系统文件夹和本地文件夹过滤。
- 后端新增本地文件夹表和接口，支持创建、列表、删除文件夹，以及将邮件放入本地文件夹。
- 后端新增邮件规则和规则条件表，支持创建规则、收取新邮件后自动归档、手动对历史邮件应用规则。
- 前端邮件页改为三栏邮箱客户端布局，左侧文件夹导航，中间搜索过滤和邮件列表，右侧常驻邮件阅读区。
- 前端新增规则管理页面，支持创建规则并手动应用到历史邮件。
- 后端新增规则删除接口，删除规则时同步删除规则条件。
- 前端补充本地文件夹新增、删除入口，并在规则列表中增加删除规则操作。
- 后端新增邮箱账号更新接口 `PUT /api/v1/mail-accounts/{id}`，编辑时 `imapPassword` 留空会保留原加密凭据。
- 后端新增邮件规则更新接口 `PUT /api/v1/mail-rules/{id}`，更新规则时会整体替换条件，避免旧条件残留。
- 前端邮箱账号页面新增编辑入口，支持修改显示名称、邮箱地址、IMAP 主机、端口、用户名、TLS、收取间隔和启用状态。
- 前端规则页面新增编辑入口，支持修改规则名称、目标文件夹、匹配方式、启用状态和条件。
- 前端邮件三栏布局新增桌面端拖拽调整宽度能力，可分别调整左侧文件夹栏和中间邮件列表栏。
- 新增 Docker Compose 部署模板：后端生产 Dockerfile、前端 nginx 镜像、前端容器内 `/api` 反代、NAS compose 示例和生产配置示例。
- 完成 `mailnest.ligson.xyz` 的 NAS Docker Compose 部署验证：后端和前端容器运行在 `mynetwork`，外层 nginx 按域名反代到 Mail Nest。

### 修复

- 当 Microsoft OAuth 未配置 `clientId` 时，后端直接返回明确错误，避免跳转到 Microsoft 后才出现 `AADSTS900144`。
- 修复邮件详情中附件和内嵌图片显示不正确的问题：后端收取邮件时解析并保存普通附件、带 `Content-ID` 的内嵌资源，详情接口返回附件列表，并将 HTML 正文中的 `cid:` 图片引用转换为可直接展示的 `data:` URL。
- 新增受 JWT 保护的附件内容下载接口，前端详情抽屉支持展示普通附件列表并通过授权请求下载附件。
- 重复同步历史邮件时，如果数据库中已有邮件记录但缺少附件数据，会基于重新解析到的附件进行一次回填，便于修复旧版本已收取邮件的附件显示。
- 修复部分旧邮件详情响应缺少 `attachments` 字段时，前端附件计算属性直接调用 `filter` 导致详情抽屉报错的问题。
- 补齐邮箱账号删除确认、文件夹新增弹框等操作按钮的中文文案，避免 Ant Design Vue 默认英文按钮出现在业务界面。
- 补充 `.gitignore`，忽略 `.superpowers/` 本地过程产物，避免提交临时预览和运行状态文件。
- 补充 `.gitignore`，忽略 Docker 部署真实配置、数据目录和本地镜像包，避免 NAS 部署敏感信息进入仓库。
- 新增后端和前端 `.dockerignore`，避免本地依赖、构建产物、配置和运行数据进入 Docker 构建上下文。

### 文档

- 更新架构和 API 文档，补充日常增量收取、全量历史同步、同步进度接口、同步后服务器旧邮件清理策略，以及升级必须保护用户数据的约束。

### 测试

- 新增后端测试覆盖邮箱账号编辑时保留旧密码、规则编辑时替换条件并按新目标文件夹归档。
- 新增后端测试覆盖全量同步不受最近 50 封限制，以及同步后清理只删除已同步入库且超过保留天数的服务器邮件 UID。
- 新增后端测试覆盖修改密码需要登录、当前密码错误拦截、新旧密码相同拦截，以及修改成功后旧密码不可登录、新密码可登录。
- 新增后端测试覆盖个人资料修改、当前用户资料回读、头像上传和受保护头像内容读取。
- 新增后端测试覆盖全量同步停止接口和 `cancelled` 状态。
