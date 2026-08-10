# Go 语言开发规范 — Agent 约束版

> 基于 Dave Cheney《Practical Go: Real world advice for writing maintainable Go programs》
> 本文件为 agent 编写或审查 Go 代码时必须遵守的规范。
> 规则级别：**MUST** = 强制违反即为错误 · **SHOULD** = 强烈建议 · **MAY** = 可选最佳实践

---

## 1. 指导原则

### 1.1 简单性（MUST）

代码 MUST 追求简单。不引入不必要的抽象、不提前优化、不过度设计。

### 1.2 可读性（MUST）

代码 MUST 优先为人类阅读而写，而非为机器执行。可读性是可维护性的前提。

### 1.3 生产力（SHOULD）

SHOULD 避免浪费时间在工具、构建、调试不可读代码上。利用 Go 生态工具提升效率。

---

## 2. 标识符命名

### 2.1 命名是为了清晰，不是简洁（MUST）

标识符 MUST 优先保证清晰，而非追求短小。名称应具有高信噪比。

```go
// 误
func (s *SNMP) Fetch(o []int, i int) (int, error)

// 正
func (s *SNMP) Fetch(oid []int, index int) (int, error)
```

### 2.2 命名长度随作用域距离增长（SHOULD）

- 循环变量、短作用域 → 单字母（`i`, `p`）
- 函数参数、返回值 → 单个词（`people`, `count`）
- 包级别声明、长生命周期 → 多个词（`sqlDrivers`, `sizeCalculationDisabled`）
- 方法、接口、包 → 单个词

### 2.3 禁止在变量名中包含类型（MUST NOT）

MUST NOT 使用 `usersMap`, `configList`, `dbPtr` 等类型后缀。

```go
// 误
var usersMap map[string]*User

// 正
var users map[string]*User
```

### 2.4 使用一致的命名风格（MUST）

同一概念 MUST 在全文中保持同一名称。如数据库连接统一用 `db`，而非 `d`/`dbase`/`DB`/`database` 混用。方法接收者名称 MUST 在同一类型的所有方法中保持一致。

### 2.5 使用一致的声明样式（MUST）

| 场景 | 方式 |
|---|---|
| 声明但不初始化（零值） | `var x int` / `var s []string` |
| 声明并初始化 | `x := 1` / `things := make([]Thing, 0)` |

```go
// 误：用 := 声明零值，读者看不出是否有意为之
var players int = 0

// 正
var players int
```

### 2.6 遵循项目既有风格（MUST）

MUST 遵循项目自身已有风格。一致性优先于个人偏好。如果代码通过了 `gofmt`，通常不值得在审查中重命名。

---

## 3. 注释

### 3.1 注释应描述"为什么"而非"是什么"（SHOULD）

变量/常量注释 MUST 描述其内容或来源，而非重复名称本身已表达的目的。

```go
// 误：重复了名称
// default path
var defaultPath = "/usr/home"

// 正：解释了来源
// defaultPath is the fallback when HOME is unset.
var defaultPath = "/usr/home"
```

### 3.2 公共符号必须注释（MUST）

包中所有公共符号（变量、常量、函数、方法、类型）MUST 有注释。注释 MUST 以该符号名称开头。

```go
// 正
// Now returns the current time.
func Now() Time
```

### 3.3 实现接口的方法注释应提供信息（SHOULD NOT）

MUST NOT 写无信息量的接口实现注释：

```go
// 误：毫无信息
// Read implements the io.Reader interface.
func (r *FileReader) Read(buf []byte) (int, error)

// 正：直接删除该注释，或写具体行为
```

### 3.4 不要注释糟糕的代码，重写它（MUST）

MUST NOT 用注释掩盖烂代码。若发现需要大段注释才能说清的代码，应重构为独立命名函数。

### 3.5 与其注释一段代码，不如提取函数（SHOULD）

```go
// 误：靠注释解释一块代码
// queue all dependent actions
var results []chan error
for _, dep := range a.Deps {
    results = append(results, execute(seen, dep))
}

// 正：提取为函数，函数名即文档
func queueDependents(a Action) []chan error { ... }
```

---

## 4. 包的设计

### 4.1 包名描述服务，而非内容（MUST）

包名 MUST 是单一名词，描述它提供什么服务，而非包含什么类型。

```go
// 误
package httphelpers
package common

// 正
package http
package strings
```

### 4.2 禁止使用 `base` / `common` / `util` 包名（MUST NOT）

MUST NOT 创建名为 `base`, `common`, `util`, `helpers` 的包。应拆分到调用者包内，或用服务导向的命名。少量重复优于错误抽象。

### 4.3 尽早 return，避免深度嵌套（MUST）

MUST 使用 guard clause 提前返回错误，使成功路径沿屏幕向下延伸（视线编码）。

```go
// 误：成功路径嵌套在右侧深处
if ok {
    if ready {
        doWork()
    }
}

// 正：guard clause 提前返回
if !ok {
    return err
}
if !ready {
    return err
}
doWork()
```

### 4.4 让零值有用（SHOULD）

类型设计 SHOULD 使其零值可直接使用、具有合理默认行为。如 `sync.Mutex`、`bytes.Buffer`。

### 4.5 禁止包级别可变状态（MUST NOT）

