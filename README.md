# UPnP 端口映射管理面板

轻量级、可视化、自定义端口与租约的 UPnP 端口映射管理工具。基于 Go + Vue 3 单文件前端，单二进制 ~6 MB，支持多架构 Docker 镜像。

## ✨ 功能特性

- 🔍 **自动发现网关** — SSDP 发现局域网内支持 UPnP IGD 的路由器
- 📋 **可视化映射列表** — 实时展示现有端口映射，一键启用/禁用/删除
- ➕ **自定义新增映射** — 内网 IP/端口、外网端口、协议、租约时长、备注全可配
- ⏰ **租约自动续约** — 后台协程在租约过期前自动续约，0 = 永久
- 💾 **持久化存储** — SQLite 存储配置，容器重启自动恢复映射并继续续约
- 🐳 **多架构镜像** — 自动构建 `linux/amd64` `linux/arm64` `linux/arm/v7`
- 🔐 **安全建议** — 建议配合反向代理 + Basic Auth / OAuth2-Proxy 使用

## 🚀 快速开始

### Docker Compose（推荐）

```yaml
# docker-compose.yml
services:
  upnp-ui:
    image: ghcr.io/matrixmango/hwassassin/upnp-ui:latest
    container_name: upnp-ui
    restart: unless-stopped
    network_mode: host          # 必须 host 网络才能发现网关 SSDP
    volumes:
      - ./data:/data            # 持久化 SQLite
    environment:
      - TZ=Asia/Shanghai
```

```bash
docker compose up -d
# 打开 http://<宿主机IP>:8080
```

### 手动构建

```bash
git clone https://github.com/MatrixMango/HWAssassin.git
cd HWAssassin/upnp-ui
docker compose up -d --build
```

## 🖥️ 界面预览

| 映射列表 | 新增映射 |
|---------|---------|
| 实时显示所有映射，支持启用/禁用/删除 | 内网IP、内网端口、外网端口、协议、租约秒数、描述全可配 |

> 界面基于 Vue 3 + Tailwind CDN，单 HTML 文件，无需构建步骤。

## ⚙️ 配置说明

| 环境变量 | 默认值 | 说明 |
|----------|--------|------|
| `TZ` | `Asia/Shanghai` | 时区 |
| `UPNP_GATEWAY_URL` | *(自动发现)* | 指定网关 description XML URL，多网关环境下强制指定 |

## 🔧 进阶用法

### 反向代理 + Basic Auth（推荐生产环境）

```nginx
# nginx.conf 片段
location / {
    auth_basic "UPnP UI";
    auth_basic_user_file /etc/nginx/.htpasswd;
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
}
```

### 指定网关（多网关环境）

```yaml
environment:
  - UPNP_GATEWAY_URL=http://192.168.1.1:5000/rootDesc.xml
```

### 手动续约/同步

```bash
# 容器内执行（或 docker exec）
curl -X POST http://localhost:8080/api/mappings/1/toggle  # 切换启用状态
```

## 📦 镜像标签

| 标签 | 说明 |
|------|------|
| `latest` | main 分支最新构建 |
| `vX.Y.Z` | 语义化版本标签 |
| `sha-<commit>` | 特定 commit 构建 |

支持架构：`linux/amd64` `linux/arm64` `linux/arm/v7`

## 🛠️ 本地开发

```bash
# 依赖：Go 1.22+
go mod tidy
go run .                    # 直接运行，监听 :8080

# 前端修改 web/index.html 后直接刷新浏览器生效（无需重新编译）
```

## 📄 许可证

MIT License © 2024 MatrixMango

---

**Star ⭐** 如果觉得好用，欢迎点个 Star！