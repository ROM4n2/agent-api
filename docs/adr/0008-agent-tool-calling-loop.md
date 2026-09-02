# ADR-0008：Agent 工具调用循环（消除命名错位）

- **日期**：2026-09-02
- **状态**：已接受

## 背景

ADR-0007 指出最大简历风险：项目叫 `agent-api`，但原实现只是「LLM 聊天的异步封装」，
**没有真正的 agent 行为**（无工具调用 / 多步规划）。用户拍板：**加真 agent 工具调用循环**，
使 "agent" 名副其实，而非改名。

## 决策

在 worker 中跑 **think → call tool → observe** 的多步循环：

1. `llm.Client` 新增 `ChatWithTools(ctx, messages, tools)`，向 DeepSeek 发送带工具定义的对话，
   返回 `AssistantTurn{Content, ToolCalls}`（OpenAI 兼容 function calling）。
2. worker 的 `runAgent` 循环：调用模型 → 若无 `tool_calls` 则返回终态文本；
   否则把 assistant 意图与每个工具的 `role=tool` 结果回填对话，再次调用，直到收尾或达 `maxAgentSteps=5`。
3. 内置两个真实工具：`calculate`（安全四则运算，递归下降求值，**不用 eval**）、`current_time`（IANA 时区）。
4. 工具以「描述（给模型）+ 本地执行器」成对注册，经函数名关联；执行错误也转字符串返回，让模型自我纠正。

## 双镜论证

- 🎯 产品/简历层：名字与实质一致，面试官问"agent 在哪"可现场演示工具调用；
  多步循环是 agent 的最小可信定义，远胜"聊天代理"。
- 🤖 技术层：循环上限 `maxAgentSteps` 防无限工具环；上下文取消（停机/超时）透传每次 LLM 调用；
  工具求值用自研解析器杜绝代码注入；DI 保持——`chatter` 接口仅增方法，mock 仍可无网测试。
- 🪜 Ladder of Reuse：工具类型直接复用 `llm` 包已有的 OpenAI 兼容 wire 结构，未引入新依赖。

## 与已有设计的衔接

- 状态机、错误脱敏（ADR-0003）、worker pool（ADR-0002）**完全不变**——agent 循环只是
  `process` 内部把单次 `Chat` 换成 `runAgent`，对外契约（202 + 轮询）不受影响。
- `chatter` 接口方法由 `Chat` 升级为 `ChatWithTools`；`llm.Client.Chat` 保留供单测，
  现有 17 单测 + 新增 agent 测试全绿。

## 后果

- ✅ "agent-api" 名副其实，简历塌房风险消除。
- ✅ 零新依赖、零外部服务，演示自包含（计算器/时间无需联网）。
- ✅ 安全：工具求值无 eval；错误脱敏仍生效（终态只存结果/粗粒度错误）。
- ⚠️ 工具集较小（2 个），按需要可扩展 `defaultTools()`。
- ⚠️ 多步循环增加 LLM 调用次数 → 成本略升，受 `maxAgentSteps` 与 ADR-0002 并发上限共同约束。
