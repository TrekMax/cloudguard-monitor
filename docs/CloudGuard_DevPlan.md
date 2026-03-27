# CloudGuard Monitor — 开发计划

> **版本:** v1.0 | **日期:** 2026-03-27

---

## 目录

- [1. 开发原则与规范](#1-开发原则与规范)
- [2. 里程碑规划](#2-里程碑规划)
- [3. Sprint 详细计划](#3-sprint-详细计划)
- [4. Claude Code 开发工作流](#4-claude-code-开发工作流)
- [5. 质量保障](#5-质量保障)

---

## 1. 开发原则与规范

### 1.1 开发原则

- **Claude Code CLI 驱动开发：** 最大化利用 AI 辅助编码提升效率
- **测试先行：** 每个模块先写测试再实现，确保质量
- **渐进式交付：** 每个 Sprint 产出可运行的版本
- **文档同步：** 代码和文档同步更新，保持 README 最新

### 1.2 代码规范

| 语言 | 规范 | 工具 |
|-----|------|-----|
| Go | Effective Go + 项目规范 | golangci-lint, gofmt |
| Kotlin | Kotlin 官方编码规范 | ktlint, detekt |
| API | RESTful + OpenAPI 3.0 | swagger-codegen |
| Git | Conventional Commits | commitlint |

### 1.3 分支策略

- `main` — 稳定版本，只通过 PR 合并
- `develop` — 开发主分支
- `feature/*` — 功能分支，从 develop 分出
- `release/*` — 发布分支

### 1.4 Monorepo 结构

```
cloudguard/
├── server/                    # Go 服务端（Agent + API）
├── cli/                       # Go CLI 客户端
├── android/                   # Android App
├── docs/                      # 项目文档
│   ├── prd.md
│   ├── tech-assessment.md
│   └── api/                   # OpenAPI 文档
├── scripts/                   # 公共脚本
├── .github/workflows/         # CI/CD
├── CLAUDE.md                  # Claude Code 项目说明
├── Makefile
└── README.md
```

---

## 2. 里程碑规划

### 2.1 总体时间线

| 里程碑 | 时间 | 目标 | 交付物 |
|-------|-----|------|-------|
| M1: 基础架构 | 第 1-2 周 | 项目初始化 + 基础采集 | 可运行的 Agent 骨架 |
| M2: 核心服务端 | 第 3-5 周 | 完整的采集 + 存储 + API | 服务端 v0.1 |
| M3: CLI 客户端 | 第 6-7 周 | CLI 工具完整功能 | CLI v0.1 |
| M4: Android App | 第 8-12 周 | Android App 核心功能 | App v0.1 |
| M5: 告警与通知 | 第 13-15 周 | 告警 + WebSocket + 推送 | 全端 v0.2 |
| M6: 安全与发布 | 第 16-18 周 | TLS + 部署脚本 + 测试 | v1.0 Release |

### 2.2 里程碑依赖关系

```
M1 (基础架构)
 └─→ M2 (核心服务端)
      ├─→ M3 (CLI 客户端)      ← M2 完成后 CLI 和 App 可并行
      ├─→ M4 (Android App)     ← 依赖 M2 的 API 接口
      └─→ M5 (告警与通知)       ← 依赖 M2 + M3/M4 部分完成
           └─→ M6 (安全与发布)
```

---

## 3. Sprint 详细计划

### 3.1 M1: 基础架构（第 1-2 周）

#### Sprint 1: 项目初始化（第 1 周）

**目标：** 搭建项目骨架，完成基础设施

- [ ] Go Module 初始化，设定项目结构
- [ ] 配置文件解析模块（YAML）
- [ ] 日志模块（slog 结构化日志）
- [ ] CI 配置（GitHub Actions: lint + test）
- [ ] CLAUDE.md 编写
- [ ] Makefile 编写（build/test/lint/run）

**验收标准：** `make build` 成功编译，`make test` 通过，`make lint` 无告警

#### Sprint 2: 基础采集器（第 2 周）

**目标：** 实现 CPU 和内存采集，验证采集框架

- [ ] Collector 接口定义
- [ ] CPU 采集器实现 + 单元测试
- [ ] 内存采集器实现 + 单元测试
- [ ] 采集调度器（定时采集 + 内存缓存）
- [ ] 采集器集成测试

**验收标准：** 程序启动后每 5 秒输出 CPU/内存指标日志

---

### 3.2 M2: 核心服务端（第 3-5 周）

#### Sprint 3: 完整采集 + 存储（第 3 周）

**目标：** 完成所有采集器，数据落盘

- [ ] 磁盘采集器（分区使用率 + IO）
- [ ] 网络采集器（流量 + 连接数）
- [ ] SQLite 存储层实现（建表 + CRUD）
- [ ] 数据降采样后台任务
- [ ] 存储层单元测试

**验收标准：** 所有指标持续采集并写入 SQLite，降采样任务正常运行

#### Sprint 4: REST API（第 4 周）

**目标：** 提供可用的 HTTP API

- [ ] Gin 路由框架搭建
- [ ] Token 认证中间件
- [ ] `GET /api/v1/status` — 实时状态接口
- [ ] `GET /api/v1/metrics` — 历史数据接口（支持时间范围查询）
- [ ] 统一错误处理与响应格式
- [ ] 请求日志中间件
- [ ] API 集成测试

**验收标准：** curl 可正常请求所有 API，认证拦截生效

#### Sprint 5: 系统信息 + 进程监控（第 5 周）

**目标：** 补充完整的系统信息采集

- [ ] 系统信息采集模块（OS/内核/主机名/启动时间）
- [ ] Top N 进程采集模块
- [ ] `GET /api/v1/system` — 系统信息接口
- [ ] `GET /api/v1/processes` — 进程列表接口
- [ ] OpenAPI 文档生成（swaggo/swag）
- [ ] API 文档自动化

**验收标准：** 所有 API 文档完整，Swagger UI 可访问

---

### 3.3 M3: CLI 客户端（第 6-7 周）

#### Sprint 6: CLI 基础功能（第 6 周）

**目标：** CLI 可连接服务器并查询数据

- [ ] cobra 命令框架搭建
- [ ] `connect` 命令 — 配置服务器连接
- [ ] `status` 命令 — 查看实时状态
- [ ] `metrics` 命令 — 查询历史指标
- [ ] 多格式输出（table / json / yaml）
- [ ] 配置文件管理（`~/.cloudguard/config.yaml`）

**验收标准：** CLI 可连接远程服务器，正常查询和展示数据

#### Sprint 7: CLI 高级功能（第 7 周）

**目标：** 完成 TUI Dashboard 和全部命令

- [ ] bubbletea 实时 Dashboard
  - CPU/内存实时进度条
  - 磁盘/网络简要信息
  - 自动刷新（可配置间隔）
- [ ] `alerts` 命令（list / ack / rules）
- [ ] `config` 命令（get / set）
- [ ] 交叉编译 + 发布流程
- [ ] CLI 集成测试

**验收标准：** `cloudguard dashboard` 展示实时 TUI 看板，所有命令正常工作

---

### 3.4 M4: Android App（第 8-12 周）

#### Sprint 8: App 基础架构（第 8 周）

**目标：** 搭建 Android 项目骨架

- [ ] Android 项目初始化（Kotlin + Compose + Gradle KTS）
- [ ] MVVM 架构搭建 + Hilt 依赖注入
- [ ] Retrofit 网络层封装
  - ApiService 接口定义
  - Token 认证拦截器
  - 统一错误处理
- [ ] 服务器连接配置页面
- [ ] DataStore 本地偏好存储

**验收标准：** App 可输入服务器地址/Token，成功请求 API 并显示原始数据

#### Sprint 9: 仪表盘页面（第 9 周）

**目标：** 实现核心 Dashboard UI

- [ ] Dashboard 页面布局（四宫格卡片）
  - CPU 使用率圆环图
  - 内存使用率进度条
  - 磁盘使用率列表
  - 网络流量实时数据
- [ ] 自动刷新机制（可配置间隔）
- [ ] 下拉刷新 + 加载状态 + 错误重试
- [ ] 离线时展示缓存数据

**验收标准：** Dashboard 实时展示服务器状态，体验流畅

#### Sprint 10: 历史图表（第 10 周）

**目标：** 实现历史数据可视化

- [ ] 图表组件集成（Vico）
- [ ] CPU 历史趋势图（折线图）
- [ ] 内存历史趋势图（面积图）
- [ ] 磁盘 IO 历史图
- [ ] 网络流量历史图
- [ ] 时间范围选择器（24h / 7d / 30d）
- [ ] 图表交互（缩放/滑动/数据点详情）

**验收标准：** 各指标历史图表准确展示，交互流畅

#### Sprint 11: 主题与多语言（第 11 周）

**目标：** 完善 UI 体验

- [ ] Material 3 主题系统
- [ ] 深色/浅色主题切换
- [ ] 动态颜色支持（Android 12+）
- [ ] 多语言支持（中文/英文）
- [ ] 底部导航栏（Dashboard / Metrics / Alerts / Settings）
- [ ] 页面过渡动画

**验收标准：** 主题切换顺滑，多语言完整

#### Sprint 12: 测试与优化（第 12 周）

**目标：** 确保 App 质量

- [ ] Compose UI 测试（核心页面）
- [ ] ViewModel 单元测试
- [ ] 屏幕适配测试（手机/平板/折叠屏）
- [ ] 性能优化（内存泄漏检测 / 过度重组检查）
- [ ] ProGuard 混淆配置
- [ ] 签名配置 + APK 构建

**验收标准：** 主流设备兼容，无内存泄漏，APK 体积 < 15MB

---

### 3.5 M5: 告警与通知（第 13-15 周）

#### Sprint 13: 告警引擎（第 13 周）

**目标：** 服务端告警功能完整

- [ ] 告警规则配置模块（CRUD）
- [ ] 阈值判断引擎
  - 支持 gt / lt / eq 操作符
  - 支持持续时间条件
  - 周期性检查
- [ ] 告警抑制逻辑（相同告警 N 分钟内不重复）
- [ ] 告警恢复检测与通知
- [ ] `GET/POST/PUT /api/v1/alerts/rules` API
- [ ] `GET /api/v1/alerts` API（分页 + 过滤）
- [ ] 告警引擎单元测试

**验收标准：** 手动设置 CPU 阈值为 1%，触发告警事件，恢复后自动标记

#### Sprint 14: WebSocket + 实时推送（第 14 周）

**目标：** 全端实时数据通道

- [ ] WebSocket Hub 实现（连接管理 + 广播）
- [ ] WebSocket 认证（首次连接验证 Token）
- [ ] 实时指标推送
- [ ] 告警事件推送
- [ ] CLI WebSocket 客户端接入
- [ ] Android WebSocket 客户端接入（OkHttp）
- [ ] Android 前台服务（后台保活）
- [ ] Android 通知渠道 + 告警通知

**验收标准：** 服务器告警时，CLI 和 App 同时收到实时通知

#### Sprint 15: 告警管理 UI（第 15 周）

**目标：** 客户端告警管理完整

- [ ] Android 告警列表页（分 firing / resolved / all）
- [ ] Android 告警详情页
- [ ] Android 告警确认操作
- [ ] Android 告警规则配置页
- [ ] CLI `alerts` 命令完善（实时告警 + 声音提示）
- [ ] 端到端测试（全链路告警流程）

**验收标准：** 从告警触发到客户端展示和确认，全流程通畅

---

### 3.6 M6: 安全与发布（第 16-18 周）

#### Sprint 16: 安全加固（第 16 周）

**目标：** 生产级安全

- [ ] TLS 支持
  - 自签名证书自动生成
  - ACME / Let's Encrypt 集成
  - 证书热更新
- [ ] IP 白名单中间件
- [ ] 审计日志模块（敏感操作记录）
- [ ] Token 轮换功能
- [ ] 安全测试

**验收标准：** HTTPS 通信正常，IP 白名单生效，审计日志记录完整

#### Sprint 17: 部署自动化（第 17 周）

**目标：** 一键部署和发布

- [ ] 一键安装脚本（`install.sh`）
  - 系统检测（OS / 架构）
  - 下载对应二进制
  - systemd 服务配置
  - 初始 Token 生成
  - 防火墙端口开放提示
- [ ] systemd 服务文件
- [ ] GitHub Actions CI/CD 完善
  - Go 多平台构建
  - Android APK 构建
  - 自动化测试
- [ ] GoReleaser 配置（自动发布 GitHub Release）
- [ ] Docker 镜像（可选）

**验收标准：** 全新 Ubuntu 服务器执行安装脚本后服务正常运行

#### Sprint 18: 测试与发布（第 18 周）

**目标：** v1.0 正式发布

- [ ] 全端集成测试
  - Server ↔ CLI 全流程
  - Server ↔ App 全流程
  - 多客户端并发
- [ ] 性能/压力测试
  - Agent 资源占用基准测试
  - API 并发测试（vegeta）
  - WebSocket 并发连接测试
- [ ] 文档完善
  - README（快速开始指南）
  - 部署指南
  - API 文档（OpenAPI）
  - 配置说明文档
  - 常见问题 FAQ
- [ ] v1.0 正式发布
  - GitHub Release（Server + CLI 多平台二进制）
  - APK 发布
  - 发布公告

**验收标准：** 所有测试通过，文档齐全，Release 发布成功

---

## 4. Claude Code 开发工作流

### 4.1 推荐工作流程

为最大化利用 Claude Code CLI 的开发效率，推荐以下工作流程：

1. **每个 Sprint 开始时，** 在 `CLAUDE.md` 中更新当前 Sprint 目标和任务列表
2. **对每个任务，** 先让 Claude Code 生成测试用例，再实现代码
3. **使用 Claude Code 的 `/review` 命令** 进行代码审查
4. **复杂模块** 先让 Claude Code 生成设计文档，确认后再实现

### 4.2 CLAUDE.md 结构建议

在项目根目录创建 `CLAUDE.md`，包含以下信息：

```markdown
# CloudGuard Monitor

## 项目概述
轻量级云服务器监控套件...

## 架构说明
Agent-Server 合一架构，单进程部署...

## 当前 Sprint
### Sprint N: [标题]
- [ ] 任务 1
- [ ] 任务 2

## 代码规范
- 错误处理：使用 fmt.Errorf + %w 包装
- 日志：使用 slog，必须携带结构化字段
- 测试：表驱动测试，覆盖率 ≥80%
- 包命名：小写单词，不使用下划线

## 常用命令
- `make build` — 编译
- `make test` — 测试
- `make lint` — 代码检查
- `make run` — 运行

## API 规范
- 统一响应格式: {code, message, data, timestamp}
- 认证: Bearer Token
- 版本: /api/v1/
```

### 4.3 Claude Code 使用技巧

| 场景 | 推荐做法 |
|-----|---------|
| 新模块开发 | 先描述需求，让 Claude Code 生成接口定义和测试 |
| Bug 修复 | 粘贴错误日志，让 Claude Code 定位并修复 |
| 代码重构 | 使用 `/review` 获取建议，再逐步重构 |
| API 开发 | 先定义 OpenAPI spec，再让 Claude Code 生成处理器 |
| 测试编写 | 描述测试场景，让 Claude Code 生成表驱动测试 |

---

## 5. 质量保障

### 5.1 测试策略

| 测试类型 | 覆盖范围 | 工具 | 目标 |
|---------|---------|-----|-----|
| 单元测试 | 采集器、存储层、告警引擎 | go test | 覆盖率 ≥80% |
| 集成测试 | API 接口、WebSocket | go test + httptest | 核心接口 100% |
| UI 测试 | Android 各页面 | Compose Testing | 核心流程覆盖 |
| 性能测试 | Agent 资源占用、API 响应时间 | pprof + vegeta | 满足 SLA |
| 端到端测试 | 全链路功能验证 | 手动 + 脚本 | 发布前必过 |

### 5.2 CI/CD 流程

```
Push ──→ lint + 单元测试
PR ────→ 全量测试 + 覆盖率检查 + 代码审查
Tag ───→ 构建 + 发布（GoReleaser / APK）
Cron ──→ 依赖安全扫描（每周）
```

**GitHub Actions 工作流：**

```yaml
# .github/workflows/ci.yml
name: CI
on: [push, pull_request]
jobs:
  server:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - run: cd server && make lint
      - run: cd server && make test
      - run: cd server && make build

  cli:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - run: cd cli && make lint
      - run: cd cli && make test
      - run: cd cli && make build

  android:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-java@v4
        with: { distribution: 'temurin', java-version: '17' }
      - run: cd android && ./gradlew lint test assembleDebug
```

### 5.3 发布检查清单

每次发布前必须确认：

- [ ] 所有测试通过（单元 + 集成 + E2E）
- [ ] 代码覆盖率达标（≥80%）
- [ ] 无高危安全漏洞
- [ ] API 文档已更新
- [ ] CHANGELOG 已更新
- [ ] README 已更新
- [ ] 多平台构建成功（linux/amd64, linux/arm64, darwin/amd64, darwin/arm64）
- [ ] Android APK 签名正确
- [ ] 安装脚本在目标系统上测试通过