MUST NOT 使用公共包级别可变变量。它是全局状态，会引入紧密耦合。应使用结构体字段 + 接口来解耦。

---

## 5. 项目结构

### 5.1 优先更少、更大的包（SHOULD）

SHOULD 避免过度拆包。除 `cmd/` 和 `internal/` 外，每个包 MUST 包含源代码。

### 5.2 文件组织（SHOULD）

- 起始一个文件，文件名 = 包名（如 `http.go`）
- 增长后按职责拆分（`client.go`, `server.go`, `messages.go`）
- 文件 SHOULD 以名词命名

### 5.3 测试放置（SHOULD）

- 单元测试 SHOULD 用内部测试（`package xxx`）
- Example 测试 SHOULD 用外部测试（`package xxx_test`），确保 godoc 中带包名前缀

### 5.4 使用 `internal/` 限制公共 API（MAY）

MAY 使用 `internal/` 目录存放项目内共享但不对外公开的代码。

### 5.5 main 包必须精简（MUST）

`main` 函数 MUST 仅做 flag 解析、连接建立、日志初始化，然后将执行交给高层对象。业务逻辑 MUST NOT 写在 main 包中。

---

## 6. API 设计

### 6.1 设计难以被误用的 API（MUST）

API MUST 易于正确使用、难以误用。

### 6.2 避免多个同类型参数（MUST NOT）

MUST NOT 设计两个以上同类型连续参数的函数。应用命名类型或方法形式区分。

```go
// 误：顺序易混淆
func CopyFile(to, from string) error

// 正：用命名类型区分
type Source string
func (src Source) CopyTo(dest string) error

// 或：改为方法
func (from *File) CopyTo(to string) error
```

### 6.3 为默认用例设计 API（SHOULD）

API SHOULD 为最常见用例服务，不应强制调用者传递他们不在乎的参数。

### 6.4 不鼓励 `nil` 作为可选参数（SHOULD NOT）

MUST NOT 在同一函数签名中混合可为 nil 和不可为 nil 的参数，nil 行为会"传染"。

### 6.5 优先可变参数而非 `[]T`（SHOULD）

```go
// 误：假设总是多个，调用者需打包
func ShutdownVMs(ids []string) error

// 正
func ShutdownVMs(ids ...string) error
```

### 6.6 让函数声明所需的最小接口（MUST）

函数参数 MUST 使用能满足需求的最小接口类型（接口隔离原则）。

```go
// 误：Save 只需要写，却要求 *os.File
func Save(f *os.File, doc *Document) error

// 正：只要求 io.Writer
func Save(w io.Writer, doc *Document) error
```

---

## 7. 错误处理

### 7.1 通过消除错误来减少错误处理（SHOULD）

SHOULD 重构代码使错误不再发生，而非改进错误处理语法。善用辅助类型（如 `bufio.Scanner`）封装错误处理。

### 7.2 错误只处理一次（MUST）

错误 MUST 只被处理一次。处理 = 检查错误并做单一决策。

```go
// 误：既记录又返回，调用者可能再记录一次 → 重复日志
if err != nil {
    log.Println("unable to write:", err)
    return err
}
```

### 7.3 用 `fmt.Errorf` 或 `errors.Wrap` 添加上下文（SHOULD）

SHOULD 在返回时用上下文包装错误，而非单独记录。这样也避免忘记 return。

```go
// 正
if err != nil {
    return fmt.Errorf("could not marshal config: %w", err)
}
```

### 7.4 忽略错误必须显式（MUST）

若有意忽略错误，MUST 显式处理（赋给 `_`），不做任何决策。

### 7.5 `log.Fatal` 仅限 main/init（MUST NOT）

MUST NOT 在 `main.main` 或 `init` 之外使用 `log.Fatal`。它会无条件 `os.Exit`，跳过 defer、不通知其他 goroutine。

---

## 8. 并发

### 8.1 不要过度使用 goroutine（SHOULD NOT）

SHOULD NOT 启动不必要的 goroutine。若 goroutine 在等另一个结果才能推进，自己做更简单。

### 8.2 将并发决定留给调用者（SHOULD）

SHOULD 让调用者决定是否异步执行。

```go
// 误：强制异步，调用者无法控制 goroutine 生命周期
func ListDirectory(dir string) chan string

// 正：同步返回，调用者自行 go
func ListDirectory(dir string) ([]string, error)
```

### 8.3 永远不要启动无法停止的 goroutine（MUST）

每个 goroutine MUST 有明确的停止机制（channel、context）。否则会导致 goroutine 泄漏。

### 8.4 用 channel/context 传播取消信号（SHOULD）

SHOULD 通过 stop channel 或 context 通知 goroutine 关闭，使其干净退出。

```go
// 正
func serve(addr string, handler http.Handler, stop <-chan struct{}) error {
    s := http.Server{Addr: addr, Handler: handler}
    go func() {
        <-stop
        s.Shutdown(context.Background())
    }()
    return s.ListenAndServe()
}
```

---

## 9. 工具与格式

### 9.1 gofmt（MUST）

所有代码 MUST 通过 `gofmt` / `goimports` 格式化。项目中不争论格式问题。

### 9.2 编译器警告（MUST NOT）

MUST NOT 提交无法通过 `go build` / `go vet` 的代码。
