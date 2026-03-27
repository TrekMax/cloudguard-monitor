# CloudGuard Monitor

轻量级云服务器监控套件 — 一键部署，手机/PC 随时查看服务器状态，异常实时告警。

## 特性

- **低资源占用** — Agent 常驻 CPU < 2%，内存 < 50MB
- **零依赖部署** — 单二进制文件 + 内嵌 SQLite，无需安装数据库
- **多端查看** — CLI 终端 TUI 看板 / Android App（开发中）
- **完整指标** — CPU、内存、磁盘、网络、进程 Top N、系统信息
- **历史回溯** — 数据自动降采样，默认保留 30 天
- **告警通知** — 阈值告警 + WebSocket 实时推送（开发中）
- **安全通信** — Token 认证，TLS 加密（开发中）

## 快速开始

### 1. 编译

```bash
# 服务端
cd server && make build
# CLI
cd cli && make build
```

### 2. 启动服务端

```bash
cd server
./bin/cloudguard
```

默认监听 `0.0.0.0:8080`，可通过配置文件自定义：

```bash
./bin/cloudguard --config /path/to/cloudguard.yaml
```

### 3. 使用 CLI

```bash
# 连接服务器（配置保存到 ~/.cloudguard/config.yaml）
cloudguard-cli connect http://your-server:8080 --token YOUR_TOKEN

# 查看实时状态
cloudguard-cli status

# 查看系统信息
cloudguard-cli system

# 查询历史指标
cloudguard-cli metrics --category cpu --range 24h

# 启动实时 TUI 看板
cloudguard-cli dashboard
```

## CLI 命令

| 命令 | 说明 | 示例 |
|------|------|------|
| `connect` | 配置并测试服务器连接 | `cloudguard-cli connect http://1.2.3.4:8080 --token xxx` |
| `status` | 查看实时状态 | `cloudguard-cli status --format json` |
| `system` | 查看系统信息 | `cloudguard-cli system` |
| `metrics` | 查询历史指标 | `cloudguard-cli metrics -c cpu -r 7d -l 100` |
| `dashboard` | 实时 TUI 看板 | `cloudguard-cli dashboard` |

所有命令支持 `--format table|json|yaml` 切换输出格式。

## API 接口

| 端点 | 方法 | 说明 | 认证 |
|------|------|------|------|
| `/health` | GET | 健康检查 | 无 |
| `/api/v1/status` | GET | 实时状态（CPU/内存/磁盘/网络/进程） | Token |
| `/api/v1/metrics` | GET | 历史指标查询 | Token |
| `/api/v1/system` | GET | 系统信息 | Token |
| `/api/v1/processes` | GET | 进程列表 | Token |
| `/api/v1/alerts` | GET | 告警事件列表（支持 status/limit/offset） | Token |
| `/api/v1/alerts/:id/ack` | POST | 确认告警 | Token |
| `/api/v1/alerts/rules` | GET/POST | 告警规则列表/创建 | Token |
| `/api/v1/alerts/rules/:id` | PUT/DELETE | 更新/删除告警规则 | Token |
| `/ws/v1/realtime` | WebSocket | 实时指标 + 告警推送 | Token (query) |

### 认证

通过 `Authorization: Bearer <token>` 请求头传递 Token。在 `cloudguard.yaml` 中配置：

```yaml
auth:
  token: "your-secret-token"
```

Token 为空时认证关闭。

### 历史指标查询参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `category` | string | 过滤类别：cpu, memory, disk, network |
| `name` | string | 过滤指标名 |
| `start` | int64 | 起始 Unix 时间戳 |
| `end` | int64 | 结束 Unix 时间戳 |
| `limit` | int | 返回记录数（默认 1000） |

### 响应格式

```json
{
  "code": 200,
  "message": "success",
  "data": { ... },
  "timestamp": 1711543200
}
```

## 配置文件

服务端配置 `cloudguard.yaml`：

