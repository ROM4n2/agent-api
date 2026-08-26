# Go 语言并发与网络八股背诵手册

本文件为 Go 并发核心八股背诵手册，结合项目实际，提炼最常考的高频知识点。

---

## 1. net/http 并发模型与 Goroutine 泄露

### Q：Go 语言的 `net/http` 服务是如何处理并发请求的？
> **核心机制**：**一连接一协程（One Goroutine per Connection/Request）**。
> **处理流程**：
> 1. `http.ListenAndServe` 底层在 `Accept()` 循环中等待 TCP 连接。
> 2. 每当接受一个新的 TCP 连接，底层便会启动一个独立的 Goroutine：`go c.serve(connCtx)` 去处理该连接。
> 3. 该 Goroutine 负责读取 HTTP 请求、匹配路由并执行 Handler。
> **弊端与风险（Goroutine 泄露）**：
> 如果 Handler 中存在阻塞操作（如本项目中无超时的 HTTP 发送、无超时的 Channel 写入），会导致这个临时处理协程永远无法退出。当海量请求涌入时，会产生几十万个僵死协程，造成内存耗尽崩溃（OOM）。

---

## 2. Context（上下文）设计与传播

### Q：Context 的作用是什么？它的底层数据结构是怎样的？
> **核心作用**：**在 Goroutine 树状链路中传递截止时间、取消信号和键值对数据**。
> **底层原理**：
> Context 本质是一个接口，包含四个核心方法：
> 1. `Deadline()`：返回截止时间。
> 2. `Done()`：返回一个只读 Channel，用于接收取消信号。
> 3. `Err()`：返回被取消的原因（如超时或主动取消）。
> 4. `Value()`：获取传递的数据。
> **层级取消**：
> 1. 采用 **Tree 树状节点组织**。父节点被取消或超时，会级联触发关闭所有子节点的 `Done()` Channel。
> 2. 常用的子类有：`cancelCtx`（带主动取消）、`timerCtx`（带定时超时）、`valueCtx`（带只读 K-V 传参）。

---

## 3. 并发安全与锁机制 (Mutex vs RWMutex vs sync.Map)

### Q：Mutex、RWMutex 与 sync.Map 三者如何抉择？
> **1. `sync.Mutex`（互斥锁）**：
> *   **机制**：完全互斥。不论读写，同一时刻仅允许一个 Goroutine 持有。
> *   **场景**：临界区极小、执行极快、且读写比不悬殊的场景（如本项目的 `Store` 读写混合操作）。
>
> **2. `sync.RWMutex`（读写锁）**：
> *   **机制**：读读并行（共享锁），读写互斥，写写互斥（排他锁）。
> *   **场景**：**读远多于写（如 90% 以上是只读）** 的高并发缓存场景。如果写操作多，内部维护锁状态的开销反而导致其性能弱于 `Mutex`。
>
> **3. `sync.Map`**：
> *   **机制**：开箱即用的并发安全 Map。
> *   **场景**：仅适用于**“读极多、写极少，且 Key 集合极稳定”**的特定硬件缓存场景。其内部利用只读和脏 Map 转移实现无锁读取。若写入频繁，会导致严重的锁穿透与数据拷贝开销，性能极差。

---

## 4. Go 调度器 GMP 模型

### Q：请讲讲 Go 并发基石 GMP 模型。
> **三个实体**：
> *   **G (Goroutine)**：轻量级协程，拥有独立的栈空间（初始 2KB）和程序计数器。
> *   **M (Machine)**：物理线程，由 OS 内核调度，代表真实的 CPU 执行力。
> *   **P (Processor)**：逻辑处理器，包含本地的可运行 G 队列。**M 必须绑定一个 P 才能执行 G**。P 的数量默认为物理 CPU 核心数（通过 `GOMAXPROCS` 控制）。
>
> **两个调度核心策略**：
> 1.  **Work Stealing（工作窃取）**：
>     当某个 P 的本地 G 队列空闲时，它会尝试从全局队列获取 G，若全局队列也为空，它会去其他 P 的队列尾部随机**“偷取”**一半的 G 来运行，极大提高多核利用率。
> 2.  **Hand Off（移交/抢占）**：
>     当一个 G 发起了阻塞的系统调用（如网络 I/O、磁盘读写）导致物理线程 M 挂起时，M 会主动释放持有的逻辑处理器 P，由其他空闲（或新创建）的物理线程 M 接手 P 并在其上继续调度执行其他协程 G，防止 CPU 闲置。

