# 邮件搜索性能与全文索引

> 当前推荐方案已调整为 [Bleve 本地检索设计与切换方案](bleve-search-design.md)。本文 MySQL ngram 保留为兼容选项，未在生产启用；采用 Bleve 无需执行本文 FULLTEXT DDL。浏览器请求治理和改造前基线仍适用。

## 问题与实测

2026-10-17 对现有 MySQL 邮件库做只读执行计划诊断：约 14,350 封邮件，`search_text` 合计约 91 MiB。搜索 `601` 时，收件箱结果统计返回 566 条，`EXPLAIN ANALYZE` 显示单次 COUNT 耗时约 16.65 秒，超过浏览器请求的 15 秒超时。该数据是改造前基线，不是改造后性能承诺。

原实现对主题、发件人、收件人、抄送、正文执行多个 `LIKE '%关键词%'`；已有用户、账号、日期索引无法有效缩小正文匹配范围。分页列表还需要再执行同样的过滤。同时输入框每次输入立即查询，旧请求没有取消，易造成数据库重复扫描。

## 本次实现

### 浏览器请求治理

- 输入停顿 350 毫秒后搜索，回车或点击搜索立即执行，并回到第一页。
- 输入新关键词、改变筛选、切换列表或离开页面时取消旧列表请求。
- 邮件、会话、草稿列表均传递 AbortSignal，只允许当前请求更新列表、总数和 loading，取消请求不弹失败提示。
- HTTP 请求 Context 传入邮件/会话的数据库查询（包括会话摘要），取消时终止旧查询。PostgreSQL 的 Context 查询也经过占位符方言转换。
- 没有通过延长 15 秒请求超时掩盖数据库瓶颈。

### MySQL ngram 候选检索

对 `subject, from_addr, to_addrs, cc_addrs, search_text` 建立一个多字段 ngram FULLTEXT 索引。系统自动维护新增、更新和删除邮件对应的索引，不需要另启搜索服务，也不额外复制邮件原文或附件。历史邮件在首次建立索引时纳入。

列表和会话共用条件生成器：

1. 从关键词中提取安全的连续双字符片段，例如 `601` 提取 `60`、`01`，`张海涛` 提取 `张海`、`海涛`。
2. 排除默认停用词会影响的片段；只使用可确定分词行为的 ASCII 字母、数字和基本汉字，最多 8 个不同片段。
3. 用参数化布尔 MATCH 检索候选邮件，不把用户输入直接拼接为布尔语法。
4. 候选结果仍执行原始字段 LIKE 复核，保持子串、字段范围、日期、附件、已读、垃圾箱和回收站等现有语义。
5. 外层始终保留当前用户的 `user_id` 约束和账号/文件夹过滤，不能跨用户返回数据或统计。

单字、纯停用词、带反斜线或无法提取安全片段的查询回退到原有 LIKE，保持能搜但不保证同等加速。SQLite、PostgreSQL 及不兼容的 MySQL/MariaDB 配置继续使用兼容搜索，不执行 MySQL 专属 DDL。后续可分别增加 SQLite FTS5、PostgreSQL pg_trgm，不能直接把 MySQL 语法套到所有数据库。

## 索引准备与部署

普通服务启动**只检查索引能力，不自动构建大表全文索引**。MySQL 需要 `ngram_token_size=2`、未配置全局自定义停用词表，且五个搜索列使用不区分大小写的排序规则。应用检查索引名称、列顺序、FULLTEXT 类型、ngram 解析器和版本标记。检查失败会记录中文日志并回退，不阻止系统启动。

生产上线前：

1. 本机 Docker 构建后端和前端镜像，不在 NAS 上构建。
2. 按 `deployment-backup-strategy.md` 做 **L2 备份**：配置、Compose、旧镜像标签、MySQL 一致性逻辑备份和回滚说明。没有文件格式或路径变更，不打包几十 GB 邮件数据目录。
3. 确认磁盘空间、备份可解压、核心表计数和持久化挂载。选择维护窗口，暂停导入、同步等写入；首次 FULLTEXT 可能重建表并阻塞写入，不能宣称完全无锁。
4. 使用新后端镜像和原有持久化配置运行一次准备命令：

