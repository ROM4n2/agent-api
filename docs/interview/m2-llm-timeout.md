# M2 面试问答：LLM 集成、超时与错误状态

> 来源：2026-08-11 M2 编码期间的教练问答，随进度追加。
> 关联：[M1 异步任务 + Worker Pool](m1-async-worker-pool.md)、[ADR-0002](../adr/0002-async-task-worker-pool-architecture.md)

## 使用方法

只看 **Q**，合上文档**出声**作答，再对照「标准答法」比对漏掉的关键词。「常见追问」才是区分度所在。

---

## Q1 【🔥 高频】不给 `http.Client` 设 `Timeout` 会怎样？

**考点**：能否把一个「少写一行」的疏忽，推演成全站宕机。这是最能体现生产经验的题。

**标准答法**——完整的故障链条：

> `http.Client` 的 `Timeout` 字段零值是 `0`，含义是**永不超时**（`http.DefaultClient`、`http.Get/Post` 都是这个默认值）。故障链条：
>
> 1. 上游 LLM 服务异常——**注意不是断开，是沉默**：TCP 连接建立成功，但迟迟不返回响应体
> 2. worker goroutine 阻塞在 `c.http.Do(req)` 这一行
> 3. 没有 Timeout → **这行永远不返回**。该 worker 事实上已死，但从外部看它「还在 running」
> 4. 3 个 worker 陆续遇到同类请求 → **全部僵死**
> 5. 队列迅速塞满 → `Enqueue` 阻塞 → HTTP handler 阻塞 → **整个服务无响应**
>
> 一次上游超时，滚成全站宕机。根因只是少了一行 `Timeout: 60 * time.Second`。
>
> 关键点：**TCP 层不会帮你**。连接没断，操作系统认为一切正常，只是没有数据到来。必须由应用层设期限。

**关键词**（漏一个就掉档）：
- 零值 = 永不超时，`http.DefaultClient` 同样没有超时
- 「沉默」比「断开」更危险——断开会立刻返回 error，沉默是无限等待
- **goroutine 泄漏**：僵死的 worker 不会被回收，看着活着实际已死
- 故障**放大**：单点超时 → 池耗尽 → 队列满 → handler 阻塞 → 全站不可用

**常见追问**：

- *`Timeout` 具体管到哪一段？* → 从**拨号开始**到**响应体读完**的全过程（含连接、TLS 握手、写请求、读响应体）。它不管 body 读完之后你自己的处理逻辑。
- *需要更细粒度怎么办？* → 用 `http.Transport` 分段设置：`DialContext` 的连接超时、`TLSHandshakeTimeout`、`ResponseHeaderTimeout`、`IdleConnTimeout`。`Client.Timeout` 是粗粒度兜底。
- ***既然有 `Timeout`，为什么还要传 `ctx`？**（必考）* → 见下题。

---

## Q2 【🔥 必考】`http.Client.Timeout` 和 `context` 超时有什么区别？两个都要吗？

**考点**：能否分清「**兜底策略**」和「**调用方控制权**」。只知其一说明没写过真实服务。

**标准答法**：

| | `http.Client.Timeout` | `context.WithTimeout` / `WithCancel` |
|---|---|---|
| 作用域 | 该 client 发出的**所有**请求 | **单次**调用 |
| 谁决定 | 构造 client 时写死（库作者） | 每次调用时传入（调用方） |
| 能否主动中断 | ❌ 只能等时间到 | ✅ **可随时 `cancel()`** |
| 定位 | **安全网**，防遗漏 | **控制权**，按需收紧 |

> 两者取**更早到期**的那个生效，所以不冲突，**都要**。
>
> `ctx` 强在两点：
> 1. **按调用收紧**——「这个任务只给 20 秒」，client 层的 60s 兜底不变
> 2. **主动取消**——服务收到关闭信号时，把 M1 里 pool 那个 `p.ctx` 传下来，**所有在飞的 LLM 请求立即中断**，不必干等 60 秒。这是 graceful shutdown 能快速收尾的前提。

**代码形状**：

```go
// 库内部：安全网，写死
http: &http.Client{Timeout: 60 * time.Second}

// 调用方：控制权，每次可变
req, _ := http.NewRequestWithContext(ctx, ...)   // ← 必须用带 ctx 的构造函数
```

**常见追问**：

- *用 `http.NewRequest` 而不是 `NewRequestWithContext` 会怎样？* → `ctx` **完全失效**，传了也白传。取消信号进不到请求里，只剩 client 的 60s 兜底。这是个静默失效的坑——编译通过、功能正常，只在需要取消时才发现没用。
- *`ctx` 超时后 `Do()` 返回什么错误？* → 包装了 `context.DeadlineExceeded` 的 `*url.Error`。判断要用 `errors.Is(err, context.DeadlineExceeded)`，不能比较字符串。
- *为什么 `ctx` 必须是函数第一个参数？* → Go 社区强约定（`ctx context.Context` 作首参），便于工具检查和链路一致性传递。

