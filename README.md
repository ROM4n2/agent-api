# Agent API

异步任务 + Worker Pool + LLM 调用的 Agent API 服务 —— 简历主线项目。

> 依据项目规范 [`docs/GO-STANDARDS.md`](docs/GO-STANDARDS.md) 编写。
> 架构决策见 [`docs/adr/`](docs/adr/)。
> 本项目为独立编码项目，所有代码由本人亲手完成。

---

## 项目目标

"一鱼两吃"：一个项目同时证明 **Go 后端能力**（HTTP API、并发、存储、测试）与 **LLM/Agent 视野**（异步任务、worker pool、LLM 调用）。目标大二下找到 Go 后端实习。

## 核心架构

```text
客户端                    服务器
  │ POST /run (prompt)    │
  │──────────────────────→│ API层: store.Create(prompt) → 返回 taskID (202)
  │                        │          ▼
  │                        │    taskQueue(chan Task)
  │                        │          ▼
  │                        │  [worker 1][worker 2][worker 3]  ← worker pool
  │                        │   每个worker: 取任务→跑agent→更新状态
  │                        │          ▼
  │                        │    TaskStore(map): pending→running→done/failed
  │ GET /tasks/{id} 轮询   │
  │──────────────────────→│ API层: store.Get(id) → 返回状态 (200)
```

## 里程碑

| 里程碑 | 内容 | 验收标准 |
| --- | --- | --- |
| **M1** | 异步任务骨架（不接 LLM，用 `time.Sleep(2s)` 模拟耗时） | `POST /run` 返回 202 + taskID；`GET /tasks/{id}` 查到 pending→running→done；worker pool 限并发；`go test ./... -race` 通过 |
| **M2** | 接 LLM（Go 或 Python worker 调 LLM API） | 真实 agent 任务跑通；LLM 超时/失败 → task failed 状态 |
| **M3** | 工程化完善（可选） | 结构化日志、Dockerfile、README、benchmark |

---

## M1 · 异步任务骨架（当前任务）

### 需求

- `POST /run`：接收 JSON body `{"prompt": "..."}`，创建任务，返回 `202 + {"task_id": "..."}`
- `GET /tasks/{id}`：返回任务状态（pending / running / done / failed）
- worker pool：3 个 worker goroutine 从队列取任务，**用 `time.Sleep(2s)` 模拟 agent 执行**（先不接 LLM）
- 任务完成时更新状态为 done，结果可先存固定 JSON

### 核心数据结构（参考 ADR-0002）

```go
// 知识点：Task 是有生命周期的对象，Status 表示当前阶段
type Task struct {
    ID     string
    Status string // pending / running / done / failed
    Prompt string
    Result json.RawMessage
}
```

### 关联规范

- §8.3 永远不要启动无法停止的 goroutine — worker 怎么干净退出？
- §8.4 用 context 传播取消信号
- §7.2 错误只处理一次
- §5.5 main 包必须精简 — handler / store / worker 分独立包

### 学习要点（中文注释标记）

```go
// 知识点：异步 = 先记录任务立即返回，后台执行
// 知识点：chan Task 作为任务队列，缓冲解耦 API 层与 worker
// 知识点：状态机 pending → running → done/failed
// 知识点：Mutex 保护 map，避免并发写 race
```

### 验收门槛（全部通过才进 M2）

- [ ] `POST /run` 返回 202 + taskID
- [ ] `GET /tasks/{id}` 能查到状态变化（pending → running → done）
- [ ] worker pool 限制并发（同时最多 3 个在跑）
- [ ] `go test ./... -race` 通过
- [ ] 能画出架构图并解释每个组件
- [ ] GitHub 仓库 ≥ 10 个有意义提交

---

## 项目结构（目标）

```
agent-api/
├── README.md           # 本文件
├── docs/
│   └── adr/            # 架构决策记录（从 urlstatus 迁移或新建）
├── go.mod
├── main.go             # 入口：启动 worker pool + HTTP server
├── store/
│   ├── store.go        # TaskStore：map + Mutex
│   └── store_test.go
├── worker/
│   ├── pool.go         # worker pool 并发
│   └── pool_test.go
└── api/
    ├── handler.go      # POST /run + GET /tasks/{id}
    └── handler_test.go # httptest
```

---

## 怎样让教练审代码

写完后告诉教练：
1. 里程碑编号（如"M1 写完了"）
2. 代码文件路径
3. **你自己觉得哪里写得不好**

教练会引用规范条款 + 提问逼你思考，但不给修复代码。

### 卡住时的求助顺序

1. 先挣扎 20 分钟
2. 读 `docs/GO-STANDARDS.md` 对应条款
3. 读 `../urlstatus/docs/adr/0002-async-task-worker-pool-architecture.md` 和 `glossary.md`
4. 最后才问教练，带具体问题

---

*最后更新：2026-08-09*
