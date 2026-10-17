# 版本管理与系统版本展示

## 版本约定

自本次版本管理功能开始，前后端默认版本均为 `0.2.0`。版本采用 `主版本.次版本.修订版本`：不兼容变更升级主版本，兼容功能升级次版本，兼容修复升级修订版本。前后端允许独立发布，不要求它们的版本号永远相同。

- 后端源码默认版本：`mailnest-be/internal/buildinfo/version.txt`，通过 Go embed 固化进程序。
- 前端源码默认版本：`mailnest-fe/package.json` 的 `version`，同步更新 `package-lock.json` 根包版本。
- 构建时可通过 `APP_VERSION` 覆盖发布版本，不能在启动时通过生产配置随意更改显示值。
- `VCS_REF` 保存构建源码的提交号，`BUILD_TIME` 使用 UTC RFC3339，例如 `2026-10-17T08:00:00Z`。未注入的提交号/后端构建时间显示“未提供”，不伪造为启动时间。
- 有未提交改动的构建建议使用 `提交号-dirty`，避免把工作区构建误认为精确对应某个提交。镜像标签用于部署定位，不自动作为语义版本。

## 展示及接口

管理员进入“系统管理”，可在用户统计前看到“系统版本”：前端/后端版本号、提交号及构建时间。前端信息来自浏览器当前加载的静态构建产物；后端信息来自当前运行二进制。只更新一端时不会错误显示另一端也已升级。浏览器缓存旧资源时仍会显示旧前端版本，刷新页面才会加载新构建。

`GET /api/v1/admin/version` 仅管理员可用：未登录返回 401，非管理员返回 403，均使用统一 envelope。接口不读取邮件或聚合存储统计，不增加大表查询。成功示例：

```json
{
  "success": true,
  "message": "获取成功",
  "httpCode": 200,
  "data": {
    "backend": {
      "version": "0.2.0",
      "commit": "unknown",
      "buildTime": "unknown"
    }
  }
}
```

版本请求独立于用户列表，失败时持续显示说明和重试按钮，不阻塞用户列表，也不把前端版本冒充后端版本。旧后端没有版本接口时显示读取失败，不猜测版本。接口只包含公开构建信息，不暴露配置、密码、路径或数据库连接。

## 构建方式

在仓库根目录执行；示例提交号/时间必须替换为本次构建的真实值。必须使用本机 Docker 构建，不能改为 NAS 构建。

```sh
docker build -f mailnest-be/docker/Dockerfile \
  --build-arg APP_VERSION=0.2.0 \
  --build-arg VCS_REF=真实提交号 \
  --build-arg BUILD_TIME=2026-10-17T08:00:00Z \
  -t ligson/mailnest-be:0.2.0 mailnest-be

docker build -f mailnest-fe/Dockerfile \
  --build-arg APP_VERSION=0.2.0 \
  --build-arg VCS_REF=真实提交号 \
  --build-arg BUILD_TIME=2026-10-17T08:00:00Z \
  -t ligson/mailnest-fe:0.2.0 mailnest-fe
```

Docker 未提供 `BUILD_TIME` 时使用实际构建时间。直接构建后端时可通过 `-ldflags` 注入；无需配置文件或数据库即可查看版本：

```sh
cd mailnest-be
go build -ldflags "-X mailnest-be/internal/buildinfo.Version=0.2.0 -X mailnest-be/internal/buildinfo.Commit=真实提交号 -X mailnest-be/internal/buildinfo.BuildTime=2026-10-17T08:00:00Z" -o mailnest ./cmd/mailnest
./mailnest -version
```

前端本地构建同样支持 `APP_VERSION`、`VCS_REF`、`BUILD_TIME` 环境变量，未覆盖版本时使用 package.json；仅这些明确允许的构建字段注入静态产物，不能把整个环境或生产配置注入前端。

## 发版检查

1. 根据变更更新对应端的默认版本和 CHANGELOG，不修改无关依赖。
2. 执行后端测试、前端构建及版本接口权限/展示回归；确认默认值及构建覆盖均生效。
3. 本机镜像构建注入真实提交号和时间，遵循现有持久化、分级备份及平滑升级规则。
4. 部署后同时核对系统管理显示的前后端信息与目标构建，而不是只看镜像标签；普通版本信息功能不修改数据库结构或邮件文件。

## 本次验证

- 后端全量测试及版本相关竞态测试通过，覆盖未登录 401、普通用户 403、管理员 200 和 envelope 一致性。
- 后端无配置 `-version` 验证默认版本与构建覆盖均生效；前端使用测试版本、提交号和时间构建，检查产物固化信息后重新构建恢复默认版本。
- 浏览器模拟后端不同版本，确认两端独立展示；503 失败提示持续保留、重试恢复，390px 窄屏版本区域无内部横向溢出，菜单收起后正常展示。
- 前端类型检查与生产构建通过，仍有既有共享 UI 大 chunk 提示；上述模拟结果属于本地验收，线上发布结果单独记录如下。

## 0.2.0 生产发布（2026-10-17）

- 本机 Docker 构建并更新 NAS 前后端，部署标签为 0.2.0-20261017，源码标记 dd8580f81235-dirty；本次发布工作区版本，没有自动提交或推送 Git。
- 浏览器系统管理与管理员版本接口分别核对前后端 0.2.0；构建时间为北京时间 2026-10-17 13:53:45（前端）、13:53:44（后端）。后端 CLI 版本一致，未登录版本请求返回 envelope 401。
- 使用 L1 轻量备份与镜像切换，不修改配置、数据库结构或数据挂载；核心记录原有 ID 保留，邮件 14,363 封、附件 23,597 个，主要接口及附件 206 通过。备份与回滚信息仅留在受保护的部署目录，不提交仓库。
