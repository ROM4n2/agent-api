# ADR-0011：任务超时可配置化与优雅停机取消树

- **日期**：2026-09-03
- **状态**：已接受

## 背景

原 `process` 写死 `context.WithTimeout(p.ctx, 60*time.Second)`（`worker/pool.go`）。两个问题：

1. **慢上游会长期占住 worker。** 如 Q5 事故：上游 P99 从 2s 飙到 30s 时，单个 worker 被占 30s+，队列迅速填满触发快速失败 503。60s 的硬上限过长，是事故被放大的因素之一。
2. **超时值不可调。** 无法按上游 SLA 常态收紧，也无法在事故期临时「止血」。

同时，优雅停机机制其实已经存在（`signal.NotifyContext` + `srv.Shutdown` + `defer p.Stop()`），但其「**取消树**」语义——父 ctx 取消使在飞 LLM 请求立即中断、该任务被标记 `failed` 而非静默丢失——此前从未写进规范。此外在重构中 `TaskStore` 接口一度丢失，导致 `ADR-0010` 的 SQLite 可插拔断链（`pool.go`/`main.go` 退回具体 `*store.Store`，`db_path` 分支消失）。

## 决策

1. **任务超时改为可配置。**
   - 新增 `config.TaskTimeoutSeconds`（`config.yaml` 的 `task_timeout_seconds`，缺省 30s），经 `NewPool(size, taskTimeout, s, client)` 注入；单任务 `process` 改用 `p.taskTimeout`。
   - **默认 30s**：比原 60s 短，覆盖绝大多数正常 LLM 调用，又不至于误杀偶发长响应。
   - **事故期止血旋钮：降到 8s**（triage 首选动作，先于客户端退避与 LLM 重试）。这与 `client.go` 里 `http.Client{Timeout: 60s}` 的传输层绝对上限**正交**：正常态由 `taskCtx` 先打断请求，60s 仅作兜底。

2. **优雅停机取消树作为既定机制记录。**
   - 停机 `p.cancel()` → 派生自 `p.ctx` 的 `taskCtx` 立即失效 → `ChatWithTools` 用 `http.NewRequestWithContext(ctx, ...)` 建的在飞请求被中断 → `process` 收到 error → `store.Complete(id, "", err)` 将该任务标记为 **failed**（终态入 store，轮询客户端可见），**不是**悄悄丢弃。
   - `main.shutdown` 里 `srv.Shutdown(30s)` 只约束 HTTP server 的停机宽限期（停止接新连接、等存量请求结束），与单任务 30s/8s 超时**正交**：停机时 `cancel` 盖过任务超时，在飞任务不拖过停机窗口。

3. **回归 `TaskStore` 接口。** 在 `store` 包补回 `TaskStore` 接口（`Create/Update/Get/Complete`），`pool.go`/`handler.go`/`main.go` 均依赖接口；`main` 按 `conf.DbPath` 在内存 `Store` 与 `SQLStore` 间切换，恢复 `ADR-0010` 的 SQLite 可插拔。

## 候选方案（超时）

- **方案 A（采用）**：配置可配 + 默认 30s，事故期降到 8s。
- **方案 B**：仅把硬编码 60s 改成 8s，不留旋钮。缺点：正常态偶发长响应会被误杀，且无法按环境微调。
- **方案 C**：超时只交给 `http.Client.Timeout`，不动 ctx。缺点：无法在停机时随取消树一起中断，且错误分类不如 ctx 清晰（cancel 与超时错误混在一起）。

## 与已学知识的映射

| 设计点 | 已学技能 |
|--------|---------|
| 单任务上限 + 取消树 | Q7 context 取消树、Q5 503 排障链条 |
| 短超时优先于重试 | triage 优先级（A 短超时 > B 客户端退避 > D 事故期绝不重试）|
| 接口抽象使后端可插拔 | ADR-0010、Store 模式 |
| 配置化 vs 硬编码魔法数字 | ADR-0006 |

## 后果

- ✅ 慢上游不再长期占住 worker；事故期可一键把超时收紧到 8s 止血，缩短队列堆积窗口。
- ✅ 优雅停机语义明确：在飞任务标记 `failed` 而非丢失，轮询客户端总能看到终态。
- ✅ SQLite 持久化恢复可插拔，进程重启不丢任务（与 ADR-0010 一致）。
- ⚠️ 默认 30s 偏保守；若上游常态慢于 30s 需调大——这正是「可配置」存在的意义。
- ⚠️ 改 `NewPool` 签名需同步 8 处调用点（含 `pool_test` / `pool_bench_test` / `api` 测试 / `main`），已一并更新并通过 `go test ./...`。
