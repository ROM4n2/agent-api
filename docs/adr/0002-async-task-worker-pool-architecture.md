# ADR-0002：异步任务 + Worker Pool 架构

- **日期**：2026-08-09
- **状态**：已接受

## 背景

Agent API 服务需要执行可能耗时数十秒的 agent 任务。若采用同步执行，存在
连接超时、服务器资源被打满、客户端被卡住三大问题。

## 决策

采用**异步任务 + worker pool** 架构：

1. **API 层**：接收请求，先记录任务（生成唯一 ID），立即返回 `202 + taskID`。
2. **任务队列**：任务经 `chan Task` 进入队列。
3. **Worker Pool**：固定数量（如 3 个）worker goroutine 从队列取任务执行，
   控制并发上限，防止服务器与外部 LLM API 被打满。
4. **Task Store**：`map[string]Task` + Mutex 存任务状态，
   状态机 `pending → running → done/failed`。
5. **客户端轮询**：`GET /tasks/{id}` 查询状态。

## 关键设计点

- **创建任务与执行任务解耦**：API 层只负责 `store.Create(prompt)` 返回 ID，
  执行交给后台 worker，两者通过 Store 连接。
- **状态机**：Task 是有生命周期的对象，Status 字段表示当前阶段。
- **Result 用 `json.RawMessage`**：agent 结果形状未知，先原样存 JSON。

## 与已学知识的映射

| 架构组件 | 已学技能 |
|---------|---------|
| taskQueue `chan Task` | 1.1 channel |
| TaskStore `map + Mutex` | 1.5 Store 模式 |
| worker pool goroutine | 1.1/1.4 并发 |
| handler + httptest | 1.5 |
| errgroup（简化形式） | 1.4 |
