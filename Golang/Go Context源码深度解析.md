# Go Context 源码深度解析：从接口设计到级联取消

> 本文基于 Go 标准库 `context` 包的源码，逐层拆解其内部实现，带你理解 `Context` 的接口设计、取消机制、值传递以及级联取消的精妙工程。

---

## 目录

- [Go Context 源码深度解析：从接口设计到级联取消](#go-context-源码深度解析从接口设计到级联取消)
  - [目录](#目录)
  - [1. Context 接口：一切的起点](#1-context-接口一切的起点)
  - [2. 根节点：Background 与 TODO](#2-根节点background-与-todo)
    - [Background](#background)
    - [TODO](#todo)
    - [contextName：统一的命名工具](#contextname统一的命名工具)
  - [3. 内部辅助类型](#3-内部辅助类型)
  - [4. cancelCtx：取消机制的核心](#4-cancelctx取消机制的核心)
    - [Value 方法：自我发现](#value-方法自我发现)
    - [Done 方法：双重检查锁定 + 懒初始化](#done-方法双重检查锁定--懒初始化)
    - [Err 方法：行为一致性优先](#err-方法行为一致性优先)
  - [5. parentCancelCtx 与 removeChild：父子关系管理](#5-parentcancelctx-与-removechild父子关系管理)
    - [parentCancelCtx：找到可信赖的父 cancelCtx](#parentcancelctx找到可信赖的父-cancelctx)
    - [removeChild：从父节点摘除自己](#removechild从父节点摘除自己)
  - [6. cancel 方法：一次取消的完整流程](#6-cancel-方法一次取消的完整流程)
  - [7. Cause：更丰富的取消原因](#7-cause更丰富的取消原因)
  - [8. WithoutCancel：切断取消信号的传播](#8-withoutcancel切断取消信号的传播)
  - [9. valueCtx：值的链式传递](#9-valuectx值的链式传递)
    - [WithValue](#withvalue)
    - [valueCtx 结构](#valuectx-结构)
    - [value 函数：O(N) 链表遍历](#value-函数on-链表遍历)
  - [10. 错误类型：Canceled 与 DeadlineExceeded](#10-错误类型canceled-与-deadlineexceeded)
  - [11. WithCancel 与 WithCancelCause](#11-withcancel-与-withcancelcause)
    - [WithCancel](#withcancel)
    - [WithCancelCause](#withcancelcause)
  - [12. timerCtx：超时的实现](#12-timerctx超时的实现)
    - [timerCtx 的 cancel](#timerctx-的-cancel)
  - [13. WithDeadline 与 WithTimeout](#13-withdeadline-与-withtimeout)
    - [WithDeadlineCause：核心实现](#withdeadlinecause核心实现)
    - [WithTimeout 系列](#withtimeout-系列)
  - [14. afterFuncCtx 与 AfterFunc](#14-afterfuncctx-与-afterfunc)
    - [afterFuncCtx](#afterfuncctx)
    - [AfterFunc](#afterfunc)
  - [15. propagateCancel：级联取消的核心算法](#15-propagatecancel级联取消的核心算法)
    - [四种情况详解](#四种情况详解)
  - [16. afterFuncer：自定义取消源的适配方案](#16-afterfuncer自定义取消源的适配方案)
  - [总结](#总结)

---

## 1. Context 接口：一切的起点

在 Go 的并发编程中，goroutine 的创建极其廉价，但随之而来的问题是：**如何优雅地管理和终止这些并发任务？**

典型场景包括：

1. **超时取消**：一次 HTTP 请求调用了多个下游服务，如果上游超时，需要及时通知所有下游 goroutine 停止工作，避免资源泄漏。
2. **主动取消**：用户取消了某个操作，所有相关的 goroutine 应当立即退出。
3. **请求级数据传递**：`traceID`、`userID` 等贯穿整个请求链路的数据，需要一个标准化的传递方式。

`context` 包正是为解决这些问题而设计的，它提供了一套**树状、可取消、可携带值**的机制。

```
Background()
    │
    ├── WithCancel()
    │       ├── WithValue("userID", "123")
    │       └── WithTimeout(5s)
    │
    └── WithValue("traceID", "abc")
            └── WithDeadline(T)
```

**核心设计理念**：Context 是不可变的树形结构，每个派生 context 都通过单链指向父节点。父节点取消时，所有子节点自动级联取消。

`Context` 包的核心只有一个接口：

```go
type Context interface {
    Deadline() (deadline time.Time, ok bool) // 返回 context 的截止时间（如果有）
    Done() <-chan struct{}                   // 返回一个只读 channel，当 context 被取消或超时时关闭
    Err() error                              // 返回 context 被关闭的原因（Canceled 或 DeadlineExceeded）
    Value(key any) any                       // 获取与 key 关联的请求范围的值
}
```

这四个方法定义了 context 的全部能力：

| 方法         | 语义                                              |
| ------------ | ------------------------------------------------- |
| `Deadline()` | 告诉调用者："我最多活到什么时候"                  |
| `Done()`     | 告诉调用者："我怎么死的"——通过 channel 关闭来广播 |
| `Err()`      | 告诉调用者："我为什么死的"                        |
| `Value()`    | 在请求链路中传递少量数据                          |

整个 context 包的所有实现，都是围绕这四个方法的不同策略来展开的。

---

## 2. 根节点：Background 与 TODO

所有 context 树的根都源自一个空实现——`emptyCtx`：

```go
type emptyCtx struct{}

func (emptyCtx) Deadline() (deadline time.Time, ok bool) { return }
func (emptyCtx) Done() <-chan struct{}                    { return nil } // 没有取消能力
func (emptyCtx) Err() error                               { return nil }
func (emptyCtx) Value(key any) any                        { return nil }
```

`emptyCtx` 的所有方法都返回零值，意味着它**永远不会取消、没有超时、不携带任何值**。在此基础上，标准库派生出两个语义不同的根节点：

### Background

```go
type backgroundCtx struct{ emptyCtx }

func (backgroundCtx) String() string { return "context.Background" }
func Background() Context           { return backgroundCtx{} }
```

`context.Background()` 是整个 context 树的根，通常用于 `main` 函数、初始化逻辑或顶层请求处理的入口。

### TODO

```go
type todoCtx struct{ emptyCtx }

func (todoCtx) String() string { return "context.TODO" }
func TODO() Context           { return todoCtx{} }
```

`context.TODO()` 和 `Background()` 在行为上完全一致，区别仅在语义——它表示"这里**将来**应该传入一个 context，但我还没想好从哪来"。在代码审查中看到 `TODO()` 就知道这是一个待替换的占位符。

### contextName：统一的命名工具

```go
type stringer interface {
    String() string
}

func contextName(c Context) string {
    if s, ok := c.(stringer); ok {
        return s.String()
    }
    return reflectlite.TypeOf(c).String()
}
```

`contextName` 会检查 context 是否实现了 `String()` 方法，如果有就调用它，否则回退到类型名。这使得每个 context 在日志中都能输出可读的名字。

---

## 3. 内部辅助类型

在深入核心实现之前，先看几个贯穿全文的内部类型：

```go
type canceler interface {
    cancel(removeFromParent bool, err, cause error)
    Done() <-chan struct{}
}

type stopCtx struct {
    Context
    stop func() bool
}
```

- **`canceler`**：所有可取消的 context 都实现这个接口，用于统一管理取消操作。
- **`stopCtx`**：一个包装器，用于在用户自定义 `Done()` 的场景下，提供安全的子 context 挂载点。后文会详细展开。

---

## 4. cancelCtx：取消机制的核心

`cancelCtx` 是整个 context 包最重要的结构体，所有可取消的 context（`WithCancel`、`WithDeadline`、`WithTimeout`）都基于它。

```go
var cancelCtxKey int // 将 &cancelCtxKey 作为返回 cancelCtx 自身的 Key

type cancelCtx struct {
    Context                          // 指向父 context

    mu       sync.Mutex              // 保护以下字段
    done     atomic.Value            // chan struct{}，懒初始化，被 cancel 时关闭
    children map[canceler]struct{}   // 子 context 集合，cancel 后置为 nil
    err      atomic.Value            // 第一个 cancel 调用设置为非 nil
    cause    error                   // 第一个 cancel 调用设置的根因
}
```

关键字段说明：

| 字段       | 作用                                                   |
| ---------- | ------------------------------------------------------ |
| `done`     | 懒初始化的 channel，用 `atomic.Value` 存储，避免锁竞争 |
| `children` | 所有子 canceler 的集合，cancel 时级联取消              |
| `err`      | 原子值，非 nil 表示已取消                              |
| `cause`    | 取消的根因，供 `Cause()` 使用                          |

> **`atomic.Value` + `sync.Mutex` 的组合**：`done` 和 `err` 使用原子操作实现无锁快路径读取，`mu` 仅在需要修改时加锁。原子读比互斥锁快约 5 倍，这在热路径上非常关键。

### Value 方法：自我发现

```go
func (c *cancelCtx) Value(key any) any {
    if key == &cancelCtxKey {
        return c
    }
    return value(c.Context, key)
}
```

当 key 为 `&cancelCtxKey` 时返回自身，这使得 `parentCancelCtx` 能够沿着 Value 链找到最近的 `*cancelCtx`——这是级联取消的关键机制。

### Done 方法：双重检查锁定 + 懒初始化

```go
func (c *cancelCtx) Done() <-chan struct{} {
    d := c.done.Load() // 原子加载，无锁快路径
    if d != nil {
        return d.(chan struct{})
    }
    // Double-Checked Locking
    c.mu.Lock()
    defer c.mu.Unlock()
    d = c.done.Load()
    if d == nil {
        d = make(chan struct{})
        c.done.Store(d) // 懒初始化
    }
    return d.(chan struct{})
}
```

这里采用了经典的**双重检查锁定（Double-Checked Locking）**优化：

1. **第一次检查（无锁）**：原子加载 `done`，如果已初始化则直接返回——这是大多数调用走的路径。
2. **加锁后二次检查**：防止并发场景下多个 goroutine 同时进入创建分支。

为什么选择**懒初始化**？因为很多 context 在其生命周期中根本不会有人调用 `Done()`，提前创建 channel 只会造成无意义的内存开销。

### Err 方法：行为一致性优先

```go
func (c *cancelCtx) Err() error {
    if err := c.err.Load(); err != nil {
        <-c.Done() // 确保 done channel 已关闭
        return err.(error)
    }
    return nil
}
```

> **行为一致性高于实现便利**：只要 `Err()` 返回错误，所有 `select { case <-ctx.Done(): }` 一定能退出。

注意这里 `<-c.Done()` 的精妙之处：`Done()` channel 的关闭是"广播式"的——关闭 channel 可以唤醒所有等待者，而发送一个值只能唤醒一个 goroutine。选择"关闭 channel"而非"发送值"有以下优势：

- **广播能力**：所有监听者同时收到信号
- **零内存消耗**：不需要 buffer
- **不可逆**：channel 关闭后不能重新打开，完美匹配"取消只能一次"的语义
- **不阻塞发送者**：取消操作本身不会被阻塞

---

## 5. parentCancelCtx 与 removeChild：父子关系管理

### parentCancelCtx：找到可信赖的父 cancelCtx

```go
var closedchan = make(chan struct{}) // 全局已关闭 channel，复用于所有已取消的 context

func init() {
    close(closedchan)
}
```

```go
func parentCancelCtx(parent Context) (*cancelCtx, bool) {
    done := parent.Done()
    if done == closedchan || done == nil {
        return nil, false // 快速排除
    }
    p, ok := parent.Value(&cancelCtxKey).(*cancelCtx)
    if !ok {
        return nil, false
    }
    pdone, _ := p.done.Load().(chan struct{})
    if pdone != done {
        return nil, false // 核心校验
    }
    return p, true
}
```

这个函数的职责是：**沿着 Value 链找到父 context 底层真正的 `*cancelCtx`，并验证它的 `done` channel 是否可信。**

**快速排除**两种不可能挂载子 context 的情况：
- `done == nil` → parent 是 `Background()` / `TODO()`，根本没有取消能力
- `done == closedchan` → parent 已永久关闭

**核心校验**：父 context 的 `done` channel 和找到的 `cancelCtx` 的 `pdone` channel 是否一致。如果不一致，说明 `Done()` 被用户自定义实现"篡改"了，此时不信任它。

为什么需要这个校验？考虑以下场景：

```go
type myCtx struct {
    context.Context
}

func (m *myCtx) Done() <-chan struct{} {
    return myChannel // 返回了一个和底层 cancelCtx 不同的 channel
}
```

此时父 context cancel 时，关闭的是内部 `cancelCtx.done`，用户监听的是 `myChannel`——用户永远收不到取消信号！`parentCancelCtx` 通过 channel 一致性校验来防止这种不安全的情况。

### removeChild：从父节点摘除自己

```go
func removeChild(parent Context, child canceler) {
    if s, ok := parent.(stopCtx); ok {
        s.stop()
        return
    }
    p, ok := parentCancelCtx(parent)
    if !ok {
        return
    }
    p.mu.Lock()
    if p.children != nil {
        delete(p.children, child)
    }
    p.mu.Unlock()
}
```

当一个 context 被取消后，需要从父节点的 `children` map 中删除自己，避免内存泄漏。当 parent 是 `stopCtx` 时，调用 `stop()` 来注销回调函数（后文详述）。

---

## 6. cancel 方法：一次取消的完整流程

```go
func (c *cancelCtx) cancel(removeFromParent bool, err, cause error) {
    if err == nil {
        panic("context: internal error: missing cancel error")
    }
    if cause == nil {
        cause = err
    }
    c.mu.Lock()
    if c.err.Load() != nil {
        c.mu.Unlock()
        return // already canceled
    }

    // 1. 设置错误状态
    c.err.Store(err)
    c.cause = cause

    // 2. 关闭 done channel
    d, _ := c.done.Load().(chan struct{})
    if d == nil {
        c.done.Store(closedchan) // 懒初始化场景：直接指向全局已关闭 channel
    } else {
        close(d) // 关闭已有 channel，唤醒所有等待者
    }

    // 3. 级联取消所有子 context
    for child := range c.children {
        child.cancel(false, err, cause)
    }
    c.children = nil
    c.mu.Unlock() // 刻意不用 defer，缩小临界区

    // 4. 从父节点摘除自己
    if removeFromParent {
        removeChild(c.Context, c)
    }
}
```

执行流程清晰分为四步：

1. **幂等检查**：如果 `err` 已非 nil，说明已取消，直接返回
2. **设置状态**：写入 `err` 和 `cause`
3. **关闭 channel**：如果 `done` 是懒初始化（无人调用过 `Done()`），直接将 `done` 指向全局已关闭的 `closedchan`；否则关闭已有 channel
4. **级联 + 清理**：递归取消所有子 context，置空 `children` map，然后从父节点摘除

> **细节亮点**：级联调用时 `removeFromParent` 设为 `false`，因为 `children` 即将被置空，子 context 无需再反向从 map 中删除自己。末尾手动 `Unlock()` 而非 `defer` 是为了缩小临界区——后续的 `removeChild` 需要获取父节点的锁，如果持有时间过长会影响并发性能。

---

## 7. Cause：更丰富的取消原因

```go
func Cause(c Context) error {
    err := c.Err()
    if err == nil {
        return nil
    }
    if cc, ok := c.Value(&cancelCtxKey).(*cancelCtx); ok {
        cc.mu.Lock()
        cause := cc.cause
        cc.mu.Unlock()
        if cause != nil {
            return cause
        }
    }
    return err
}
```

`Cause()` 返回取消的**根因**，而非标准化的错误值。它优先返回 `cancelCtx` 中存储的 `cause`，如果没有则回退到 `Err()` 的结果。

为什么需要 `err` 和 `cause` 两个字段？

- **`err`**：向后兼容，永远返回标准的 `context.Canceled` 或 `context.DeadlineExceeded`
- **`cause`**：携带更丰富的诊断信息，供 `Cause()` 使用

---

## 8. WithoutCancel：切断取消信号的传播

```go
type withoutCancelCtx struct {
    c Context
}

func WithoutCancel(parent Context) Context {
    if parent == nil {
        panic("cannot create context from nil parent")
    }
    return withoutCancelCtx{parent}
}

func (withoutCancelCtx) Deadline() (deadline time.Time, ok bool) { return }
func (withoutCancelCtx) Done() <-chan struct{}                    { return nil }
func (withoutCancelCtx) Err() error                               { return nil }

func (c withoutCancelCtx) Value(key any) any {
    return value(c, key) // 透传 Value
}

func (c withoutCancelCtx) String() string {
    return contextName(c.c) + ".WithoutCancel"
}
```

`WithoutCancel` 创建一个派生 context，它**继承父 context 的 Value，但不继承取消信号**。父 context 被取消时，它完全不受影响。

> **重要认知**：在 Go 里，"取消"只是 context 的一个逻辑状态，不是对象的销毁。context 对象本身和普通 Go 对象一样——只要还有引用就活着，没有引用了就被 GC 回收。`Err()` 说"我取消了"，`Value()` 说"我的数据还在"，两者互不干扰。

```go
ctx, cancel := context.WithCancel(context.Background())
ctx = context.WithValue(ctx, "userID", "12345")

cancel()
ctx = nil // 没有引用了

// 此时整条链（valueCtx → cancelCtx → Background）都可以被 GC
```

---

## 9. valueCtx：值的链式传递

### WithValue

```go
func WithValue(parent Context, key, val any) Context {
    if parent == nil {
        panic("cannot create context from nil parent")
    }
    if key == nil {
        panic("nil key")
    }
    if !reflectlite.TypeOf(key).Comparable() {
        panic("key is not comparable")
    }
    return &valueCtx{parent, key, val}
}
```

每次 `WithValue` 都返回一个**新的** `valueCtx`，形成一条从叶子到根的单向链表。

### valueCtx 结构

```go
type valueCtx struct {
    Context
    key, val any
}

func (c *valueCtx) String() string {
    return contextName(c.Context) + ".WithValue(" +
        stringify(c.key) + ", " + stringify(c.val) + ")"
}

func (c *valueCtx) Value(key any) any {
    if c.key == key {
        return c.val
    }
    return value(c.Context, key) // 沿链向上查找
}
```

### value 函数：O(N) 链表遍历

```go
func value(c Context, key any) any {
    for {
        switch ctx := c.(type) {
        case *valueCtx:
            if key == ctx.key {
                return ctx.val
            }
            c = ctx.Context
        case *cancelCtx:
            if key == &cancelCtxKey {
                return c
            }
            c = ctx.Context
        case withoutCancelCtx:
            if key == &cancelCtxKey {
                return nil // WithoutCancel 创建的 context 没有 cancelCtx
            }
            c = ctx.c
        case *timerCtx:
            if key == &cancelCtxKey {
                return &ctx.cancelCtx
            }
            c = ctx.Context
        case backgroundCtx, todoCtx:
            return nil
        default:
            return c.Value(key) // 尊重用户自定义的 Value() 实现
        }
    }
}
```

`value` 函数针对标准库内部的各种 context 类型做了 `switch` 优化，避免递归调用 `Value()` 的开销。当遇到未知类型时，回退到通用的 `c.Value(key)` 调用。

> **为什么用链表而不用 map？** 不可变、无锁、轻量。每次 `WithValue` 只创建一个新节点，不需要复制整个 map。context.Value 不应该被滥用作参数传递，它只适合传 `traceID`、`userID` 这类少量数据。数据量少，O(N) 就不是问题。

---

## 10. 错误类型：Canceled 与 DeadlineExceeded

```go
var Canceled = errors.New("context canceled")

var DeadlineExceeded error = deadlineExceededError{}

type deadlineExceededError struct{}

func (deadlineExceededError) Error() string   { return "context deadline exceeded" }
func (deadlineExceededError) Timeout() bool   { return true }
func (deadlineExceededError) Temporary() bool { return true }
```

两类标准错误：

| 错误               | 含义               | 附加接口                            |
| ------------------ | ------------------ | ----------------------------------- |
| `Canceled`         | context 被主动取消 | 无                                  |
| `DeadlineExceeded` | context 超时       | 实现了 `Timeout()` 和 `Temporary()` |

`DeadlineExceeded` 实现了 `Timeout()` 和 `Temporary()` 方法，使得网络库可以通过类型断言判断是否为超时错误，并决定是否重试。

---

## 11. WithCancel 与 WithCancelCause

```go
func withCancel(parent Context) *cancelCtx {
    if parent == nil {
        panic("cannot create context from nil parent")
    }
    c := &cancelCtx{}
    c.propagateCancel(parent, c) // 将 c 注册到父 context 的 *cancelCtx 中
    return c
}
```

### WithCancel

```go
type CancelFunc func()

func WithCancel(parent Context) (ctx Context, cancel CancelFunc) {
    c := withCancel(parent)
    return c, func() { c.cancel(true, Canceled, nil) }
}
```

### WithCancelCause

```go
type CancelCauseFunc func(cause error)

func WithCancelCause(parent Context) (ctx Context, cancel CancelCauseFunc) {
    c := withCancel(parent)
    return c, func(cause error) { c.cancel(true, Canceled, cause) }
}
```

两者的区别仅在于取消时是否携带 `cause`。`WithCancel` 的 `CancelFunc` 不接受参数，`WithCancelCause` 的 `CancelCauseFunc` 接受一个 `cause error`，用于诊断。

---

## 12. timerCtx：超时的实现

```go
type timerCtx struct {
    cancelCtx
    timer *time.Timer // Under cancelCtx.mu.
    deadline time.Time
}

func (c *timerCtx) Deadline() (deadline time.Time, ok bool) {
    return c.deadline, true
}

func (c *timerCtx) String() string {
    return contextName(c.cancelCtx.Context) + ".WithDeadline(" +
        c.deadline.String() + " [" + time.Until(c.deadline).String() + "])"
}
```

`timerCtx` 在 `cancelCtx` 基础上增加了 `timer` 和 `deadline`，用于实现超时取消。

### timerCtx 的 cancel

```go
func (c *timerCtx) cancel(removeFromParent bool, err, cause error) {
    c.cancelCtx.cancel(false, err, cause) // 委托给 cancelCtx
    if removeFromParent {
        removeChild(c.cancelCtx.Context, c)
    }
    c.mu.Lock()
    if c.timer != nil {
        c.timer.Stop() // 提前终止定时器，防止到达 deadline 后再次 cancel
        c.timer = nil
    }
    c.mu.Unlock()
}
```

先委托给内嵌的 `cancelCtx.cancel`，再额外停止定时器。这确保了即使 context 被主动取消（而非超时），定时器也不会泄漏。

---

## 13. WithDeadline 与 WithTimeout

```go
func WithDeadline(parent Context, d time.Time) (Context, CancelFunc) {
    return WithDeadlineCause(parent, d, nil)
}
```

### WithDeadlineCause：核心实现

```go
func WithDeadlineCause(parent Context, d time.Time, cause error) (Context, CancelFunc) {
    if parent == nil {
        panic("cannot create context from nil parent")
    }
    if cur, ok := parent.Deadline(); ok && cur.Before(d) {
        // 父 context 的截止时间比新的更早，无需设置新定时器
        return WithCancel(parent)
    }
    c := &timerCtx{deadline: d}
    c.cancelCtx.propagateCancel(parent, c)
    dur := time.Until(d)
    if dur <= 0 {
        c.cancel(true, DeadlineExceeded, cause) // 已经过了截止时间
        return c, func() { c.cancel(false, Canceled, nil) }
    }
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.err.Load() == nil {
        c.timer = time.AfterFunc(dur, func() {
            c.cancel(true, DeadlineExceeded, cause)
        })
    }
    return c, func() { c.cancel(true, Canceled, nil) }
}
```

逻辑非常清晰：

1. **父 deadline 更早** → 退化为 `WithCancel`，不需要新定时器
2. **已超时** → 立即 cancel，返回的 cancel 函数做幂等操作
3. **未超时** → 启动 `time.AfterFunc`，到时间后自动 cancel

### WithTimeout 系列

```go
func WithTimeout(parent Context, timeout time.Duration) (Context, CancelFunc) {
    return WithDeadline(parent, time.Now().Add(timeout))
}

func WithTimeoutCause(parent Context, timeout time.Duration, cause error) (Context, CancelFunc) {
    return WithDeadlineCause(parent, time.Now().Add(timeout), cause)
}
```

`WithTimeout` 就是 `WithDeadline` 的便捷封装，将相对时间转换为绝对时间。

---

## 14. afterFuncCtx 与 AfterFunc

`AfterFunc` 允许你将一个函数关联到 context 上，当 context 被取消时触发执行。

### afterFuncCtx

```go
type afterFuncCtx struct {
    cancelCtx
    once sync.Once
    f    func() // 在 cancel 中触发，调用 stop 时不触发
}

func (a *afterFuncCtx) cancel(removeFromParent bool, err, cause error) {
    a.cancelCtx.cancel(false, err, cause)
    if removeFromParent {
        removeChild(a.Context, a)
    }
    a.once.Do(func() {
        go a.f() // 开协程执行，防止阻塞调用 cancel 的 goroutine
    })
}
```

`sync.Once` 确保 `f()` 只被执行一次。开新协程调用 `f()` 是为了防止在主动调用 `cancel()` 时被 `f()` 阻塞。

### AfterFunc

```go
func AfterFunc(ctx Context, f func()) (stop func() bool) {
    a := &afterFuncCtx{f: f}
    a.cancelCtx.propagateCancel(ctx, a)
    return func() bool {
        stopped := false
        a.once.Do(func() {
            stopped = true
        })
        if stopped {
            a.cancel(true, Canceled, nil) // 此时不会调用 a.f()
        }
        return stopped
    }
}
```

`AfterFunc` 返回一个 `stop` 函数：

- 调用 `stop()` 时，通过 `once.Do` 抢占执行权，将 `stopped` 设为 `true`，然后执行 `cancel`。由于 `once` 已被消费，后续即使 context 被取消，`f()` 也不会再执行。
- 如果 context 先被取消，`cancel` 中的 `once.Do` 会先执行 `f()`，此时再调用 `stop()` 返回 `false`。

---

## 15. propagateCancel：级联取消的核心算法

这是整个 context 包中最复杂的函数，负责将子 context 注册到父 context 的取消链中。

```go
func (c *cancelCtx) propagateCancel(parent Context, child canceler) {
    c.Context = parent
    done := parent.Done()

    // ── 情况一：parent 永不取消 ──
    if done == nil {
        return
    }

    // ── 情况一补充：parent 已取消 ──
    select {
    case <-done:
        child.cancel(false, parent.Err(), Cause(parent))
        return
    default:
    }

    // ── 情况二：parent 链中有可信的 *cancelCtx ──
    if p, ok := parentCancelCtx(parent); ok {
        p.mu.Lock()
        if err := p.err.Load(); err != nil {
            child.cancel(false, err.(error), p.cause)
        } else {
            if p.children == nil {
                p.children = make(map[canceler]struct{})
            }
            p.children[child] = struct{}{}
        }
        p.mu.Unlock()
        return
    }

    // ── 情况三：parent 实现了 afterFuncer ──
    if a, ok := parent.(afterFuncer); ok {
        c.mu.Lock()
        stop := a.AfterFunc(func() {
            child.cancel(false, parent.Err(), Cause(parent))
        })
        c.Context = stopCtx{Context: parent, stop: stop}
        c.mu.Unlock()
        return
    }

    // ── 情况四：兜底，起 goroutine 监听 ──
    goroutines.Add(1)
    go func() {
        select {
        case <-parent.Done():
            child.cancel(false, parent.Err(), Cause(parent))
        case <-child.Done():
        }
    }()
}
```

### 四种情况详解

**情况一：parent 永不取消**

`done == nil`，说明 parent 是 `Background()` / `TODO()` / `WithoutCancel()` 这类永远不会取消的 context，直接返回。

**情况一补充：parent 已被取消**

通过 `select` 非阻塞检查 `done` channel 是否已关闭。如果已关闭，立即取消 child。

**情况二：parent 链中有可信的 cancelCtx**

通过 `parentCancelCtx` 找到底层的 `*cancelCtx`，将 child 注册到它的 `children` map 中。如果注册时发现 parent 已被取消（并发场景），则立即取消 child。

**情况三：parent 实现了 afterFuncer**

当 parent 的取消源不在标准 context 树里（自定义类型），但实现了 `AfterFunc` 方法时，通过回调注册取消逻辑。此时 `c.Context` 被替换为 `stopCtx`，以便后续 `removeChild` 时调用 `stop()` 注销回调。

**情况四：兜底 goroutine**

以上都不满足时，启动一个 goroutine 同时监听 `parent.Done()` 和 `child.Done()`。当 parent 被取消时级联取消 child；当 child 先被取消时退出监听。

---

## 16. afterFuncer：自定义取消源的适配方案

`propagateCancel` 的情况三适用于这样的场景：**你的取消源不在标准 context 树里，但你希望下游能用标准 context API 派生和响应。** 如果取消源本身已经是 context，直接 `WithCancel` 即可，不需要自定义。

```go
type afterFuncContext struct {
    mu         sync.Mutex
    afterFuncs map[*byte]func() // 注册表：唯一指针做 key
    done       chan struct{}
    err        error
}

func (c *afterFuncContext) AfterFunc(f func()) func() bool {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.err != nil {
        // 已取消：契约要求立即（在自己 goroutine 里）执行
        c.mu.Unlock()
        go f()
        return func() bool { return false }
    }
    k := new(byte) // 每次注册一个唯一 key → 多次注册相互独立
    if c.afterFuncs == nil {
        c.afterFuncs = make(map[*byte]func())
    }
    c.afterFuncs[k] = f
    return func() bool {
        c.mu.Lock()
        defer c.mu.Unlock()
        _, ok := c.afterFuncs[k] // 已被取消(map=nil)或已 stop → ok=false
        delete(c.afterFuncs, k)
        return ok // true = 本次调用阻止了 f 运行
    }
}

func (c *afterFuncContext) cancel(err error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.err != nil {
        return
    }
    c.err = err
    for _, f := range c.afterFuncs {
        go f() // 关键：永远在自己的 goroutine 里调 f
    }
    c.afterFuncs = nil
}
```

这是一个完整的 `afterFuncer` 示例实现：

- **注册**：`AfterFunc` 将回调函数 `f` 存入 map，用 `new(byte)` 生成的唯一指针做 key
- **注销**：返回的闭包调用 `delete` 从 map 中移除回调
- **触发**：`cancel` 时遍历所有已注册的回调，每个都在独立的 goroutine 中执行
- **幂等**：`c.afterFuncs = nil` 后，后续的 `AfterFunc` 调用会因为 `c.err != nil` 而立即执行 `f`

---

## 总结

Go 的 `context` 包用 800 多行代码，实现了一套精巧的并发控制机制：

| 设计决策                       | 原因                                         |
| ------------------------------ | -------------------------------------------- |
| `done` channel 懒初始化        | 大多数 context 不会被监听 `Done()`，节省内存 |
| 双重检查锁定                   | 无锁快路径 + 安全初始化                      |
| channel 关闭而非发送           | 广播能力、零内存、不可逆、不阻塞             |
| Value 用链表而非 map           | 不可变、无锁、轻量                           |
| `err` + `cause` 双字段         | 向后兼容 + 丰富诊断                          |
| `propagateCancel` 四种情况     | 覆盖标准树、自定义类型、兜底场景             |
| `parentCancelCtx` channel 校验 | 防止用户自定义 `Done()` 导致信号丢失         |

理解这些设计背后的权衡，不仅能帮助我们正确使用 context，更能学到在并发场景下如何用最少的原语实现最大的能力。
