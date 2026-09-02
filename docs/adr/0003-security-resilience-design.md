# ADR-0003：安全红线、内存存储取舍与停机/背压设计

- **日期**：2026-09-02
- **状态**：已接受

## 背景

ADR-0001/0002 已确立「异步任务 + Worker Pool」主线架构，但未记录一组**非功能性但致命**的设计约束：
错误信息如何对外暴露、任务状态存哪里、进程如何干净退出、队列满时如何反应。
这些点恰恰是高可用与信息安全面试的高频拷问点，且每一处都经历过真实的取舍权衡。

## 决策与双镜论证

### 决策 1：错误信息脱敏（task.Error 只存粗粒度分类）

**背景**：Worker 执行 LLM 调用失败时，上游错误原文（含配额、账单、内部路径、上游响应体）会进入日志；
而 `GET /tasks/{id}` 把 `Task` 整体序列化返回给**任意调用方**。

**双镜论证**
- 🎯 用户体验层：调用方需要"知道失败了"，但**绝不需要**知道上游账单号。暴露细节只会制造焦虑且无行动价值。
- 🤖 技术/安全层：`worker/pool.go:103` 将错误重写为 `errors.New("upstream error")` 再写入 `task.Error`；
  原文仅 `slog.Error` 进服务端日志。这是**最小必要信息披露**原则。

**决策**：`task.Error` 只承载由 Store 内部硬编码的粗粒度分类（如 `"upstream error"`），
原始错误全文禁止进入任何会被序列化的字段。

---

### 决策 2：内存存储、进程重启即丢失（显式 YAGNI）

**背景**：`store.Store` 用 `map[string]Task` + `sync.Mutex`，状态只活在进程内存。

**双镜论证**
- 🎯 用户体验层：MVP 阶段用户提交任务后立刻轮询拿结果，平均生命周期 < 1 分钟，
  重启丢失的代价可接受；引入 DB 反而拖慢交付、增加运维面。
- 🤖 技术层：`store/store.go:4` 注释已明确标注此限制。零值不可用、必须 `NewStore()`
  构造（map 为 nil 会 panic）是已知约束。

**决策**：MVP 不引入持久化。代价是重启丢任务，收益是零依赖、零运维、逻辑聚焦并发与接口。
后续如需持久化，应在 ADR 中单独评估 SQLite/Redis（见决策 4 修复方向）。

---

### 决策 3：优雅停机用 context cancel，而非 close(queue)

**背景**：`Pool.Stop()` 需要同时让所有 worker 退出，并取消在飞的 LLM 请求。

**双镜论证**
- 🤖 技术层：`worker/pool.go:27-29` 用 `context.WithCancel` 做广播。`Enqueue` 向 `queue` 发送，
  若 `Stop` 用 `close(queue)`，则已在排队或即将入队的发送方会 `panic`（向已关闭 channel 发送）。
  cancel 让 `worker()` 的 `select` 同时监听 `p.queue` 与 `p.ctx.Done()`，干净退出。
- 🤖 资源层：`process()` 从 `p.ctx` 派生 60s 超时子 ctx（`pool.go:94`），Stop 时父 ctx 取消，
  在飞请求**不必干等超时**立即中断，避免 goroutine/连接悬挂。

**决策**：统一用 context 取消传播，禁止对任务队列 channel 执行 close。

---

### 决策 4：队列缓冲 = worker 数，满则阻塞 Handler（背压退化为同步）

**背景**：`queue` 缓冲大小等于 `size`（`worker/pool.go:21`）。`Enqueue` 是无 `select` 的阻塞发送。

**双镜论证**
- 🎯 用户体验层：`POST /run` 在队列满时会被挂起，用户请求转圈；但**不会**返回错误，
  调用方无感知地等待——比立刻失败更隐蔽。
- 🤖 技术/SRE 层：满队列阻塞的是 `net/http` 派生的 handler goroutine，连接被长期占用；
  更糟的是「Store 已 Create，但调用方拿不到 task_id」的僵尸任务（`HandleRun` 先 Create 再 Enqueue）。
  这是**已知限制**，非 bug。

**决策**：MVP 接受此退化行为（实现极简）。两条修复路线已记录在 `docs/interview/project-qa.md` Q3：
- 短期：`select` + `default` 非阻塞写入，满则回滚 Store 记录并返回 `503`。
- 长期：换持久化队列（SQLite/Redis），隔离内存压力且防重启丢失。

---

### 决策 5：先 Create 再 Enqueue 的顺序不变量

**背景**：`api/handler.go:61-62` 严格 `store.Create` 在前、`pool.Enqueue` 在后。

**双镜论证**
- 🤖 技术层：Enqueue 后 worker 可能**立刻**被调度执行 `Update/Get`。若 ID 尚未落库，
  worker 会拿到 `ErrNotFound` 直接跳过。先落库保证「ID 必存在」是这条链路的**前提不变量**，
  顺序颠倒会引发偶发的静默丢任务（最难复现的一类 bug）。

**决策**：Create→Enqueue 顺序不可交换，禁止在此间插入可能 panic 或返回 early 的逻辑。

## 与已学知识的映射

| 设计点 | 已学技能 |
|--------|---------|
| 错误脱敏 / 最小信息披露 | 安全编码基线 |
| 内存 map + Mutex | 1.5 Store 模式 |
| context cancel 广播 | 1.4 context 取消传播 |
| 队列缓冲与背压 | 1.1 channel 容量语义 |
| Create→Enqueue 不变量 | 并发时序正确性 |

## 后果

- ✅ 信息安全红线清晰：上游敏感信息不泄露给调用方。
- ✅ 停机干净：无 goroutine/连接泄漏，在飞请求可中断。
- ⚠️ 进程重启丢全部任务（MVP 可接受，长期需评估持久化）。
- ⚠️ 队列满时 Handler 阻塞（MVP 可接受，已规划 503 快速失败）。
