# 容器地址变化导致的 Nginx 502

## 原因与定位

容器重启或重建后，Docker 网络地址可能变化。固定主机名形式的 `proxy_pass` 如果只在 Nginx 加载配置时解析，可能继续连接旧地址。表现为页面返回 502，错误日志出现 `connect() failed (111: Connection refused) while connecting to upstream`，而直接通过当前容器主机名访问正常。

检查容器状态、启动时间、当前地址及 Nginx 错误日志中的 upstream 地址，确认二者是否一致。页面和 API 应分别检查，避免只恢复静态页面。日志可能包含邮件或用户信息，只读取排障所需范围。

## 恢复与预防

先运行 `nginx -t`，检查通过后执行 `nginx -s reload`，让代理重新解析地址并平滑恢复。

对于与上游处于同一 Docker 自定义网络的 Nginx，可在对应 server 中配置：

```nginx
resolver 127.0.0.11 valid=10s ipv6=off;
resolver_timeout 5s;
set $mailnest_backend http://mailnest-be:8080;
set $mailnest_frontend http://mailnest-fe:80;
```

API location 使用 `proxy_pass $mailnest_backend;`，页面 location 使用 `proxy_pass $mailnest_frontend;`。变量形式在运行时解析容器地址；不追加 URI 后缀，以保留原始路径和查询参数。保留原有转发头、上传大小与超时设置。此 DNS 地址仅适用于 Docker 容器网络。

## 验证与回滚

修改前按 L1 方案保存站点代理配置、应用配置、Compose 和当前镜像标签，真实配置仅保存在受保护的服务器备份目录，不提交仓库。语法检查失败时恢复站点配置，再检查并平滑重载。

验证个人设置页、邮件页、前端资源和健康接口正常；受保护接口未登录时应返回统一 JSON envelope 和 401，而非 502。动态解析缓存到期后再次检查。正常恢复无需重建应用容器或修改数据库。验证通过后，同类自动备份保留最近两次。