---

## Q3 API key 怎么管理？

**考点**：机密不进代码库。这题答错在简历项目里是**一票否决**。

**标准答法**：

> 从**环境变量**读：`os.Getenv("DEEPSEEK_API_KEY")`。三个理由：
> 1. key 不在代码里 → **不进 git** → 无泄漏风险
> 2. 本地/测试/生产用不同 key，**代码一行不改**
> 3. 12-Factor App 的 Config 原则：配置随环境变化，代码不随环境变化
>
> 且必须**启动时校验**：`os.Getenv` 取不到时返回**空字符串而非报错**，会带着空 `Authorization` 头发请求，收到一个莫名的 401。所以在 `main` 里检查为空就**立刻退出（fail fast）**，别让错误延迟到第一个请求才暴露。

**红线**：
- ❌ `const APIKey = "sk-xxx"` 写进代码
- ❌ `.env` 文件提交进 git（`.env` 必须进 `.gitignore`）
- ❌ key 出现在日志里（打日志时要脱敏）

**常见追问**：
- *生产环境呢？* → K8s Secret / 云厂商的密钥管理服务（KMS、Secrets Manager），配合定期轮转。环境变量是最小可用方案。
- *为什么在 `main` 校验而不在 `NewClient` 里？* → `NewClient` 是库代码，**库不该决定进程生死**。校验和退出是 `main` 的职责。

---

## Q4 构造函数参数多了怎么设计？

**考点**：API 设计品味。

**标准答法**：

| 情况 | 做法 |
|---|---|
| ≤3 个参数，**类型各不相同** | 直接列参数 |
| 多个**同类型**参数，都必填 | **config struct** |
| 有**可选**参数、需要默认值 | **functional options** |

> 核心风险是**多个同类型参数没有编译期保护**：
>
> ```go
> NewClient(key, "deepseek-chat", "https://api.deepseek.com")  // 顺序传反，编译通过，运行时才炸
> ```
>
> config struct 用**字段名**消除这个风险，也让未来加字段不破坏调用方：
>
> ```go
> llm.NewClient(llm.Config{APIKey: k, BaseURL: url, Model: m})
> ```
>
> functional options 适合「必填留签名、可选走 opts」：
>
> ```go
> func NewClient(apiKey string, opts ...Option) *Client
> llm.NewClient(key)                                 // 全默认
> llm.NewClient(key, llm.WithModel("reasoner"))      // 只改一项
> ```
> gRPC、zap 等主流库都是这个模式。

**常见追问**：
- *为什么不用可变参数或 map？* → 丢失类型安全和编译期检查，IDE 也无法补全。
- *Go 为什么没有默认参数/函数重载？* → 语言刻意不支持（简洁性权衡），functional options 就是社区对此的标准应答。

---

## Q5 为什么手写 HTTP 客户端而不用官方 SDK？

**考点**：技术选型能否讲出理由，而非跟风。

**标准答法**：

> 这个接口就是「一个 POST，JSON 进 JSON 出」，`net/http` + `encoding/json` 足够，三个理由：
> 1. **零依赖**——`go.mod` 干净，无供应链风险，无版本升级负担
> 2. **不锁 provider**——OpenAI SDK 认的是 OpenAI；换成 DeepSeek / 智谱 / 火山等兼容端点，自己拼请求只需改一个环境变量
> 3. **可控**——超时、重试、429 处理、错误分类全在自己手里，不必研究 SDK 怎么配

**反向也要能答**（面试官会问「什么时候该用 SDK」）：
> 需要 streaming、tool calling、多模态、自动重试与限流退避这些复杂能力时，SDK 的价值就超过依赖成本了。**判断标准是接口复杂度**：单个 JSON 端点自己写，一整套协议用 SDK。

---

## 自检清单

- [ ] `Timeout` 零值含义，以及从单点超时到全站宕机的**完整 5 步链条**
- [ ] 「沉默」比「断开」危险的原因（TCP 层不报错）
- [ ] `Client.Timeout` vs `ctx`：作用域 / 谁决定 / 能否主动取消 / 定位
- [ ] `NewRequestWithContext` 与 `NewRequest` 的区别，以及后者导致 ctx 静默失效
- [ ] key 走环境变量的 3 个理由 + 为什么要 fail fast + 为什么校验放 `main`
- [ ] 构造函数三种参数风格的选用标准
- [ ] 手写 client 的 3 个理由，以及何时该反过来用 SDK