---

## 5. Channel 的底层结构

### Q：Channel 的底层是怎么实现的？为什么它是线程安全的？
> **底层结构**：
> Channel 的底层是一个叫做 `hchan` 的结构体，其核心包含：
> 1.  **`buf`**：循环队列（数组），用于存储缓冲通道的数据。
> 2.  **`lock`**：`sync.mutex` 互斥锁。**通道的每一次发送与接收操作，底层都必须对 `hchan` 加锁，从而保证线程安全**。
> 3.  **`recvq` 与 `sendq`**：双向链表队列。用于挂起和等待接收/发送的 Goroutine 节点（`sudog` 结构）。
>
> **Select 机制的非阻塞原理**：
> 当使用 `select` 配备 `default` 时，Go 编译器会将通道操作翻译为 `selectnb` 快速调用。如果通道已满/已空，它会直接跳过阻塞队列挂起流程，通过对锁的快速尝试判断，直接执行 `default` 分支，从而实现非阻塞逻辑。

---

## 6. GC：三色标记与 STW

### Q：Go 的垃圾回收是怎么工作的？

> **一句话**：**并发三色标记清除（concurrent mark & sweep），非分代、非压缩**。GC 和用户代码同时跑，只在两个极短的点上暂停全世界（STW）。
>
> **三种颜色**（颜色不是对象里的字段，是"在哪个集合里"）：
> *   **白**：还没被扫描到。标记结束时仍是白 = 垃圾。
> *   **灰**：已经知道它活着，但它引用的对象还没检查。**灰色集合就是待办队列**。
> *   **黑**：它和它引用的对象都检查完了，不用再看。
>
> **标记流程**：
> 1. 开始时所有对象是白色。
> 2. 从 **GC roots**（各 goroutine 的栈、全局变量、寄存器）出发，把直接可达的对象染灰，放进待办队列。
> 3. 循环：从队列取一个灰对象，把它引用的所有白对象染灰入队，然后把自己染黑。
> 4. 队列空 = 标记结束。剩下的白色对象就是不可达的，交给 sweep 回收。
>
> **关键难点：并发标记会漏标**。
> 因为用户代码在标记期间还在改指针。最坏情况：一个白对象唯一的引用者从灰对象移到了**已经变黑**的对象上（黑对象不会再被扫描），这个白对象就被漏掉，还活着却被当垃圾回收了。
>
> **解法：写屏障（write barrier）**。
> 标记期间，任何指针赋值都要走一段编译器插入的额外代码，把涉及的对象染灰重新入队。Go 1.8 起用**混合写屏障**（Dijkstra 插入屏障 + Yuasa 删除屏障）：
> *   插入屏障：新写进去的指针指向的对象染灰。
> *   删除屏障：被覆盖掉的那个旧指针指向的对象也染灰。
> *   两者结合的收益是：**可以把栈上对象直接当成黑色，不需要在标记结束时 STW 重扫所有栈**。这一步是 Go 把 STW 从百毫秒级压到亚毫秒级的主要原因。
>
> **两次 STW 分别在哪**：
> 1. **Mark Setup**：开启写屏障、统计 root 集合。
> 2. **Mark Termination**：关闭写屏障、收尾统计。
>
> 两次都是几十到几百微秒级。**Sweep（清扫）不 STW 也不集中做**，是惰性的：下次分配内存时顺手扫一块。
>
> **什么时候触发**：
> *   `GOGC`（默认 100）：**当前存活堆增长到上次 GC 后存活堆的 2 倍时触发**。GOGC=200 就是 3 倍，牺牲内存换更少的 GC 次数。
> *   `GOMEMLIMIT`（Go 1.19+）：软内存上限，逼近时提高 GC 频率。容器里比调 GOGC 更好用。
> *   兜底：2 分钟没触发过就强制来一次。
>
> **追问：为什么 Go 不做分代 GC？**
> 分代 GC 的前提是"大部分对象很快就死"，靠只扫新生代省事。但 Go 有**逃逸分析**：不逃逸的短命对象直接分配在栈上，函数返回就没了，根本不进堆、不需要 GC 管。分代要收割的那批对象，Go 已经在编译期解决掉了；再引入分代只会增加写屏障成本和实现复杂度。同理不做压缩（compaction）是因为要移动对象就得停下来改所有指针，与"低延迟"目标冲突。

