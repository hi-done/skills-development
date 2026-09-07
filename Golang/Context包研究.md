# 深入理解 Go Context 包：从源码到实战

> Context 是 Go 并发编程的基石之一。本文将从源码出发，系统剖析 `context` 包的设计思想、内部实现与最佳实践，帮助你真正理解并正确使用它。

---

## 目录

- [一、为什么需要 Context](#一为什么需要-context)
- [二、Context 接口定义](#二context-接口定义)
- [三、根 Context：Background 与 TODO](#三根-contextbackground-与-todo)
- [四、取消机制：cancelCtx 深度解析](#四取消机制cancelctx-深度解析)
  - [4.1 WithCancel 的使用](#41-withcancel-的使用)
  - [4.2 cancelCtx 核心结构](#42-cancelctx-核心结构)
  - [4.3 Done() 与懒初始化](#43-done-与懒初始化)
  - [4.4 cancel() 级联取消](#44-cancel-级联取消)
  - [4.5 propagateCancel：父子绑定](#45-propagatecancelfrom子绑定)
  - [4.6 WithCancelCause：携带取消原因](#46-withcancelcause携带取消原因)
- [五、超时机制：timerCtx 深度解析](#五超时机制timerctx-深度解析)
  - [5.1 WithTimeout / WithDeadline](#51-withtimeout--withdeadline)
  - [5.2 timerCtx 结构](#52-timerctx-结构)
- [六、值传递：valueCtx](#六值传递valuectx)
- [七、WithoutCancel：断开取消链](#七withoutcancel断开取消链)
- [八、AfterFunc：取消后回调](#八afterfunc取消后回调)
- [九、实战示例](#九实战示例)
  - [9.1 HTTP 请求超时控制](#91-http-请求超时控制)
  - [9.2 多协程并发取消](#92-多协程并发取消)
  - [9.3 值传递最佳实践](#93-值传递最佳实践)
- [十、最佳实践与常见陷阱](#十最佳实践与常见陷阱)
- [十一、总结](#十一总结)

---

## 一、为什么需要 Context

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

---

## 二、Context 接口定义

```go
type Context interface {
    // Deadline 返回 context 的截止时间
    // ok == false 表示没有设置截止时间
    Deadline() (deadline time.Time, ok bool)

    // Done 返回一个只读 channel，当 context 被取消或超时时关闭
    // 返回 nil 表示永远不会被取消（如 Background）
    Done() <-chan struct{}

    // Err 返回 context 被关闭的原因
    // 可能的返回值：Canceled 或 DeadlineExceeded
    Err() error

    // Value 获取与 key 关联的请求范围的值
    Value(key any) any
}
```

四个方法各司其职，构成了 context 的全部能力：

| 方法 | 职责 | 典型用途 |
|------|------|---------|
| `Deadline()` | 获取截止时间 | 数据库连接设置 query timeout |
| `Done()` | 监听取消信号 | `select { case <-ctx.Done(): }` |
| `Err()` | 获取取消原因 | 日志记录、错误处理 |
| `Value()` | 获取请求级数据 | 传递 traceID、userID |

---

## 三、根 Context：Background 与 TODO

所有 context 树的根基是 `emptyCtx`，它什么也不做——没有截止时间、不能被取消、不携带任何值。

```go
type emptyCtx struct{}

func (emptyCtx) Deadline() (deadline time.Time, ok bool) { return }
func (emptyCtx) Done() <-chan struct{}                    { return nil } // 没有取消能力
func (emptyCtx) Err() error                               { return nil }
func (emptyCtx) Value(key any) any                        { return nil }
```

在此基础上，标准库派生出两个根 context：

```go
type backgroundCtx struct{ emptyCtx }

func (backgroundCtx) String() string { return "context.Background" }

func Background() Context { return backgroundCtx{} }
```

```go
type todoCtx struct{ emptyCtx }

func (todoCtx) String() string { return "context.TODO" }

func TODO() Context { return todoCtx{} }
```

**两者的区别是语义上的：**

| 场景 | 使用 |
|------|------|
| `main` 函数、顶层入口、测试入口 | `context.Background()` |
| 重构过渡期、不确定该用什么 context | `context.TODO()` |

> `TODO` 本质上是一个标记——"这里将来需要替换"，方便代码审查和 `grep` 搜索。

---

## 四、取消机制：cancelCtx 深度解析

取消（cancellation）是 context 最核心的能力。其设计核心可以概括为一句话：

> **"取消"是一个状态，不是一个事件。通过关闭 channel 广播，而非向 channel 发送值。**

为什么选择「关闭 channel」而不是「发送一个值」？

- 关闭 channel 可以**广播**给所有监听者；发送值只能唤醒一个 goroutine
- 关闭是**不可逆**的，语义上「取消只能发生一次」正好匹配
- 不需要 buffer，不消耗额外内存
- 不会阻塞发送者

### 4.1 WithCancel 的使用

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel() // 养成习惯：永远 defer cancel，防止泄漏

go func(ctx context.Context) {
    select {
    case <-ctx.Done():
        fmt.Println("被取消了:", ctx.Err())
    case result := <-doWork():
        fmt.Println("完成:", result)
    }
}(ctx)
```

### 4.2 cancelCtx 核心结构

```go
var cancelCtxKey int // 将 &cancelCtxKey 作为查找 cancelCtx 自身的特殊 Key

type cancelCtx struct {
    Context                          // 指向父 context

    mu       sync.Mutex              // 保护以下字段
    done     atomic.Value            // chan struct{}，懒初始化，被 cancel 时关闭
    children map[canceler]struct{}   // 子 context 集合，cancel 后置为 nil
    err      atomic.Value            // 第一个 cancel 调用设置为非 nil
    cause    error                   // 第一个 cancel 调用设置的根因
}
```

> **`atomic.Value` + `sync.Mutex` 的组合**：`done` 和 `err` 使用原子操作实现无锁快路径读取，`mu` 仅在需要修改时加锁。原子读比互斥锁快约 5 倍，这在热路径上非常关键。

### 4.3 Done() 与懒初始化

`Done()` 方法使用了经典的**双重检查锁定（Double-Checked Locking）**模式：

```go
func (c *cancelCtx) Done() <-chan struct{} {
    d := c.done.Load() // 无锁快路径：如果已创建，直接返回
    if d != nil {
        return d.(chan struct{})
    }
    // 慢路径：加锁创建
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

**为什么要懒初始化？** 很多 context 在其生命周期中根本不会有人调用 `Done()`，提前创建 channel 只会造成无意义的内存开销。

`Err()` 方法同样利用原子读实现快速路径：

```go
func (c *cancelCtx) Err() error {
    if err := c.err.Load(); err != nil {
        // 行为一致性保证：只要 Err() 返回非 nil，
        // 所有 select { case <-ctx.Done(): } 一定能退出
        <-c.Done() // 等待 channel 关闭
        return err.(error)
    }
    return nil
}
```

### 4.4 cancel() 级联取消

`cancel()` 是整个取消机制的执行核心：

```go
func (c *cancelCtx) cancel(removeFromParent bool, err, cause error) {
    if err == nil {
        panic("context: internal error: missing cancel error")
    }
    if cause == nil {
        cause = err
    }
    c.mu.Lock()

    // 幂等性检查：已取消则直接返回
    if c.err.Load() != nil {
        c.mu.Unlock()
        return
    }

    // 1. 记录错误和原因
    c.err.Store(err)
    c.cause = cause

    // 2. 关闭 done channel（广播取消信号）
    d, _ := c.done.Load().(chan struct{})
    if d == nil {
        // 无人调用过 Done()，设置为全局已关闭 channel
        // 后续调用 Done() 时能立即感知"已取消"
        c.done.Store(closedchan)
    } else {
        close(d) // 关闭已有 channel，唤醒所有等待者
    }

    // 3. 级联取消所有子 context
    for child := range c.children {
        // 递归调用，removeFromParent=false 因为 children 马上被清空
        child.cancel(false, err, cause)
    }
    c.children = nil

    c.mu.Unlock() // 刻意不用 defer，缩小临界区

    // 4. 从父 context 中移除自身
    if removeFromParent {
        removeChild(c.Context, c)
    }
}
```

**全局已关闭 channel `closedchan`** 在包初始化时创建：

```go
var closedchan = make(chan struct{})

func init() {
    close(closedchan)
}
```

### 4.5 propagateCancel：父子绑定

当创建新的 cancelCtx 时，需要将其注册到父 context 的取消链中：

```go
func (c *cancelCtx) propagateCancel(parent Context, child canceler) {
    c.Context = parent

    done := parent.Done()
    if done == nil {
        return // parent 永远不会取消（Background/TODO）
    }

    select {
    case <-done:
        // parent 已经被取消，立即取消 child
        child.cancel(false, parent.Err(), Cause(parent))
        return
    default:
    }

    // 尝试找到父 context 底层的 *cancelCtx 并注册
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

    // 兜底方案：起一个 goroutine 监听双方的 Done() channel
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

**`propagateCancel` 的三条路径：**

```
propagateCancel(parent, child)
    │
    ├── parent.Done() == nil → 无需注册（永不取消）
    │
    ├── parent 已取消 → 立即取消 child
    │
    ├── 找到父 *cancelCtx → 注册到 children map
    │
    ├── parent 实现 afterFuncer → 用 AfterFunc 注册回调
    │
    └── 兜底 → 起 goroutine 监听双方 Done()
```

### 4.6 parentCancelCtx：安全级联的关键

`parentCancelCtx` 负责从父 context 中找出「真正可安全挂载子节点的 *cancelCtx」，同时防止绕过用户自定义的 `Done()` 实现：

```go
func parentCancelCtx(parent Context) (*cancelCtx, bool) {
    done := parent.Done()
    if done == closedchan || done == nil {
        return nil, false
    }
    // 快速排除：done == nil → Background/TODO；done == closedchan → 已永久关闭

    p, ok := parent.Value(&cancelCtxKey).(*cancelCtx)
    if !ok {
        return nil, false
    }

    // 核心校验：父 context 的 done channel 必须与找到的 cancelCtx 的一致
    pdone, _ := p.done.Load().(chan struct{})
    if pdone != done {
        return nil, false
    }
    // 不一致 → 被包装/篡改过 → 不信任 → 返回 false
    return p, true
}
```

### 4.7 removeChild：从父节点摘除

```go
func removeChild(parent Context, child canceler) {
    if s, ok := parent.(stopCtx); ok {
        s.stop() // stopCtx 是逃生通道，直接操作内部 cancelCtx
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

### 4.8 WithCancelCause：携带取消原因

```go
type CancelCauseFunc func(cause error)

func WithCancelCause(parent Context) (ctx Context, cancel CancelCauseFunc) {
    c := withCancel(parent)
    return c, func(cause error) { c.cancel(true, Canceled, cause) }
}
```

为什么需要 `err` 和 `cause` 两个字段？

| 字段 | 用途 | 示例 |
|------|------|------|
| `err` | 向后兼容，永远返回 `context.Canceled` | `ctx.Err() == context.Canceled` |
| `cause` | 诊断用，携带丰富根因 | `context.Cause(ctx)` 返回 `"database connection lost"` |

`Cause()` 函数用于获取取消的真实原因：

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

---

## 五、超时机制：timerCtx 深度解析

### 5.1 WithTimeout / WithDeadline

```go
func WithTimeout(parent Context, timeout time.Duration) (Context, CancelFunc) {
    return WithDeadline(parent, time.Now().Add(timeout))
}

func WithDeadline(parent Context, d time.Time) (Context, CancelFunc) {
    return WithDeadlineCause(parent, d, nil)
}
```

`WithDeadlineCause` 是超时机制的核心：

```go
func WithDeadlineCause(parent Context, d time.Time, cause error) (Context, CancelFunc) {
    if parent == nil {
        panic("cannot create context from nil parent")
    }
    // 如果父 context 的截止时间更早，则直接使用 WithCancel
    if cur, ok := parent.Deadline(); ok && cur.Before(d) {
        return WithCancel(parent)
    }

    c := &timerCtx{deadline: d}
    c.cancelCtx.propagateCancel(parent, c)

    dur := time.Until(d)
    if dur <= 0 {
        c.cancel(true, DeadlineExceeded, cause) // 已过期，立即取消
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

**关键行为**：如果父 context 的 deadline 比新设置的更早，则新设置的 deadline 无效——取"更紧"的那个。

### 5.2 timerCtx 结构

```go
type timerCtx struct {
    cancelCtx                    // 内嵌 cancelCtx，复用取消能力
    timer    *time.Timer         // 在 cancelCtx.mu 保护下
    deadline time.Time
}

func (c *timerCtx) Deadline() (deadline time.Time, ok bool) {
    return c.deadline, true
}

// cancel 时需要额外停掉定时器，防止到期后再次触发 cancel
func (c *timerCtx) cancel(removeFromParent bool, err, cause error) {
    c.cancelCtx.cancel(false, err, cause)
    if removeFromParent {
        removeChild(c.cancelCtx.Context, c)
    }
    c.mu.Lock()
    if c.timer != nil {
        c.timer.Stop()
        c.timer = nil
    }
    c.mu.Unlock()
}
```

---

## 六、值传递：valueCtx

`WithValue` 在 context 链上追加一个 key-value 对，查询时从叶子向根逐层遍历：

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

type valueCtx struct {
    Context
    key, val any
}

func (c *valueCtx) Value(key any) any {
    if c.key == key {
        return c.val
    }
    return value(c.Context, key) // 递归向上查找
}
```

**为什么用链表而不是 map？**

| 特性 | 链表（实际方案） | map |
|------|----------------|-----|
| 不可变性 | 天然支持 | 需要 copy |
| 并发安全 | 无锁 | 需要加锁 |
| 内存开销 | 每个节点仅 2 个字段 | map 初始化开销大 |
| 查找复杂度 | O(N) | O(1) |

> O(N) 遍历在绝大多数场景下不是问题——context.Value 只适合传递少量请求级数据（如 traceID、userID），不应该被滥用为通用参数传递。

`value()` 函数对各类 context 做了针对性优化，避免每次都走 `c.Value(key)` 的虚方法调用：

```go
func value(c Context, key any) any {
    for {
        switch ctx := c.(type) {
        case *valueCtx:
            if key == ctx.key { return ctx.val }
            c = ctx.Context
        case *cancelCtx:
            if key == &cancelCtxKey { return c }
            c = ctx.Context
        case withoutCancelCtx:
            if key == &cancelCtxKey { return nil }
            c = ctx.c
        case *timerCtx:
            if key == &cancelCtxKey { return &ctx.cancelCtx }
            c = ctx.Context
        case backgroundCtx, todoCtx:
            return nil
        default:
            return c.Value(key) // 尊重用户自定义实现
        }
    }
}
```

---

## 七、WithoutCancel：断开取消链

Go 1.21 引入的 `WithoutCancel` 创建一个继承父 context 值但**不受父取消影响**的新 context。典型用途：在请求处理完成后，用原始 context 的值去执行后台清理任务（如写日志、发通知）。

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

func (withoutCancelCtx) Deadline() (deadline time.Time, ok bool) { return }  // 无截止时间
func (withoutCancelCtx) Done() <-chan struct{}                   { return nil } // 永不触发
func (withoutCancelCtx) Err() error                              { return nil } // 永不报错

func (c withoutCancelCtx) Value(key any) any {
    return value(c, key) // 透传 Value 到父 context
}

func (c withoutCancelCtx) String() string {
    return contextName(c.c) + ".WithoutCancel"
}
```

**注意**：`withoutCancelCtx` 没有截止时间，也没有取消能力——如果滥用，可能导致 goroutine 泄漏。

---

## 八、AfterFunc：取消后回调

Go 1.21 引入的 `AfterFunc` 在 context 被取消后异步执行一个回调函数：

```go
type afterFuncer interface {
    AfterFunc(func()) func() bool
}

type afterFuncCtx struct {
    cancelCtx
    once sync.Once // 保证 f 只执行一次
    f    func()
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

func AfterFunc(ctx Context, f func()) (stop func() bool) {
    a := &afterFuncCtx{f: f}
    a.cancelCtx.propagateCancel(ctx, a)
    return func() bool {
        stopped := false
        a.once.Do(func() { stopped = true })
        if stopped {
            a.cancel(true, Canceled, nil)
        }
        return stopped
    }
}
```

`AfterFunc` 适用于需要在取消后做清理工作的场景，如释放资源、关闭连接等。

---

## 九、实战示例

### 9.1 HTTP 请求超时控制

```go
func handler(w http.ResponseWriter, r *http.Request) {
    // 使用请求自带的 context（请求结束时自动取消）
    ctx := r.Context()

    // 为数据库查询设置 3 秒超时
    queryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
    defer cancel()

    result, err := db.QueryContext(queryCtx, "SELECT * FROM users WHERE id = ?", userID)
    if err != nil {
        if errors.Is(err, context.DeadlineExceeded) {
            http.Error(w, "查询超时", http.StatusGatewayTimeout)
            return
        }
        http.Error(w, "查询失败", http.StatusInternalServerError)
        return
    }
    // 使用 result...
}
```

### 9.2 多协程并发取消

```go
func fetchAll(ctx context.Context, urls []string) ([]string, error) {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel() // 任一失败时，取消其余所有

    results := make([]string, len(urls))
    errc := make(chan error, len(urls))

    for i, url := range urls {
        go func(i int, url string) {
            req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
            resp, err := http.DefaultClient.Do(req)
            if err != nil {
                errc <- err
                return
            }
            defer resp.Body.Close()
            body, _ := io.ReadAll(resp.Body)
            results[i] = string(body)
            errc <- nil
        }(i, url)
    }

    for range urls {
        if err := <-errc; err != nil {
            return nil, err // defer cancel() 会取消剩余请求
        }
    }
    return results, nil
}
```

### 9.3 值传递最佳实践

```go
// 用自定义类型做 key，避免与其他包冲突
type contextKey string

const (
    traceIDKey contextKey = "traceID"
    userIDKey  contextKey = "userID"
)

func WithTraceID(ctx context.Context, traceID string) context.Context {
    return context.WithValue(ctx, traceIDKey, traceID)
}

func TraceIDFromContext(ctx context.Context) string {
    v, _ := ctx.Value(traceIDKey).(string)
    return v
}

// 在中间件中使用
func TracingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        traceID := r.Header.Get("X-Trace-ID")
        if traceID == "" {
            traceID = uuid.New().String()
        }
        ctx := WithTraceID(r.Context(), traceID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

---

## 十、最佳实践与常见陷阱

### 最佳实践

1. **Context 作为第一个参数**：遵循 `func DoSomething(ctx context.Context, arg1, arg2 string)` 的签名约定。
2. **永远 defer cancel()**：忘记调用 `cancel` 会导致 goroutine 泄漏和 context 树无限增长。
3. **优先使用 `http.Request.Context()`**：HTTP handler 中不要新建根 context，应基于请求 context 派生。
4. **超时取最小值**：`WithDeadline` 会自动取父/子中更早的截止时间，无需手动判断。
5. **用自定义类型做 key**：避免 `string` 类型 key 与其他包冲突。

### 常见陷阱

| 陷阱 | 说明 |
|------|------|
| 将 context 存入 struct | context 是请求级的，应该是函数参数而非结构体字段 |
| 传递 nil context | 应使用 `context.TODO()` 代替 |
| 滥用 Value 传参 | 仅用于 traceID、userID 等少量请求级数据 |
| cancel 后继续使用 ctx | `ctx.Err()` 返回非 nil 后应尽快退出 |
| 不处理 `DeadlineExceeded` | 超时错误需要被显式识别和处理 |

---

## 十一、总结

```
┌──────────────────────────────────────────────────────┐
│                    Context 体系全景                    │
├──────────────────────────────────────────────────────┤
│                                                      │
│  emptyCtx (根基)                                     │
│    ├── backgroundCtx  → Background()                 │
│    └── todoCtx        → TODO()                       │
│                                                      │
│  cancelCtx (取消)                                    │
│    ├── WithCancel()                                  │
│    ├── WithCancelCause()                             │
│    └── timerCtx (超时)                               │
│         ├── WithTimeout()                            │
│         └── WithDeadline()                           │
│                                                      │
│  valueCtx (值传递)                                   │
│    └── WithValue()                                   │
│                                                      │
│  withoutCancelCtx (断开取消链)                        │
│    └── WithoutCancel()                               │
│                                                      │
│  afterFuncCtx (取消回调)                              │
│    └── AfterFunc()                                   │
│                                                      │
└──────────────────────────────────────────────────────┘
```

**核心设计原则：**

1. **树形不可变结构**：每个派生 context 只追加一层，父节点取消时自动级联。
2. **关闭 channel 广播取消**：一次关闭，所有监听者同时感知，不可逆。
3. **懒初始化**：`done` channel 只在首次调用 `Done()` 时创建，减少内存开销。
4. **原子操作 + 互斥锁**：读路径用原子操作实现无锁快路径，写路径用互斥锁保证正确性。
5. **Value 链表遍历**：用不可变的链表代替 map，实现无锁、不可变的值传递。

> Context 不是一个"功能丰富"的包，它是一个"恰到好处"的包——用最少的抽象解决了并发编程中最普遍的问题。理解它的源码，不仅能帮助你写出更好的 Go 代码，更能体会到什么是优秀的接口设计。
