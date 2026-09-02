# P1 可观测 + Demo + 测试增强 Implementation Plan

> **Goal**: 给 agent-api 增加可观测端点（`/healthz`、`/metrics`）、一个零依赖单页 Demo，以及 httptest 集成测试 + pool benchmark。
> **Tech Stack**: Go 1.26 (stdlib only)
> **Spec Reference**: `docs/adr/0007-resume-positioning-and-optimization.md` (P1 项)
> **Global Constraints**:
> - 零新依赖（守住 ADR-0004 YAGNI）：指标用 stdlib `sync/atomic` 自实现，Prometheus 文本格式手写，不引 `prometheus/client_golang`。
> - `/healthz`、`/metrics` 只暴露聚合计数/存活，不泄露任务内容（store 文本不出现在指标里）。
> - 全链路 UTF-8 输出；guard clause 扁平（≤2 层）。
> - 不破坏现有单测（`pool_test.go` / `handler_test.go` 必须保持绿）。

---

## Task 1: metrics 计数器包 [Role: TDD Builder]

**Files:**
- Create: `metrics/metrics.go`
- Test: `metrics/metrics_test.go`
- Modify: `api/handler.go:74-75`（HandleRun 落库后 `metrics.IncSubmitted(); metrics.IncRunning()`）
- Modify: `worker/pool.go:87,116,110`（process 内 `IncRunning` 已在提交时计数，此处 `IncDone`/`IncFailed`）

**Interfaces:**
- Consumes: `store.Store`（仅在调用点），无新接口
- Produces:
  - `func IncSubmitted()`, `func IncRunning()`, `func IncDone()`, `func IncFailed()`
  - `func Handler() http.Handler` —— 输出 Prometheus 文本格式

**Subagent Prompt Scaffold (for /vault-exec):**
> "Implement Task 1: metrics 计数器包。
> Goal: 用 sync/atomic 实现进程内计数器（submitted/running/done/failed）并导出 /metrics 处理器。
> Target Files: Create `metrics/metrics.go`, Test `metrics/metrics_test.go`。
> TDD Steps:
> 1. 写 `metrics_test.go`：构造调用 Inc* 后用 Handler().ServeHTTP 抓响应，断言含 `agent_tasks_submitted N`。
> 2. 运行 `go test ./metrics/` 确认 RED。
> 3. 实现 `metrics.go`（atomic.Int64 + Handler 拼 Prometheus 文本）。
> 4. 运行 `go test ./metrics/` 确认 GREEN。
> 5. 接线 api/handler.go 与 worker/pool.go 的计数点。
> Return: 测试通过证据 + diff 摘要。"

**Step Breakdown:**
- [ ] Step 1: 写失败测试（RED）
- [ ] Step 2: 运行测试确认失败
- [ ] Step 3: 实现 `metrics.go`（GREEN）
- [ ] Step 4: 运行测试确认全绿
- [ ] Step 5: 接线 HandleRun / process 计数点
- [ ] Step 6: Git atomic commit `feat: add metrics package with atomic counters`

---

## Task 2: `/healthz` 与 `/metrics` 路由 [Role: TDD Builder]

**Files:**
- Modify: `api/handler.go:34-49`（`Routes()` 增加 `GET /healthz`、`GET /metrics`）
- Test: `api/handler_test.go`（新增 `TestHealthz`、`TestMetrics`）

**Interfaces:**
- Consumes: `metrics.Handler()`
- Produces: 无新类型，仅路由注册

**设计要点**: `/healthz` 与 `/metrics` 都只包 `Recover(RequestID(...))`，不走 `Limit`/`Auth`，保证 LB 探针与指标抓取可达。`/metrics` 只暴露聚合计数，无敏感数据。

**Subagent Prompt Scaffold (for /vault-exec):**
> "Implement Task 2: 注册 /healthz 与 /metrics。
> Goal: 在 Routes() 增加 GET /healthz（返回 200 ok）与 GET /metrics（转发 metrics.Handler()），两者仅用 Recover+RequestID 包裹，不经过 Limit/Auth。
> Target Files: Modify `api/handler.go`, Test `api/handler_test.go`。
> TDD Steps:
> 1. 在 handler_test.go 写 TestHealthz（httptest 请求 /healthz 期望 200）、TestMetrics（期望 200 且 body 含 agent_tasks_）。
> 2. 运行确认 RED。
> 3. 在 Routes() 注册两个路由。
> 4. 运行确认 GREEN。
> Return: 测试通过证据。"

**Step Breakdown:**
- [ ] Step 1: 写失败测试（RED）
- [ ] Step 2: 运行确认失败
- [ ] Step 3: 注册路由（GREEN）
- [ ] Step 4: 运行确认全绿
- [ ] Step 5: Git atomic commit `feat: add /healthz and /metrics endpoints`

---

## Task 3: 单页 Demo（`GET /`）[Role: TDD Builder]

**Files:**
- Create: `api/demo.html`（零依赖单页：prompt 输入 + 提交 + 轮询展示状态/结果 + 可选 token 输入框）
- Create: `api/demo.go`（`//go:embed demo.html` + `serveDemo` handler）
- Modify: `api/handler.go:34-49`（`Routes()` 增加 `GET /` → `serveDemo`，包 `Recover(RequestID(...))`）
- Test: `api/handler_test.go`（新增 `TestDemoPage` 期望 200 + text/html）