---

## 7. defer 的执行顺序与陷阱

### Q：defer 什么时候执行？有哪些坑？

> **执行时机**：所在**函数**即将返回时（return 赋值之后、真正回到调用方之前）。注意是函数级，不是块级——`if`/`for` 的花括号结束不会触发 defer。
>
> **执行顺序**：**LIFO，后进先出**，像一个栈。
>
> ```go
> for i := 0; i < 3; i++ {
>     defer fmt.Println(i)
> }
> // 输出 2 1 0
> ```
>
> **底层实现（问到就是加分项）**：
> *   Go 1.12 及以前：defer 记录堆分配 + 链表，慢。
> *   Go 1.13：改为栈上分配，快约 30%。
> *   Go 1.14：**open-coded defer（开放编码）**，编译器直接把 defer 的调用内联到函数所有返回路径的末尾，开销接近普通函数调用。条件是：函数内 defer 数量 ≤ 8 且**不在循环里**。所以循环里写 defer 除了泄露风险，还会掉回慢路径。
>
> ### 陷阱一：参数在 defer 那一行就求值了
>
> ```go
> func f() {
>     i := 0
>     defer fmt.Println(i) // 输出 0，不是 1
>     i++
> }
> ```
> `defer` 语句执行时**立即计算参数并拷贝保存**，只是把调用推迟了。想拿到最新值就包一层闭包：`defer func() { fmt.Println(i) }()`。
>
> ### 陷阱二：defer 可以修改命名返回值
>
> ```go
> func f() (result int) {
>     defer func() { result++ }()
>     return 1 // 实际返回 2
> }
> ```
> `return 1` 分两步：先 `result = 1`，再执行 defer，最后返回 `result`。**匿名返回值没有这个能力**——它没有变量名可供 defer 修改。这也是 `defer` + `recover` 能把 panic 转成 error 返回的原理。
>
> ### 陷阱三：循环里 defer，资源攒到函数结束才释放
>
> ```go
> for _, name := range files {
>     f, _ := os.Open(name)
>     defer f.Close() // 一万个文件就同时开一万个句柄
> }
> ```
> 修法是把循环体抽成一个函数，让 defer 随该函数返回而执行。
>
> ### 陷阱四：`os.Exit` 和 `log.Fatal` 会跳过所有 defer —— 本项目的活例子
>
> `main.go` 里：
> ```go
> p.Start()
> defer p.Stop()
> // ...
> err := http.ListenAndServe(":8080", mux)
> if err != nil {
>     log.Fatal(err)
> }
> ```
> **这个 `defer p.Stop()` 永远不会执行**。`http.ListenAndServe` 只在出错时返回（正常运行时永不返回），而出错就走 `log.Fatal` —— `log.Fatal` 内部调用 `os.Exit(1)`，**`os.Exit` 直接终止进程，不 unwind 栈、不跑任何 defer**。
>
> 面试怎么讲：这是我知道但刻意推迟修的技术债。正确做法是 `signal.NotifyContext` 监听 SIGTERM，配合 `srv.Shutdown(ctx)` 做优雅停机，把 `log.Fatal` 换成正常的错误返回路径。现在没做是因为当前存储在内存里，重启本来就丢数据，优雅停机保不住任何东西；等换成 Redis/SQLite 时才有意义。
>
> ### 陷阱五：recover 必须被 defer 直接调用
>
> ```go
> defer recover()              // 无效
> defer func() { recover() }() // 有效
> ```
> 只有在被 defer 的那个函数体内直接调用 `recover()` 才能拦住 panic，再包一层函数就失效了。

