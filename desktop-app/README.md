# 学校智能排课系统 · 本地 exe 版（desktop-app）

双击 `build/scheduler.exe` 即可运行：程序启动内置 HTTP 服务并自动打开浏览器，
系统托盘提供「打开系统 / 打开数据目录 / 退出」菜单。

## 特性

- Go 编译为单个 exe，内嵌前端 + SQLite，免安装、绿色便携
- 系统托盘常驻，关闭窗口不退出，后台持续提供本机服务
- 数据目录跟随 exe 所在位置（`data/`、`backups/`、`logs/`），拷贝整个目录即可迁移
- 支持 `SAS_HOST` / `SAS_PORT` / `SAS_DATA_DIR` / `SAS_HEADLESS` 环境变量

## 构建

```powershell
cd desktop-app
go build -o build/scheduler.exe .
```

## 端到端测试

```powershell
go test -v -run TestE2E -count=1 -timeout 10m
```

测试会自动以 `SAS_HEADLESS=1` 启动服务，覆盖登录、基础数据、排课、课表、统计、导出、备份、权限全链路。

## 目录结构

```
desktop-app/
├── main.go            # 入口：托盘 + 内嵌前端 + 自动备份
├── internal/
│   ├── api/           # HTTP API + 内嵌前端静态资源（web/ 子目录）
│   ├── auth/          # 用户/会话/权限
│   ├── sched/         # 排课算法
│   ├── store/         # SQLite 存储
│   ├── config/        # 运行参数（环境变量）
│   └── backup/        # 备份/恢复
├── e2e_test.go        # 端到端测试
└── build/             # 编译产物
```

> 数据目录默认与 exe 同级的 `data/`，备份在 `backups/`。可用 `SAS_DATA_DIR` 指定其它位置。
