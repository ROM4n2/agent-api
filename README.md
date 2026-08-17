# Agent API

异步任务 + Worker Pool + LLM 调用的 Go 后端服务。客户端提交 prompt，服务异步调用 DeepSeek 生成结果，通过轮询获取。

## Architecture

```text
客户端                     服务器
  │ POST /run (prompt)     │
  │───────────────────────→│ api: store.Create(prompt) → task_id (202)
  │                         │         ▼
  │                         │   queue(chan string)
  │                         │         ▼
  │                         │  [worker 1][worker 2][worker 3]
  │                         │   store.Update(running) → llm.Chat(prompt)
  │                         │         ▼
  │                         │   store.Complete → done / failed
  │ GET /tasks/{id} 轮询    │
  │───────────────────────→│ api: store.Get(id) → Task JSON (200)
```

## Quick Start

### 本地运行

```powershell
$env:DEEPSEEK_API_KEY = "sk-你的key"
go run .
```

### Docker

```powershell
docker build -t agent-api .
docker run -e DEEPSEEK_API_KEY=sk-你的key -p 8080:8080 agent-api
```

### 验证

```powershell
# 提交任务
Invoke-RestMethod -Uri http://localhost:8080/run -Method Post `
  -ContentType 'application/json' `
  -Body ([System.Text.Encoding]::UTF8.GetBytes('{"prompt":"用一句话解释goroutine"}'))

# 查询结果（task_id 换成实际返回的值）
Invoke-RestMethod -Uri http://localhost:8080/tasks/1
```

## API

| Method | Path | 说明 | Request Body | Response |
|--------|------|------|-------------|----------|
| POST | `/run` | 提交任务 | `{"prompt":"..."}` | `202 {"task_id":"1"}` |
| GET | `/tasks/{id}` | 查询状态 | - | `{"ID":"1","Status":"done","Prompt":"...","Result":"...","Error":""}` |

Status 取值：`pending` → `running` → `done` / `failed`

并发上限 3 个任务同时执行，超出部分排队等待。

## Project Structure

```
agent-api/
├── main.go            # 入口：环境变量 → 组装依赖 → 启动服务
├── api/               # HTTP 层：路由、请求解析、响应编码
├── store/             # 任务存储：内存 map + Mutex，状态机
├── worker/            # Worker Pool：goroutine 消费队列，调用 LLM
├── llm/               # LLM 客户端：封装 DeepSeek API 调用
├── docs/              # 项目文档、面试准备
└── Dockerfile         # 多阶段构建
```

## Tech Stack

Go 1.26 · slog · Docker · DeepSeek API · `net/http` (stdlib)

## License

MIT