---

## 8. slice 扩容与底层数组共享

### Q：slice 的底层结构是什么？append 之后原来的 slice 会不会变？

> **底层结构**：slice 是一个**三字段的结构体**（不是指针，也不是数组）：
> ```go
> type slice struct {
>     array unsafe.Pointer // 指向底层数组
>     len   int            // 当前长度
>     cap   int            // 从 array 起点到底层数组末尾的容量
> }
> ```
> 传参时**按值拷贝这三个字段**，所以两个 slice 变量各有独立的 len/cap，但 `array` 指向**同一块底层数组**。这是所有 slice 陷阱的根源。
>
> **扩容规则（Go 1.18+）**：
> 1. `append` 后所需长度 ≤ cap：**原地写入底层数组**，不分配新内存。
> 2. 超出 cap：分配新数组，拷贝旧数据，返回指向新数组的 slice。
>    *   旧 cap **< 256**：直接翻倍。
>    *   旧 cap **≥ 256**：按 `newcap += (newcap + 3*256) / 4` 递增，从 2 倍平滑过渡到约 1.25 倍，避免大 slice 一次多要一倍内存。
> 3. 算出来的容量还要按 mallocgc 的 **size class 向上取整**，所以实测 cap 常常比公式值略大。
>
> （Go 1.17 及以前的阈值是 1024，且是硬切换的 2 倍 / 1.25 倍。答题时说清版本，别背错。）
>
> ### 陷阱一：append 是否影响原 slice，取决于有没有扩容
>
> ```go
> a := []int{1, 2, 3, 4, 5}
> b := a[1:3]        // b = [2 3], len=2, cap=4（从下标1到末尾）
> b = append(b, 99)  // len 3 ≤ cap 4，原地写！
> // a 变成 [1 2 3 99 5] —— a[3] 被踩了
> ```
> 如果 `b` 的 cap 恰好满了，append 会复制一份新数组，`a` 就毫发无损。**同一段代码的行为随 cap 而变，这正是它危险的地方**：测试用小数据看不出问题，生产数据一变就出诡异 bug。
>
> 防御写法是**三索引切片**限制 cap：
> ```go
> b := a[1:3:3] // cap 也是 2，下次 append 必然复制，绝不踩到 a
> ```
>
> ### 陷阱二：函数里 append 不影响调用方的 len
>
> ```go
> func add(s []int) { s = append(s, 1) } // 调用方看不到
> func add(s []int) []int { return append(s, 1) } // 必须返回
> ```
> 因为 slice 结构体是值拷贝，被调方改的是自己那份 `len`。这就是为什么标准库的 `append` 签名是**返回新 slice** 而不是原地修改——语言层面它做不到。
>
> ### 陷阱三：小切片让大数组无法 GC
>
> ```go
> data := readAll()      // 100MB
> head := data[:10]      // 只想留 10 个元素
> // 但 head.array 指向那 100MB 的数组，整块都不能回收
> ```
> 解法：`head := append([]int(nil), data[:10]...)` 或 `copy` 到新 slice，切断对大数组的引用。
>
> ### 陷阱四：`copy` 按两者 len 的最小值拷贝
>
> ```go
> dst := make([]int, 0, 10) // len=0！
> copy(dst, src)            // 拷了 0 个元素
> ```
> `copy` 看的是 `len` 不是 `cap`。要么 `make([]int, len(src))`，要么用 `append`。

---

## 9. map 为什么不能并发读写

### Q：并发读写 map 会发生什么？为什么 Go 不让 map 自带锁？

