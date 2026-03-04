# airgate-openai

AirGate OpenAI 网关插件，支持 API Key 和 OAuth 两种账号类型。

## 目录结构

```
├── backend/                Go 后端（插件主体）
│   ├── main.go             入口
│   ├── internal/gateway/   网关核心逻辑（含 WebSocket 客户端）
│   ├── resources/          嵌入资源（instructions.md）
│   └── cmd/chat/           交互式测试工具
├── web/                    前端（自定义账号表单组件）
└── plugin.yaml             插件描述文件
```

## 构建

```bash
# 后端
cd backend && go build -o airgate-openai .

# 前端
cd web && npm install && npm run build
```
