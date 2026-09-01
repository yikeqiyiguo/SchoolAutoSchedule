# 学校智能自定义排课系统

多约束智能排课 · 可视化微调 · 多格式导出（Excel / PDF）。同一套系统提供两种运行形态：

| 形态 | 目录 | 适用场景 |
| --- | --- | --- |
| 🖥️ 本地 exe 版 | `desktop-app/` | 单机部署：双击 exe 即用，内置托盘、浏览器自动打开、SQLite 本地存储 |
| 🌐 Web 线上版 | `online-platform/` | 服务器部署：Vue3 + Vite 前端 + Go API 服务，浏览器访问，数据可持久化于服务器 |

## 快速开始

### 本地 exe 版

```powershell
cd desktop-app
go build -o build/scheduler.exe .
.\build\scheduler.exe
```

双击运行后自动打开浏览器，数据存放在 exe 同级 `data/` 目录。系统托盘提供打开系统 / 打开数据目录 / 退出。

### Web 线上版

后端（API 服务）：

```powershell
cd online-platform/backend
go build -o build/server.exe .
.\build\server.exe --port 8000 --web-dir ..\frontend\dist
```

前端（Vue 工程，开发 / 构建）：

```powershell
cd online-platform/frontend
npm install
npm run dev      # 开发模式，http://localhost:5173（/api 自动代理到 8000）
npm run build    # 生产构建，产物输出到 dist/，由后端 --web-dir 托管
```

> 默认管理员账号：`admin / admin123`（首次启动自动初始化）。

## 目录结构

```
SchoolAutoSchedule/
├── desktop-app/                 # 本地 exe 版（Go 单文件，内嵌前端 + 托盘）
│   ├── main.go
│   ├── internal/                # api / auth / sched / store / config / backup
│   ├── e2e_test.go              # 端到端测试
│   └── build/scheduler.exe
├── online-platform/             # Web 线上版
│   ├── backend/                 # Go API 服务（纯 HTTP，可部署 Linux/Windows 服务器）
│   │   ├── main.go
│   │   └── internal/            # 与桌面版同源的核心逻辑
│   └── frontend/                # Vue3 + Vite 前端工程
│       ├── src/                 # 入口 main.js + 页面组件（legacy 全局组件 + 工具模块）
│       └── dist/                # npm run build 产物
└── README.md
```

## 功能一览

- 基础数据：年级 / 班级 / 科目 / 教师 / 任课关系，支持 Excel 导入模板
- 智能排课：贪心放置 + 软约束打分（主科优先、连排限制、教师均衡）+ 冲突检测兜底
- 可视化微调：单元格手改、拖拽交换、一键重排
- 多视图：班级课表 / 教师课表 / 课时统计
- 导出：全校班级课表、全校教师课表、单班 / 单教师 Excel 与 PDF、统计报表
- 系统：多角色权限（超级管理员 / 管理员 / 教师 / 访客）、操作日志、备份恢复、每日自动备份

## 测试

桌面版含端到端测试（启动服务并覆盖全链路）：

```powershell
cd desktop-app
go test -v -run TestE2E -count=1 -timeout 10m
```

## 常见环境变量

| 变量 | 说明 | 默认 |
| --- | --- | --- |
| `SAS_HOST` | 监听地址 | 桌面版 `127.0.0.1`；服务版 `0.0.0.0` |
| `SAS_PORT` | 监听端口 | `8000` |
| `SAS_DATA_DIR` | 数据目录（存放 `scheduler.db` / 备份 / 日志） | exe 同级 |
| `SAS_HEADLESS` | `1` 时仅运行服务（无托盘，供自动化测试） | - |
