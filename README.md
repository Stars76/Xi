<div align="center">

<img src="resources/icon.png" alt="526Hz" width="84" />

# 526Hz

**一款纯 Go 原生 GPU 渲染的极简海洋桌面专注与待办应用**

零 Electron · 零 WebView2 · 单可执行文件（~15 MB）· 100% 本地优先

[![License: MIT](https://img.shields.io/badge/License-MIT-007CFF.svg)](LICENSE)
[![Go 1.27+](https://img.shields.io/badge/Go-1.27+-00ADD8.svg)](go.mod)
[![Platform: Windows](https://img.shields.io/badge/Platform-Windows-0066D6.svg)](#安装与使用)

</div>

---

## 界面一览

| 今日事（纸白 × 海蓝） | 专注钟 · 526Hz 蓝鲸 |
| :---: | :---: |
| ![今日事](docs/screenshots/today-light.png) | ![专注钟 526Hz 蓝鲸](docs/screenshots/focus-whale-light.png) |

| 深海夜蓝 · 弦月与灯塔 | 年度统计 · 52 周专注热力图 |
| :---: | :---: |
| ![深海夜蓝灯塔](docs/screenshots/focus-night-dark.png) | ![年度热力图](docs/screenshots/stats-heatmap.png) |

---

## 核心特性

- **极简原生架构**：基于 Go + [`github.com/egoist/mygo`](https://github.com/egoist/mygo)（Direct2D / D3D11 原生 GPU 渲染），不打包浏览器内核，冷启动迅捷、常驻轻量。
- **今日事待办工作台**：
  - 顶部精干纯纸面抬头 + 底部 32px 526Hz 潮汐声波细带（待机静水微息、专注轻涌、完成任务泛起海蓝微澜）。
  - 支持子任务拆解、拖拽排序、智能闹钟解析（输入 `16:30 团队同步` 自动设闹钟）、未完自动顺延、每日固定任务模板，以及底部可折叠「已完成」分组。
- **沉浸式专注钟 & 60fps 动态海面生态**：
  - 支持多轮番茄计划、自定义时长、置顶画中画小窗（与主窗 1:1 共享同一片海）及 `264×50` 悬浮灵动条（带底边 `2.5px` 海蓝进度线）。
  - **天色光线**：随本地时间在晨光、晴昼、暮色、星夜间自动过渡，搭配体积云、矢量振翅海鸟、发光弦月与海面月影。
  - **5 种天气 & 实况同步**：晴空、细雨（浪面真实雨花）、飞雪、雷雨（分叉矢量闪电）、薄雾（视差海雾），并支持可选的 Open-Meteo 实时天气同步。
  - **6 种可互动海洋彩蛋**：垂钓小船、海盗船、526Hz 蓝鲸、海豚群、灯塔、极光——全部采用真实 Gerstner 波浪起伏与刚体船身俯仰倾斜，点击触发微声光互动并记入图鉴。
- **立体声程序化天气环境音**：纯 Go 实时合成海浪、细雨、雷雨与微风 4 套立体声环境白噪音，专注计时自动淡入、暂停自动淡出，无需任何外部音频文件。
- **日历、统计与 Markdown 复盘**：
  - 日历视图直接展示每日专注分钟数与任务完成状态。
  - 统计页提供每日 24h 时间轴（支持手写补录与覆写）、周/月时段分布与 52 周纯海蓝年度热力图。
  - 内置 Obsidian / Notion Markdown 复盘模板（每日复盘 / 单任务 / 周报），一键复制或导出 `.md`。
- **本地安全存储与桌面常驻**：
  - 数据 100% 存储在本地（原子写入、滚动备份、一键导入/导出 JSON），默认零网络上传。
  - 支持系统托盘常驻、Windows 开机静默自启（`--start-hidden`），并无缝兼容历史存档目录。

---

## 安装与使用

### 直接下载

前往 [Releases](https://github.com/Stars76/526Hz/releases) 下载最新 `526Hz.exe`，双击即可运行，无需安装额外运行时。

### 启动参数

```powershell
# 默认：打开主窗口并常驻系统托盘
.\526Hz.exe

# 静默启动到系统托盘（开机自启默认使用）
.\526Hz.exe --start-hidden

# 直接打开置顶专注钟画中画小窗
.\526Hz.exe --compact-timer

# 指定独立数据目录运行（用于测试或多配置隔离）
.\526Hz.exe --user-data .\my-data
```

---

## 从源码构建（Windows PowerShell）

环境要求：**Go 1.27+**（Windows amd64）

```powershell
# 1. 静态检查（含 mygo UI 框架检查）
go vet -tags mygo_nowebview ./...
go tool mygo vet

# 2. 运行全部单元测试
go test -tags mygo_nowebview -count=1 ./...

# 3. 编译纯原生单文件（产物位于 release\windows-amd64\526Hz.exe）
$env:GOFLAGS = "-tags=mygo_nowebview -buildvcs=false"
go tool mygo build -skip-installer
```

---

## 目录结构

```text
526Hz/
├── main.go                     # 单实例锁、CLI 参数、数据目录兼容与启动编排
├── mygo.json                   # MyGo 应用元数据（526Hz / 3.0.0）
├── resources/icon.png          # 应用图标
├── docs/screenshots/           # README 实景展示图
└── internal/
    ├── assets/                 # 嵌入图标与托盘模板资源
    ├── dateutil/               # 本地日期规范化、顺延差值与 HH:MM 闹钟解析
    ├── store/                  # Schema v2 本地存储引擎（原子写入、滚动备份、导入导出）
    ├── timer/                  # 专注钟状态机、多轮计划与任务闹钟调度
    ├── todo/                   # 今日事、子任务、每日模板与未完顺延
    ├── stats/                  # 日/周/月/年统计、24h 时间轴与 52 周热力图聚合
    ├── notes/                  # Obsidian / Notion Markdown 复盘模板引擎
    ├── sound/                  # 纯 Go 实时立体声环境白噪音与微音效合成
    ├── weather/                # Open-Meteo 实时天气查询客户端（可选开启）
    └── uiapp/                  # 纯 mygo/ui 原生界面、60fps 波浪物理引擎与托盘
```

---

## 开源协议

本项目基于 [MIT License](LICENSE) 开源。