> **现象**：
> ```
> fatal error: concurrent map read and map write
> fatal error: concurrent map writes
> ```
> 注意是 **`fatal error` 而不是 `panic`**：这是 runtime 直接 `throw`，**`recover()` 拦不住，整个进程必死**。Go 团队故意选择不给逃生通道。
>
> **检测机制**：`hmap` 结构里有个 `flags` 字段，写操作开始时置上 `hashWriting` 位，结束时清除。
> *   写入前发现该位已经是 1 → 有别人在写 → `throw`。
> *   读取时发现该位是 1 → 有人正在写 → `throw`。
>
> 这是**廉价的竞态检测，不是锁**：一次位检查，无原子操作开销，也因此**只能抓到部分并发场景**（时间窗口错开就抓不到）。不要以为"没报错就是安全的"，真正查竞态要用 `go test -race`。
>
> **追问一：为什么必须直接崩，不能容忍脏读？**
> 因为 map 会**渐进式扩容（incremental rehashing）**：装载因子超标时分配新桶数组，然后在每次写入时搬迁一两个旧桶（`growWork`），`oldbuckets` 和 `buckets` 会同时存在一段时间。如果这期间有并发读者，它可能读到**搬迁进行到一半的桶**——返回错误的值、遍历出重复元素、甚至跟着无效指针走。**返回悄悄错掉的数据比崩溃糟糕得多**（这是"fail loudly"原则的教科书案例）。
>
> **追问二：为什么 Go 不给 map 内置锁？**
> 因为**绝大多数 map 只在单个 goroutine 里用**。内置锁意味着所有这些场景都要白付原子操作和缓存行竞争的代价。Go 的取舍是"不为你没用到的东西付费"，把并发保护交给调用方按实际需要选：`Mutex`、`RWMutex`、`sync.Map`、分片锁。
>
> **本项目怎么做的（这段是重点）**：
> `store.Store` 用 `sync.Mutex` 保护 `map[string]Task`。
>
> 更关键的是——**这里不能用 `sync.Map` 替代**。因为 `Update`/`Complete` 是**读-改-写的复合操作**：
> ```go
> task, ok := s.tasks[id] // 读
> if !ok { return ErrNotFound }
> task.Status = status    // 改
> s.tasks[id] = task      // 写
> ```
> 就算底层容器每一步单独看都是并发安全的，两个 goroutine 交错执行这三步仍然会**丢更新**（后写的覆盖先写的）。**并发安全的容器 ≠ 并发安全的业务逻辑**——需要保护的是整个复合操作的原子性，所以必须自己持锁横跨这三步。
>
> 另外 `Store` 里存的是 `Task` 值而不是 `*Task`：从 map 取出来是一份拷贝，即使调用方拿着它乱改也污染不到 store 里的权威副本。这和"队列只传 task ID"是同一个思路——**store 是状态的唯一权威**。

---

## 10. interface 的底层结构（iface / eface）

### Q：interface 在内存里长什么样？为什么 `err != nil` 有时会骗人？

