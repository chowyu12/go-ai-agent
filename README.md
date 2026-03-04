# Go AI Agent

基于 Go + Vue 3 构建的 AI Agent 管理与执行平台，支持多模型供应商接入、工具调用、技能编排和多轮对话。

![管理后台](portal.png)

## 功能特性

### Agent 管理
- Agent 增删改查，支持设置名称、UUID、系统提示词、模型参数（温度、最大 token 等）
- 每个 Agent 可关联多个工具（Tools）、技能（Skills）和子 Agent
- 支持工具优先执行策略，Agent 自动判断是否需要调用工具

### 模型供应商
- 支持多种 LLM Provider：OpenAI、Qwen（通义千问）、Kimi、OpenRouter、NewAPI
- 可配置 Base URL、API Key、可用模型列表
- 创建 Agent 时自动拉取供应商模型列表，支持搜索过滤

### 工具系统
- 内置工具：获取当前时间、UUID 生成、计算器、Base64 编解码、JSON 格式化、文本哈希、随机数生成
- HTTP 工具：天气查询、IP 查询、URL 内容读取
- 支持自定义 HTTP 工具和脚本工具
- 工具执行过程全链路追踪

### 技能系统
- 预置技能：翻译助手、代码审查、文章摘要、写作助手、数据分析、SQL 助手
- 每个技能可关联多个工具，按指令模板执行
- 支持自定义技能和指令

### 对话与记忆
- 支持多轮对话，自动维护上下文
- 对话历史持久化存储（MySQL）
- 支持流式（SSE）和阻塞式两种响应模式
- 流式响应实时展示执行步骤（Dify 风格）

### 执行日志
- 完整记录每次 Agent 调用的执行链路
- 详细记录每个步骤：LLM 调用、工具调用、子 Agent 调用
- 包含输入输出、耗时、Token 用量、错误信息等

### 用户系统
- 超管（admin）和访客（guest）两种角色
- JWT 认证，支持 Token 自动续期
- 首次访问引导创建超级管理员
- 访客只读，无法进行新增、编辑、删除操作
- 超管可管理用户（创建、禁用、删除、修改角色）

### 管理后台
- 现代化 Web UI（Vue 3 + Element Plus）
- Dashboard 概览
- 供应商、Agent、工具、技能、用户的 CRUD 管理
- 对话测试 Playground
- 执行日志查看器
- 前端编译后嵌入 Go 二进制，单文件部署

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.25、net/http、logrus |
| AI 编排 | langchaingo（LLM 调用、Function Calling） |
| 数据库 | MySQL |
| 认证 | JWT（golang-jwt/v5）、bcrypt |
| 前端 | Vue 3、TypeScript、Element Plus、Pinia、Vue Router |
| 构建 | Go embed、Vite |

## 项目结构

```
go-ai-agent/
├── cmd/
│   ├── server/              # 主服务入口
│   └── test_agent/          # Agent 测试 CLI
├── etc/
│   └── config.yaml          # 配置文件
├── internal/
│   ├── agent/               # Agent 执行器、工具注册、记忆管理、步骤追踪
│   ├── config/              # 配置解析
│   ├── handler/             # HTTP Handler（Agent/Provider/Tool/Skill/Chat/Auth）
│   ├── model/               # 领域模型定义
│   ├── provider/            # LLM Provider 适配层
│   ├── seed/                # 初始化种子数据（默认工具和技能）
│   └── store/
│       └── mysql/           # MySQL 数据访问实现
├── migrations/              # SQL 迁移脚本
├── pkg/
│   ├── httputil/            # HTTP 响应工具
│   └── sse/                 # SSE 流式响应工具
├── web/                     # Vue 前端项目
│   ├── src/
│   │   ├── api/             # API 调用封装
│   │   ├── components/      # 通用组件
│   │   ├── router/          # 路由配置 + 权限守卫
│   │   ├── stores/          # Pinia 状态管理
│   │   └── views/           # 页面（Dashboard/Agent/Provider/Tool/Skill/Chat/Log/User/Login/Setup）
│   └── embed.go             # Go embed 入口
├── Makefile
├── go.mod
└── README.md
```

## 快速开始

### 前置要求

- Go 1.25+
- Node.js 18+
- MySQL 8.0+

### 1. 克隆项目

```bash
git clone https://github.com/chowyu12/go-ai-agent.git
cd go-ai-agent
```

### 2. 初始化数据库

```bash
mysql -u root -p -e "CREATE DATABASE go_ai_agent DEFAULT CHARSET utf8mb4"
mysql -u root -p go_ai_agent < migrations/001_init.sql
mysql -u root -p go_ai_agent < migrations/002_execution_steps.sql
mysql -u root -p go_ai_agent < migrations/003_users.sql
```

### 3. 修改配置

编辑 `etc/config.yaml`，填入数据库连接信息：

```yaml
server:
  host: "0.0.0.0"
  port: 8080

database:
  driver: mysql
  dsn: "user:password@tcp(127.0.0.1:3306)/go_ai_agent?charset=utf8mb4&parseTime=True&loc=Local"

log:
  level: info

jwt:
  secret: "your-secret-key"
  expire_hours: 24
```

### 4. 安装依赖并启动

```bash
# 安装 Go 依赖
go mod tidy

# 安装前端依赖
cd web && npm install && cd ..

# 构建前端 + 启动服务
make dev
```

浏览器访问 `http://localhost:8080`，首次打开会引导创建超级管理员账号。

### 5. 配置模型供应商

登录后进入「模型供应商」页面，添加至少一个 LLM Provider（如 OpenAI），填入 API Key 和 Base URL。

### 6. 创建 Agent 开始对话

进入「Agent 管理」创建 Agent，选择模型、配置工具和技能，然后在「对话测试」中体验。

## 常用命令

```bash
make build            # 编译后端二进制（含嵌入前端）
make dev              # 开发模式启动（自动构建前端）
make test             # 运行所有测试
make build-frontend   # 单独构建前端
make dev-frontend     # 前端开发模式（热更新，需单独启动后端）
make clean            # 清理构建产物
make deps             # 整理 Go 依赖
```

## API 接口

所有接口以 `/api/v1` 为前缀，除登录和初始化接口外均需 JWT 认证。

| 方法 | 路径 | 说明 | 权限 |
|------|------|------|------|
| GET | `/auth/setup-check` | 检查系统是否已初始化 | 公开 |
| POST | `/auth/setup` | 首次创建超管 | 公开 |
| POST | `/auth/login` | 登录 | 公开 |
| GET | `/auth/me` | 获取当前用户信息 | 登录 |
| GET/POST/PUT/DELETE | `/providers` | 供应商管理 | 读：登录 / 写：超管 |
| GET/POST/PUT/DELETE | `/agents` | Agent 管理 | 读：登录 / 写：超管 |
| GET/POST/PUT/DELETE | `/tools` | 工具管理 | 读：登录 / 写：超管 |
| GET/POST/PUT/DELETE | `/skills` | 技能管理 | 读：登录 / 写：超管 |
| POST | `/chat/completions` | 阻塞式对话 | 登录 |
| POST | `/chat/stream` | 流式对话（SSE） | 登录 |
| GET/DELETE | `/conversations` | 会话管理 | 登录 |
| GET/POST/PUT/DELETE | `/users` | 用户管理 | 超管 |

## 部署

项目支持单文件部署，构建后的二进制文件已包含前端静态资源：

```bash
make all
./bin/go-ai-agent -config etc/config.yaml
```
