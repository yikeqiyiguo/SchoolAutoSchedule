# 线上版 · Vue3 + Vite 前端

学校智能排课系统的 Web 前端工程（Vue 3 + Vite）。页面组件与桌面版同源（`src/legacy/`），以工程化方式组织、构建与迭代。

## 环境要求

- Node.js 18+
- npm 9+

## 使用

```powershell
npm install       # 安装依赖
npm run dev       # 开发模式：http://localhost:5173，/api 自动代理到 http://127.0.0.1:8000
npm run build     # 生产构建：产物输出到 dist/
npm run preview   # 本地预览构建产物
```

开发时需先启动后端：

```powershell
cd ..\backend
go run . --port 8000
```

## 目录结构

```
frontend/
├── index.html            # SPA 入口
├── vite.config.js        # dev 代理 /api → 后端
├── src/
│   ├── main.js           # 应用入口：注册组件、路由、全局工具
│   ├── api.js            # 请求封装（fetch + 统一错误处理）
│   ├── util.js           # toast / 格式化 / 确认弹窗等工具
│   └── legacy/           # 页面组件（与桌面版同源，逐步迁移为 SFC）
│       ├── css/app.css
│       └── js/pages_*.js
└── dist/                 # 构建产物（由后端 --web-dir 托管）
```

## 与后端联调

- 开发：`vite` 已配置 `/api` 代理到 `http://127.0.0.1:8000`，无需跨域配置。
- 生产：将 `dist/` 交给后端 `--web-dir` 托管（同源部署），或由 Nginx 静态托管并将 `/api` 反向代理到后端。
