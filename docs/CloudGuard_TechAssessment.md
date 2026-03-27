# CloudGuard Monitor — 技术评估报告

> **版本:** v1.0 | **日期:** 2026-03-27

---

## 目录

- [1. 技术架构概览](#1-技术架构概览)
- [2. 服务端详细设计](#2-服务端详细设计)
- [3. 客户端技术方案](#3-客户端技术方案)
- [4. 通信协议设计](#4-通信协议设计)
- [5. 安全方案](#5-安全方案)
- [6. 风险评估](#6-风险评估)
- [7. 部署方案](#7-部署方案)

---

## 1. 技术架构概览

### 1.1 系统架构

系统采用典型的 Agent-Server 架构，服务端同时承担数据采集（Agent）和 API 服务的角色。由于当前场景为单台服务器，Agent 和 API Server 合并为同一进程，简化部署。

**架构分层：**

- **数据采集层：** 通过 /proc、/sys 文件系统采集系统指标
- **数据处理层：** 指标计算、聚合、告警判断
- **数据存储层：** SQLite 嵌入式数据库
- **API 服务层：** REST API + WebSocket 实时推送

```
┌─────────────────────────────────────────────────────────┐
│                    Cloud Server                         │
│  ┌───────────────────────────────────────────────────┐  │
│  │            CloudGuard Server (Go)                 │  │
│  │                                                   │  │
│  │  ┌─────────────┐  ┌──────────┐  ┌─────────────┐  │  │
│  │  │  Collector   │  │  Alert   │  │  API Server │  │  │
│  │  │  (采集层)    │→ │  Engine  │  │  (Gin)      │  │  │
│  │  └──────┬──────┘  └────┬─────┘  └──────┬──────┘  │  │
│  │         │              │               │          │  │
│  │         ▼              ▼               ▼          │  │
│  │  ┌─────────────────────────────────────────────┐  │  │
│  │  │           SQLite (WAL Mode)                 │  │  │
│  │  └─────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────┘  │
│                          │                              │
│                  REST API + WebSocket                   │
└─────────────────────────┬───────────────────────────────┘
                          │
            ┌─────────────┼─────────────┐
            │             │             │
      ┌─────▼─────┐ ┌────▼────┐ ┌──────▼──────┐
      │ Android   │ │ PC CLI  │ │  Browser    │
      │ App       │ │ (Go)    │ │  (Future)   │
      └───────────┘ └─────────┘ └─────────────┘
```

### 1.2 技术栈汇总

| 组件 | 技术选型 | 版本/规格 | 选型理由 |
|-----|---------|----------|---------|
| 服务端 | Go | ≥1.22 | 高性能、低资源占用、单二进制部署 |
| HTTP 框架 | Gin | v1.10+ | 轻量高性能，生态成熟 |
| WebSocket | gorilla/websocket | v1.5+ | Go 生态最成熟的 WS 库 |
| 数据库 | SQLite + go-sqlite3 | 3.x | 嵌入式、零配置、适合单机 |
| CLI 框架 | cobra + bubbletea | 最新稳定版 | cobra 做命令解析，bubbletea 做 TUI |
| Android | Kotlin | 1.9+ | 原生语言，性能最优 |
| Android UI | Jetpack Compose | 最新稳定版 | 声明式 UI，开发效率高 |
| Android 网络 | Retrofit + OkHttp | 最新稳定版 | 成熟稳定，支持 WebSocket |
| Android 图表 | MPAndroidChart / Vico | 最新稳定版 | 灵活的图表库 |
| 配置格式 | YAML | - | 可读性强，便于手动编辑 |

---

## 2. 服务端详细设计

### 2.1 项目结构

推荐采用 Go 标准工程布局：

```
cloudguard-server/
├── cmd/
│   └── cloudguard/
│       └── main.go              # 程序入口
├── internal/
│   ├── collector/               # 指标采集器
│   │   ├── cpu.go
│   │   ├── memory.go
│   │   ├── disk.go
│   │   ├── network.go
│   │   ├── process.go
│   │   ├── system.go
│   │   └── collector.go         # 采集调度器
│   ├── store/                   # 数据存储层
│   │   ├── sqlite.go
│   │   ├── metrics.go
│   │   └── downsample.go        # 降采样任务
│   ├── api/                     # REST API 处理器
│   │   ├── router.go
│   │   ├── middleware.go         # 认证/日志中间件
│   │   ├── status.go
│   │   ├── metrics.go
│   │   ├── alerts.go
│   │   └── config.go
│   ├── ws/                      # WebSocket 服务
│   │   ├── hub.go               # 连接管理
│   │   └── client.go
│   ├── alert/                   # 告警引擎
│   │   ├── engine.go
│   │   ├── rules.go
│   │   └── suppress.go          # 告警抑制
│   └── config/                  # 配置管理
│       └── config.go
├── configs/
│   └── cloudguard.yaml          # 默认配置文件
├── scripts/
│   └── install.sh               # 一键安装脚本
├── go.mod
├── go.sum
├── Makefile
├── CLAUDE.md                    # Claude Code 项目说明
└── README.md
```

### 2.2 指标采集方案

采用读取 Linux /proc 和 /sys 文件系统的方式采集指标，避免调用外部命令，确保低开销。

| 指标 | 数据源 | 计算方式 |
|-----|-------|---------|
| CPU 使用率 | `/proc/stat` | 两次采样差值计算 |
| 内存使用 | `/proc/meminfo` | 直接读取各字段 |
| 磁盘使用率 | `syscall (statfs)` | 挂载点遍历 |
| 磁盘 IO | `/proc/diskstats` | 两次采样差值计算 |
| 网络流量 | `/proc/net/dev` | 两次采样差值计算 |
| 进程信息 | `/proc/[pid]/stat` | 遍历进程目录 |
| 系统信息 | `/proc/version`、`/etc/os-release` | 启动时读取一次 |

**采集器接口设计：**

```go
type Collector interface {
    Name() string
    Collect(ctx context.Context) (*Metrics, error)
    Interval() time.Duration
}
```

### 2.3 数据存储设计

SQLite 表设计要点：

- 指标表采用时间分区，按天分表以提高查询性能
- 启用 WAL 模式支持读写并发
- 定时运行 VACUUM 保持数据库紧凑
- 历史数据的降采样通过后台 goroutine 定期执行

**核心表结构：**

```sql
-- 实时指标表（按天分表：metrics_20260327）
CREATE TABLE metrics_YYYYMMDD (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp   INTEGER NOT NULL,       -- Unix timestamp
    category    TEXT NOT NULL,           -- cpu/memory/disk/network
    name        TEXT NOT NULL,           -- 具体指标名
    value       REAL NOT NULL,
    labels      TEXT,                    -- JSON 标签（如磁盘分区名）
    INDEX idx_ts_cat (timestamp, category)
);

-- 告警规则表
CREATE TABLE alert_rules (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL,
    category    TEXT NOT NULL,
    metric      TEXT NOT NULL,
    operator    TEXT NOT NULL,           -- gt/lt/eq
    threshold   REAL NOT NULL,
    duration    INTEGER DEFAULT 0,       -- 持续时间(秒)
    enabled     INTEGER DEFAULT 1,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

-- 告警事件表
CREATE TABLE alert_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id     INTEGER NOT NULL,
    status      TEXT NOT NULL,           -- firing/resolved/acknowledged
    value       REAL NOT NULL,
    message     TEXT,
    fired_at    INTEGER NOT NULL,
    resolved_at INTEGER,
    acked_at    INTEGER,
    FOREIGN KEY (rule_id) REFERENCES alert_rules(id)
);

-- 系统信息表
CREATE TABLE system_info (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    updated_at  INTEGER NOT NULL
);
```

### 2.4 API 设计原则

- RESTful 风格，JSON 格式响应
- 统一响应体：`{code, message, data, timestamp}`
- 版本控制：URL 路径版本号 `/api/v1/`
- CORS 支持：配置允许的源
- 速率限制：基于 Token 的请求限流

---

## 3. 客户端技术方案

### 3.1 PC CLI 架构

CLI 工具基于 cobra 框架构建命令体系，使用 bubbletea 实现实时 TUI Dashboard。

**核心命令设计：**

| 命令 | 功能 | 示例 |
|-----|------|-----|
| `cloudguard status` | 查看实时状态 | `cloudguard status --format json` |
| `cloudguard metrics` | 查询历史指标 | `cloudguard metrics cpu --range 24h` |
| `cloudguard alerts` | 告警管理 | `cloudguard alerts list --status active` |
| `cloudguard config` | 配置管理 | `cloudguard config set alert.cpu.threshold 90` |
| `cloudguard dashboard` | 实时看板 | `cloudguard dashboard` |
| `cloudguard connect` | 连接服务器 | `cloudguard connect 1.2.3.4:8080 --token xxx` |

**CLI 项目结构：**

```
cloudguard-cli/
├── cmd/
│   └── cloudguard/
│       └── main.go
├── internal/
│   ├── cmd/                     # 命令定义
│   │   ├── root.go
│   │   ├── status.go
│   │   ├── metrics.go
│   │   ├── alerts.go
│   │   ├── config.go
│   │   ├── dashboard.go
│   │   └── connect.go
│   ├── client/                  # API 客户端
│   │   ├── http.go
│   │   └── websocket.go
│   ├── tui/                     # TUI Dashboard
│   │   ├── model.go
│   │   ├── view.go
│   │   └── update.go
│   └── output/                  # 输出格式化
│       ├── table.go
│       ├── json.go
│       └── yaml.go
├── go.mod
└── go.sum
```

### 3.2 Android App 架构

采用 MVVM 架构模式，Jetpack Compose 构建 UI。

**架构分层：**

- **UI 层：** Jetpack Compose + Material 3，声明式 UI 组件
- **ViewModel 层：** 管理 UI 状态，使用 StateFlow
- **Repository 层：** 数据仓库，协调网络和本地缓存
- **网络层：** Retrofit + OkHttp，WebSocket 实时连接
- **本地存储：** DataStore (Preferences) + Room（历史缓存）

**Android 项目结构：**

```
cloudguard-android/
├── app/src/main/java/com/cloudguard/monitor/
│   ├── CloudGuardApp.kt              # Application
│   ├── MainActivity.kt
│   ├── di/                            # 依赖注入 (Hilt)
│   │   └── AppModule.kt
│   ├── data/
│   │   ├── remote/                    # 网络层
│   │   │   ├── ApiService.kt          # Retrofit 接口
│   │   │   ├── WebSocketManager.kt
│   │   │   └── dto/                   # 数据传输对象
│   │   ├── local/                     # 本地存储
│   │   │   ├── PrefsDataStore.kt
│   │   │   └── MetricsDao.kt          # Room DAO
│   │   └── repository/
│   │       ├── MetricsRepository.kt
│   │       └── AlertRepository.kt
│   ├── domain/
│   │   └── model/                     # 领域模型
│   │       ├── ServerStatus.kt
│   │       ├── Metric.kt
│   │       └── Alert.kt
│   └── ui/
│       ├── theme/                     # Material 3 主题
│       ├── navigation/                # 导航
│       ├── dashboard/                 # 仪表盘页面
│       │   ├── DashboardScreen.kt
│       │   └── DashboardViewModel.kt
│       ├── metrics/                   # 历史图表页面
│       │   ├── MetricsScreen.kt
│       │   └── MetricsViewModel.kt
│       ├── alerts/                    # 告警中心
│       │   ├── AlertsScreen.kt
│       │   └── AlertsViewModel.kt
│       └── settings/                  # 设置页面
│           ├── SettingsScreen.kt
│           └── SettingsViewModel.kt
└── app/build.gradle.kts
```

**关键技术决策：**

- 推送通知采用 **前台服务 + WebSocket** 方案，避免依赖 FCM（国内不可用）
- 图表库优先考虑 **Vico**（Compose 原生），备选 MPAndroidChart
- 网络状态监听 + 自动重连机制

---

## 4. 通信协议设计

### 4.1 REST API 规范

**统一响应格式：**

```json
{
  "code": 200,
  "message": "success",
  "data": { ... },
  "timestamp": 1711543200
}
```

**错误响应格式：**

```json
{
  "code": 401,
  "message": "invalid token",
  "data": null,
  "timestamp": 1711543200
}
```

**通用查询参数：**

| 参数 | 类型 | 说明 |
|-----|------|-----|
| `start` | int64 | 起始时间戳 (Unix) |
| `end` | int64 | 结束时间戳 (Unix) |
| `interval` | string | 聚合间隔 (1m/5m/1h/1d) |
| `format` | string | 响应格式 (json/csv) |

### 4.2 WebSocket 协议

WebSocket 采用 JSON 格式消息，包含两种消息类型：

**指标数据推送（metrics）：**

```json
{
  "type": "metrics",
  "timestamp": 1711543200,
  "data": {
    "cpu": { "usage": 45.2, "load1": 1.5, "load5": 1.2, "load15": 0.9 },
    "memory": { "total": 8192, "used": 4096, "available": 3500, "swap_used": 0 },
    "disk": [{ "mount": "/", "usage": 65.3, "read_speed": 1024, "write_speed": 512 }],
    "network": { "rx_bytes": 102400, "tx_bytes": 51200, "connections": 42 }
  }
}
```

**告警事件推送（alert）：**

```json
{
  "type": "alert",
  "timestamp": 1711543200,
  "data": {
    "id": 1,
    "rule_name": "CPU High",
    "status": "firing",
    "value": 95.5,
    "threshold": 90.0,
    "message": "CPU usage exceeded 90% for 60 seconds"
  }
}
```

**心跳机制：** 客户端每 30 秒发送 ping，服务端回复 pong。超过 60 秒无心跳则断开连接。

---

## 5. 安全方案

### 5.1 TLS 加密

服务端支持两种 TLS 证书方案：

- **自签名证书：** 服务端启动时自动生成，客户端首次连接时信任并缓存
- **Let's Encrypt：** 支持通过 ACME 自动申请和续期（需域名）

### 5.2 认证流程

API Token 认证流程：

1. 服务端启动时生成 256 位随机 Token
2. Token 哈希存储在本地配置文件
3. 客户端通过 `Authorization: Bearer <token>` 请求头传递
4. 服务端对比 Token 哈希值验证

---

## 6. 风险评估

| 风险项 | 影响程度 | 发生概率 | 缓解措施 |
|-------|---------|---------|---------|
| SQLite 并发性能瓶颈 | 中 | 低 | 当前单机场景足够，后续可替换为时序数据库 |
| 国内推送通知不可靠 | 高 | 高 | 采用前台服务+WebSocket 保活方案 |
| TLS 证书管理复杂度 | 低 | 中 | 提供自签名 + ACME 两种方案 |
| Go 交叉编译兼容性 | 低 | 低 | 使用 CGo 只影响服务端，CLI 可纯 Go |
| Android 屏幕适配 | 中 | 中 | Compose 响应式布局 + 多屏幕测试 |

---

## 7. 部署方案

### 7.1 服务端部署

提供一键安装脚本，支持主流 Linux 发行版：

1. 下载预编译二进制文件
2. 创建 systemd 服务文件
3. 生成默认配置和初始 Token
4. 启动服务并设置开机自启

**支持架构：** amd64, arm64

**支持系统：** Ubuntu 20.04+, Debian 11+, CentOS 7+, RHEL 8+

### 7.2 一键安装脚本示例

```bash
curl -fsSL https://install.cloudguard.dev | bash
# 或
wget -qO- https://install.cloudguard.dev | bash
```

安装完成后输出：

```
✅ CloudGuard Monitor installed successfully!
📍 API Endpoint: https://<server-ip>:8443
🔑 Your API Token: <generated-token>
📱 Use this token to connect from your phone or PC
```

### 7.3 systemd 服务配置

```ini
[Unit]
Description=CloudGuard Monitor Server
After=network.target

[Service]
Type=simple
User=cloudguard
ExecStart=/usr/local/bin/cloudguard server --config /etc/cloudguard/config.yaml
Restart=always
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```
