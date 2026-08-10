
# Agent API 项目说明

本项目是 Go 后端服务：**异步任务 + Worker Pool + LLM 调用的 Agent API**。
简历主线项目，目标大二下找到 Go 后端实习。

## 项目文档

- 计划与里程碑：`../docs/PLAN.md`、`../docs/MILESTONES.md`（整个学习计划的追踪）
- 架构决策：`docs/adr/`（ADR-0001/0002 在 `../urlstatus/docs/adr/`）
- 术语表：`../urlstatus/docs/glossary.md`

## Go 代码规范

**编写或审查 Go 代码时，必须先读取 `docs/GO-STANDARDS.md` 并遵守其中的 MUST/SHOULD 规则。**

该规范基于 Dave Cheney《Practical Go》，覆盖：指导原则、标识符命名、注释、包设计、项目结构、API 设计、错误处理、并发、工具格式。

人类程序员可参考精简版 `docs/GO-CHEATSHEET.md`（~50 行速查）。

## 学习约定

- 所有代码由用户亲手编写，agent 只做教练（审、提问、讲解、绝不代写）
- 代码中的知识点注释用中文（学习用途），命名先求清晰
- 卡住时：先挣扎 20 分钟 → 读 GO-STANDARDS.md → 再问 agent