> **两种结构，都是两个字宽**：
>
> 空接口 `interface{}` / `any` 用 **`eface`**：
> ```go
> type eface struct {
>     _type *_type         // 动态类型信息
>     data  unsafe.Pointer // 指向实际数据
> }
> ```
>
> 带方法的接口用 **`iface`**：
> ```go
> type iface struct {
>     tab  *itab          // 类型 + 方法表
>     data unsafe.Pointer // 指向实际数据
> }
>
> type itab struct {
>     inter *interfacetype // 接口类型，如 chatter
>     _type *_type         // 具体类型，如 *llm.Client
>     fun   [1]uintptr     // 方法地址数组，编译期填好，变长
> }
> ```
> 记住一句就够：**接口值 = 类型信息 + 数据指针**。`itab` 里的 `fun` 是把具体类型的方法地址按接口方法顺序排好的表，`itab` 本身由运行时缓存在全局哈希表里，同一组（接口, 具体类型）只生成一次。
>
> **动态派发的成本**：调用 `p.llm.Chat(...)` 时，要先从 `itab.fun[0]` 加载函数地址再间接跳转。比直接调用多一次内存访问，更重要的是**编译器无法内联**接口方法。这就是"接口不是免费的"的具体含义——但代价只是一次指针加载，为了可测试性完全值得。
>
> ### 陷阱：nil 接口 ≠ 装着 nil 指针的接口
>
> ```go
> var p *llm.APIError = nil
> var err error = p
> fmt.Println(err == nil) // false！
> ```
> 因为 `err` 的 `tab` 字段被填上了 `(error, *APIError)` 的 itab，只有 `data` 是 nil。**接口等于 nil 的条件是两个字都为零**，而这里类型信息在。
>
> 真实事故长这样：
> ```go
> func doSomething() error {
>     var e *MyError        // 具体类型
>     if somethingFailed {
>         e = &MyError{...}
>     }
>     return e              // 即使没出错，返回的 error 也非 nil
> }
> ```
> **修法：返回类型写成 `error`，成功路径显式 `return nil`**，不要让具体指针类型直接当返回值。
>
> **本项目怎么避开的**：`Chat` 的签名是 `(string, error)`，出错时 `return "", &APIError{...}`，成功时 `return content, nil`——从来没有"声明一个 `*APIError` 变量再返回它"这一步，所以踩不到这个坑。
>
> **类型断言的原理**：`v, ok := i.(T)`，T 是具体类型时直接比较 `_type` 指针；T 是接口时要查/建 itab 看方法是否齐全。
>
> **项目关联**：`chatter` 接口只有一个方法，所以它的 `itab.fun` 只有一项。接口定义在 **worker 包**（消费方定义接口），因此 worker **不 import llm 包**——`import` 语句的消失是解耦的硬证据。

---

## 11. 值接收者 vs 指针接收者

### Q：方法什么时候用值接收者，什么时候用指针接收者？

> **方法集规则（这是全部考点的根）**：
> *   类型 `T` 的方法集：**只有值接收者的方法**。
> *   类型 `*T` 的方法集：**值接收者 + 指针接收者的方法**。
>
> **直接后果**：如果一个接口里有指针接收者实现的方法，**只有 `*T` 满足这个接口，`T` 不满足**。
>
> ```go
> type Speaker interface { Speak() }
> type Dog struct{}
> func (d *Dog) Speak() {}
>
> var s Speaker = Dog{}  // 编译错误
> var s Speaker = &Dog{} // OK
> ```
>
> **追问：为什么 `d.Speak()` 能直接写（d 是 Dog 值），赋给接口就不行？**
> 因为 `d` 是**可寻址**的变量，编译器帮你改写成 `(&d).Speak()`。而**接口里存的是一份拷贝，那份拷贝不可寻址**——就算能取到地址，改的也是拷贝而不是原值，语言干脆禁止这种误导性的隐式转换。
>
> **怎么选**：
> 1. **方法要修改接收者** → 必须指针。值接收者改的是拷贝，改完就丢。
> 2. **结构体里有 `sync.Mutex`、`sync.WaitGroup` 等** → **必须指针**。值接收者会拷贝整把锁，两份锁各自独立，等于没加锁。`go vet` 的 `copylocks` 检查专门抓这个。
> 3. **结构体大** → 指针，避免每次调用拷贝。
> 4. **小的、不可变的值**（`time.Time`、坐标点这类）→ 值接收者，更安全也更容易被内联。
> 5. **一致性**：同一个类型的所有方法统一风格，不要一半值一半指针。
>
> **本项目的三个实例**：
> *   `Store` 有 `mu sync.Mutex` 字段，所以所有方法都是 `func (s *Store) ...`——**这是硬性要求，不是风格选择**。值接收者会把 Mutex 一起拷走，锁立刻失效。
> *   `APIError` 用指针接收者：`func (e *APIError) Error() string`。所以**只有 `*APIError` 实现了 error 接口**，`APIError{}` 值不是 error。这就是 `client.go` 里写 `return "", &APIError{...}` 而不是 `APIError{...}` 的原因，也是测试里必须声明 `var apiErr *APIError` 再 `errors.As(err, &apiErr)` 的原因——`As` 的目标类型必须是那个真正实现了 error 的类型。
> *   `fakeChatter` 用**值接收者**：`func (f fakeChatter) Chat(...)`。所以 `fakeChatter{...}` 和 `&fakeChatter{...}` **都**满足 `chatter`，测试里直接传值最省事。它只有两个不可变的配置字段（`delay`、`err`），不需要修改自身，用值接收者是对的。

