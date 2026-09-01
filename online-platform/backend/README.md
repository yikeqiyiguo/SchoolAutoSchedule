# 线上版 · Go API 后端

纯 HTTP 服务（无托盘、无自动打开浏览器），适合部署到服务器。前端由 `--web-dir` 指向的 Vue 构建产物提供，也可使用内置前端兜底。

## 构建

```powershell
go build -o build/server.exe .
```

## 运行

```powershell
# 基本运行（监听 0.0.0.0:8000，数据在 exe 同级 data/）
.\build\server.exe

# 指定端口与前端目录（推荐：托管 frontend/dist）
.\build\server.exe --port 8000 --web-dir ..\frontend\dist

# 指定数据目录
.\build\server.exe --data-dir D:\sas-data --web-dir ..\frontend\dist
```

## 参数

| 参数 | 说明 | 默认 |
| --- | --- | --- |
| `--host` | 监听地址 | `0.0.0.0` |
| `--port` | 监听端口 | `8000` |
| `--data-dir` | 数据目录（`scheduler.db` / `backups` / `logs`） | exe 同级 `data/` |
| `--web-dir` | 前端静态目录（Vue dist 产物） | 内置前端 |

也兼容环境变量 `SAS_HOST` / `SAS_PORT` / `SAS_DATA_DIR`。

## 部署建议（Linux）

```bash
# 交叉编译
GOOS=linux GOARCH=amd64 go build -o build/server .

# 服务器运行（可选 systemd / supervisor 守护）
./build/server --port 8000 --web-dir /opt/sas/frontend/dist --data-dir /var/lib/sas
```

## 目录结构

```
backend/
├── main.go            # 入口：参数解析 + HTTP 服务 + 每日自动备份
└── internal/
    ├── api/           # HTTP API + 静态托管（web/ 为内置前端，ExternalWebDir 支持外部 dist）
    ├── auth/          # 用户 / 会话 / 权限
    ├── sched/         # 排课算法
    ├── store/         # SQLite 存储
    ├── config/        # 运行参数
    └── backup/        # 备份 / 恢复
```