**设计要点**: 纯 HTML+内联 CSS+原生 JS，无 CDN、可离线打开。提交时带 `Authorization: Bearer <token>`（token 框留空则开发模式直发）。轮询用现有 `GET /tasks/{id}`。

**Subagent Prompt Scaffold (for /vault-exec):**
> "Implement Task 3: 单页 Demo。
> Goal: 用 go:embed 把 demo.html 挂到 GET /，页面可提交 prompt 并轮询展示结果。
> Target Files: Create `api/demo.html`, `api/demo.go`, Modify `api/handler.go`, Test `api/handler_test.go`。
> TDD Steps:
> 1. 写 TestDemoPage 期望 GET / 返回 200 且 Content-Type 含 text/html。
> 2. 运行确认 RED。
> 3. 写 demo.html（美观、离线可用）+ demo.go（embed）+ 注册路由。
> 4. 运行确认 GREEN。
> Return: 测试通过证据 + 页面截图说明。"

**Step Breakdown:**
- [ ] Step 1: 写失败测试（RED）
- [ ] Step 2: 运行确认失败
- [ ] Step 3: 实现 demo.html / demo.go / 路由（GREEN）
- [ ] Step 4: 运行确认全绿
- [ ] Step 5: Git atomic commit `feat: add single-page demo at /`

---

## Task 4: httptest 全链路集成测试 [Role: TDD Builder]

**Files:**
- Create: `api/integration_test.go`（`package api_test`）

**Interfaces:**
- Consumes: `api.NewHandler`, `worker.NewPool`, `store.NewStore`, `llm.Message`/`llm.AssistantTurn`
- Produces: 无新类型

**设计要点**: 用本地 fake chatter（实现 `ChatWithTools`，直接返回终态文本、无 tool_calls）装配 store+pool+mux，启动 pool，POST `/run` → 拿 task_id → 轮询 `GET /tasks/{id}` → 断言最终 `done` 且 result 非空；并断言 `GET /metrics` 里 `agent_tasks_submitted` ≥ 1。验证 handler+store+pool+中间件 真实联动，无网络。

**Subagent Prompt Scaffold (for /vault-exec):**
> "Implement Task 4: 全链路集成测试。
> Goal: 在 api_test 包用 fake chatter 装配真实 mux，跑通 run→poll 全流程并断言 done。
> Target Files: Create `api/integration_test.go`。
> TDD Steps:
> 1. 写测试：构造 fakeChatter + Pool + Handler + mux，POST /run，轮询至 done，断言 result。
> 2. 运行确认 RED（此时无此测试/或接口未对齐）。
> 3. 必要时微调（如暴露所需导出），使测试通过。
> 4. 运行确认 GREEN。
> Return: 测试通过证据。"

**Step Breakdown:**
- [ ] Step 1: 写集成测试（RED）
- [ ] Step 2: 运行确认失败/对齐
- [ ] Step 3: 使其通过（GREEN）
- [ ] Step 4: 运行确认全绿
- [ ] Step 5: Git atomic commit `test: add httptest integration test for run→poll flow`

---

## Task 5: Pool Benchmark [Role: Perf]

**Files:**
- Create: `worker/pool_bench_test.go`

**Interfaces:**
- Consumes: `worker.NewPool`, `store.NewStore`
- Produces: 无新类型

**设计要点**: `BenchmarkPoolEnqueue` 用 no-op fake chatter（立即返回 done），`b.ResetTimer` 后批量 Enqueue 并等 wg 完成；另加 `BenchmarkRunAgent` 直接测 agent 循环（无工具调用的单轮）。只测调度/循环开销，不测真实 LLM。

**Subagent Prompt Scaffold (for /vault-exec):**
> "Implement Task 5: Pool Benchmark。
> Goal: 用 fake chatter 给 Pool 加基准测试，量化调度开销。
> Target Files: Create `worker/pool_bench_test.go`。
> Steps:
> 1. 写 BenchmarkPoolEnqueue（no-op chatter，批量 enqueue + 等完成）。
> 2. 运行 `go test -bench=. -benchmem ./worker/` 确认可跑。
> Return: bench 输出证据。"

**Step Breakdown:**
- [ ] Step 1: 写 benchmark
- [ ] Step 2: 运行确认可跑
- [ ] Step 3: Git atomic commit `perf: add pool enqueue benchmark`

---

## 依赖与执行顺序
Task1 → Task2 → Task3 → Task4 → Task5（线性，后项依赖前项导出的 `metrics` 包与路由形态）。

## 验收（Definition of Done）
- `go build ./...` 通过；`go test ./...` 全绿（含单测、集成、bench）。
- 启动后 `curl /healthz` 返回 200；`curl /metrics` 返回 Prometheus 文本含 4 个 `agent_tasks_*` 指标。
- 浏览器打开 `/` 可提交 prompt 并看到轮询结果与状态演进。
- 全程零新第三方依赖。