---

## 12. 错误包装：`%w` / `errors.Is` / `errors.As`

### Q：`%w` 和 `%v` 有什么区别？`Is` 和 `As` 分别什么时候用？

> **`%w` 建链，`%v` 断链**：
> ```go
> fmt.Errorf("llm: send request: %w", err) // 生成 wrapError，带 Unwrap() error 方法
> fmt.Errorf("llm: send request: %v", err) // 只拼字符串，原错误再也取不出来
> ```
> 两者打印出来一模一样，**区别只在能不能被 `Is`/`As` 穿透**。Go 1.20 起 `%w` 可以出现多次，配合 `errors.Join` 组多错误树。
>
> **`errors.Is(err, target)`** —— 判断"是不是这个特定错误"：
> 沿 `Unwrap` 链逐层做 `==` 比较（若某层实现了 `Is(error) bool` 方法则优先调它）。**用于哨兵错误（sentinel error）**，即预先声明的 `var ErrXxx = errors.New(...)`。
>
> **`errors.As(err, &target)`** —— 判断"是不是这类错误，并把它取出来"：
> 沿链找第一个能赋值给 `target` 的类型，赋值后返回 true。**用于自定义错误类型**，因为你要读它的字段。`target` 必须是指针，否则 panic。
>
> 一句话记：**`Is` 比身份，`As` 取内容。**
>
> **本项目的三处用法**：
>
> 1. **哨兵 + `Is`**：`store.ErrNotFound` 是包级哨兵变量，`api/handler.go` 里
>    ```go
>    if errors.Is(err, store.ErrNotFound) { // 404
>    ```
>    store 的文档注释明确写了"调用方应当用 errors.Is 判断，而不是比较字符串"——**字符串比较会因为错误信息措辞改动而静默失效**，属于隐形耦合。
>
> 2. **类型 + `As`**：`APIError` 带 `StatusCode` 和 `Body` 字段，测试里
>    ```go
>    var apiErr *APIError
>    if !errors.As(err, &apiErr) { ... }
>    ```
>    要读状态码就只能用 `As`，`Is` 给不了字段。
>
> 3. **实测 `%w` 穿透两层（这个例子拿去讲很有说服力）**：
>    超时测试断言的是 `errors.Is(err, context.DeadlineExceeded)`，而这条链有三层：
>    ```
>    fmt.Errorf("llm: send request: %w", err)   ← 我写的
>      └─ *url.Error                            ← http.Client.Do 包的
>           └─ context.DeadlineExceeded          ← 真正的原因
>    ```
>    `errors.Is` 一路 `Unwrap` 到底才匹配上。**中间任何一层换成 `%v`，断言就失败**——这是"错误链是一条契约链，一处断掉整条报废"的实证。
>
> **追问：所有错误都该 wrap 吗？不是。**
> `worker/pool.go` 里刻意**不** wrap：
> ```go
> slog.Error("worker: task %s failed: %v", id, err)             // 全文进日志
> p.tasks.Complete(id, "", errors.New("upstream error"))        // 只存粗粒度分类
> ```
> 因为 `task.Error` 会经 `GET /tasks/{id}` 原样返回给外部调用方，而原始错误里裹着上游响应体，可能含配额、账单、内部端点等信息。**这里 wrap 就是信息泄露**。原则是：**日志留全文，响应留分类**。
>
> 另一个不该 wrap 的理由：wrap 会把内部实现暴露成 API 契约。一旦调用方开始 `errors.Is(err, sql.ErrNoRows)`，你就再也换不掉数据库了。想切断这种耦合就用 `%v`，或转成自己包的错误类型。
