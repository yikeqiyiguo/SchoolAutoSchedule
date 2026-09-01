# 学校智能排课系统 · Web 线上版

`online-platform/` 是面向服务器部署的线上版本，由两部分组成：

```
online-platform/
├── backend/     # Go API 服务（纯 HTTP，内嵌前端兜底，支持 --web-dir 托管构建产物）
└── frontend/    # Vue3 + Vite 前端工程
```

## 快速开始（本地联调）

1. 启动后端

   ```powershell
   cd backend
   go run . --port 8000
   ```

2. 启动前端（开发模式）

   ```powershell
   cd frontend
   npm install
   npm run dev
   ```

   浏览器打开 http://localhost:5173 ，登录账号 `admin / admin123`。

## 生产部署

```powershell
# 1. 构建前端
cd frontend
npm run build

# 2. 构建后端
cd ..\backend
go build -o build\server.exe .

# 3. 启动服务（托管前端产物）
.\build\server.exe --port 8000 --web-dir ..\frontend\dist
```

详细说明见 `backend/README.md` 与 `frontend/README.md`。
