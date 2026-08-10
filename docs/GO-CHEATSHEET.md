# Go 代码规范速查 — 人类参考

> 基于 Dave Cheney《Practical Go》。每条规则一句话 + 最短示例。

---

## 1. 原则
- **简单第一**：不提前优化、不过度设计
- **可读优先**：代码是写给人看的
- **生产力**：让工具替你干活（gofmt、go vet）

## 2. 命名
- **清晰 > 简洁**：`oid` 而非 `o`，`index` 而非 `i`（长作用域时）
- **作用域大 → 名字长**：循环用 `i`，参数用 `people`，包级用 `sqlDrivers`
- **禁止类型后缀**：`usersMap` → `users`
- **同一概念同一名字**：数据库统一叫 `db`
- **包名 = 服务**：`package strings` 而非 `package util`
- **声明**：零值用 `var x int`，初始化用 `x := 1`

## 3. 注释
- **公共符号必须注释**，以符号名开头：`// Now returns ...`
- **注释说"为什么"**，不说"是什么"
- **不注释烂代码 → 重写它**
- **与其注释一块代码 → 提取成函数**

## 4. 包设计
- **禁止 `base`/`common`/`util`**，拆分到调用者内
- **早 return，别嵌套**：guard clause 提前退
- **让零值有用**：`var b bytes.Buffer` 直接可用
- **禁止公共包级变量**：用结构体字段替代

## 5. 项目结构
- **更少、更大的包**：别过度拆
- **`internal/` 限 API**：项目内共享、对外私有
- **main 包极简**：只解析 flag、建连接、交给高层对象

## 6. API 设计
- **难用对的参数 → 改签名**：`CopyFile(to, from)` → `Source.CopyTo(dest)`
- **默认用例优先**：别强迫调用者传不在乎的参数
- **可变参数 > `[]T`**：`ShutdownVMs(ids ...string)`
- **参数用最小接口**：`func Save(w io.Writer, ...)` 而非 `*os.File`

## 7. 错误处理
- **错误只处理一次**：要么记录，要么返回，别都做
- **包装别裸传**：`return fmt.Errorf("marshal: %w", err)`
- **忽略就显式**：`_ = w.Write(buf)`
- **`log.Fatal` 只放 main/init**

## 8. 并发
- **别滥用 goroutine**：能自己做就自己做
- **并发决定留给调用者**：同步返回，调用者自行 `go`
- **每个 goroutine 都能停**：用 channel/context 传播取消
- **别用 `for{}`/`select{}` 阻塞 main**

## 9. 工具
- **gofmt 无条件通过**
- **go build / go vet 零警告**