```sh
# Compose 的镜像标签先指向新版本；命令不启动后台同步任务
docker compose run --rm --no-deps mailnest-be -prepare-search-index
```

本地源码运行示例：

```sh
cd mailnest-be
MAILNEST_CONFIG=/path/to/config.yaml go run ./cmd/mailnest -prepare-search-index
```

实际新增 DDL 为：

```sql
ALTER TABLE mail_messages
  ADD FULLTEXT INDEX ft_mail_messages_ngram_v1
  (subject, from_addr, to_addrs, cc_addrs, search_text)
  WITH PARSER ngram COMMENT 'mailnest-ngram-2-default-stopwords-v1';
```

准备命令在专用连接使用默认停用词、限制元数据锁等待 15 秒，整体最长 30 分钟；不调整共享 MySQL 全局配置。同名索引不符合预期时直接报错，不覆盖。重复执行成功的准备命令无须重建索引。启动迁移仅在 `search_text` 不是 LONGTEXT 时改类型，避免每次启动无意义 ALTER。

5. 启动/替换前后端，检查日志出现“邮件搜索已启用 MySQL ngram”。使用同一用户、相同参数比较新旧查询的总数和邮件 ID，测量冷/热查询、列表和会话接口耗时；检查健康、登录、账号、规则、邮件详情与附件。
6. 确认数据未减少，记录镜像版本和验证结果，通过后按级别只保留最近两次自动备份。

普通应用启动期间也会执行已有的兼容迁移，准备命令应使用已验证过的新版本，不能跳过备份。

## 回滚

优先恢复旧前后端镜像及 Compose。新增全文索引不改变邮件内容，旧版本仍可运行，无须先删除索引或覆盖数据库。若要移除索引，在维护窗口停止使用新检索路径的服务后执行：

```sql
ALTER TABLE mail_messages DROP INDEX ft_mail_messages_ngram_v1;
```

不要在运行中的新服务下直接删索引，能力只在启动时检测。默认不恢复整个数据库，避免覆盖部署后新收取的邮件；只有确有数据损坏才按独立恢复方案处理。

## 测试与诊断

```sh
cd mailnest-be
go test ./...
# 仅连接一次性测试库；测试会创建用户、账号和邮件，不能指向生产
MAILNEST_TEST_MYSQL_DSN='root@tcp(127.0.0.1:13316)/mailnest_search_test?parseTime=true&charset=utf8mb4' \
  go test ./internal/storage -run TestMailSearch -v -count=1
cd ../mailnest-fe
npm run build
```

测试比较全文候选路径和原 LIKE 路径的邮件/会话结果，覆盖中文、大小写、数字、邮箱、短词、通配符、转义字符、多个字段组合、用户隔离、历史数据、新增/更新、重复建索引及重启检测；SQLite 验证兼容路径和取消查询。

浏览器侧使用本机开发页面和模拟接口测试：快速输入 `601` 只发出一次最终搜索；人为延迟 `60` 的响应后输入 `601`，旧请求被取消，旧总数不会覆盖新总数；回车立即搜索且不会在防抖到期后再发重复请求。这些验证不使用真实账号或生产邮件。

本机 MySQL 执行计划已确认候选查询的访问类型为 `fulltext`，使用 `ft_mail_messages_ngram_v1`。另用 `SLEEP(2)` 验证 Context 超时能取消执行中的 MySQL 查询。生产索引尚未创建，线上优化后的延迟需在获准上线后实测。

慢查询日志包含 `search_mode=mysql-ngram` 或 `search_mode=like`、耗时、结果数，不记录搜索词或邮件内容。索引未部署前线上仍运行原路径，不能把本地测试通过写成线上已生效。

技术依据：MySQL 官方《ngram Full-Text Parser》：https://dev.mysql.com/doc/refman/8.4/en/fulltext-search-ngram.html 。尤其注意 ngram 的停用词是“片段包含停用词即排除”，不是普通英文全文检索的整词规则。