```yaml
server:
  host: "0.0.0.0"
  port: 8080

collector:
  cpu_interval: 5       # 秒
  memory_interval: 5
  disk_interval: 30
  network_interval: 5

database:
  path: "./data/cloudguard.db"
  retention_days: 30

log:
  level: "info"         # debug, info, warn, error
  format: "text"        # text, json

auth:
  token: ""             # 为空时自动生成

tls:
  enabled: false        # 启用 HTTPS
  auto_cert: true       # 自动生成自签名证书
  cert_file: ""         # 自定义证书路径
  key_file: ""          # 自定义私钥路径

security:
  ip_whitelist: []      # 为空则不限制，如 ["127.0.0.1", "10.0.0.0/8"]
```

## 部署

### 一键安装

```bash
sudo bash scripts/install.sh
```

安装脚本会自动：创建系统用户、下载二进制、生成配置和 Token、配置 systemd 服务。

### 手动部署

```bash
cd server && make build
cp bin/cloudguard /usr/local/bin/
cp configs/cloudguard.yaml /etc/cloudguard/
./bin/cloudguard --config /etc/cloudguard/cloudguard.yaml
```

## 项目结构

```
cloudguard-monitor/
├── server/                 # Go 服务端（Agent + API）
│   ├── cmd/cloudguard/     #   程序入口
│   ├── internal/
│   │   ├── api/            #   REST API (Gin)
│   │   ├── collector/      #   指标采集器 (CPU/内存/磁盘/网络/进程)
│   │   ├── store/          #   SQLite 存储层
│   │   ├── alert/          #   告警引擎
│   │   ├── ws/             #   WebSocket 推送
│   │   ├── security/       #   TLS、审计、IP 白名单
│   │   ├── config/         #   配置管理
│   │   └── logging/        #   结构化日志
│   └── configs/            #   默认配置文件
├── cli/                    # Go CLI 客户端
│   ├── cmd/cloudguard-cli/ #   程序入口
│   └── internal/
│       ├── cmd/            #   cobra 命令
│       ├── client/         #   API 客户端
│       ├── tui/            #   bubbletea TUI Dashboard
│       └── output/         #   多格式输出
├── scripts/                # 部署脚本
│   └── install.sh          #   一键安装
├── .github/workflows/      # CI/CD
└── docs/                   # 项目文档
```

## 开发

```bash
# 服务端
cd server
make build     # 编译
make test      # 测试（含 race detector）
make lint      # 代码检查
make coverage  # 覆盖率报告

# CLI
cd cli
make build
make test
make lint
```

### 技术栈

| 组件 | 技术 |
|------|------|
| 服务端 | Go, Gin, SQLite (WAL), slog |
| 告警引擎 | 阈值判断 + 持续时间 + 抑制 + 自动恢复 |
| WebSocket | gorilla/websocket 实时推送 |
| CLI | Go, cobra, bubbletea, lipgloss |
| 安全 | TLS (自签名/自定义证书)、Token 认证、IP 白名单、审计日志 |
| 数据采集 | /proc 文件系统直接读取 |
| 配置 | YAML |

## 采集指标

| 类别 | 指标 | 数据源 |
|------|------|--------|
| CPU | 使用率、用户/系统/IO 占比、负载均值、核心数 | `/proc/stat`, `/proc/loadavg` |
| 内存 | 总量/已用/可用/缓存、Swap 使用率 | `/proc/meminfo` |
| 磁盘 | 各分区使用率、IO 读写速率/IOPS | `statfs`, `/proc/diskstats` |
| 网络 | 收发速率、总流量、TCP 连接数 | `/proc/net/dev`, `/proc/net/tcp` |
| 进程 | Top 10 内存占用进程、总进程数 | `/proc/[pid]/stat` |
| 系统 | 主机名、OS、内核、架构、启动时间 | `/proc/version`, `/etc/os-release` |

## 开发路线

- [x] M1: 基础架构 — 项目骨架、配置、日志、CPU/内存采集
- [x] M2: 核心服务端 — 完整采集器、SQLite 存储、REST API
- [x] M3: CLI 客户端 — 命令框架、TUI Dashboard、多格式输出
- [ ] M4: Android App
- [x] M5: 告警与通知 — 告警引擎、WebSocket 推送、CLI 告警管理
- [x] M6: 安全与发布 — TLS、IP 白名单、审计日志、安装脚本、CI/CD

## License

MIT
