```go
type Context interface {
	Deadline() (deadline time.Time, ok bool) // 返回 context 的截止时间（如果有）
	Done() <-chan struct{} // 返回一个只读 channel，当 context 被取消或超时时关闭
	Err() error // 返回 context 被关闭的原因（Canceled 或 DeadlineExceeded）
	Value(key any) any // 获取与 key 关联的请求范围的值
}
```

```go
type emptyCtx struct{}

func (emptyCtx) Deadline() (deadline time.Time, ok bool) {
	return
}

func (emptyCtx) Done() <-chan struct{} {
	return nil // 没有取消能力
}

func (emptyCtx) Err() error {
	return nil
}

func (emptyCtx) Value(key any) any {
	return nil
}
```

```go
type backgroundCtx struct{ emptyCtx }

func (backgroundCtx) String() string {
	return "context.Background"
}

func Background() Context {
	return backgroundCtx{}
}
```

```go
type todoCtx struct{ emptyCtx }

func (todoCtx) String() string {
	return "context.TODO"
}

func TODO() Context {
	return todoCtx{}
}
```

```go
type stringer interface {
	String() string
}

// 返回context的Name，检查有没有实现String()方法，否则返回Type类型名
func contextName(c Context) string {
	if s, ok := c.(stringer); ok {
		return s.String()
	}
	return reflectlite.TypeOf(c).String()
}
```

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

```go
var cancelCtxKey int // 将&cancelCtxKey作为返回cancelCtx自身的Key

type cancelCtx struct {
	Context

	mu       sync.Mutex            // protects following fields
	done     atomic.Value          // of chan struct{}, created lazily, closed by first cancel call
	children map[canceler]struct{} // set to nil by the first cancel call
	err      atomic.Value          // set to non-nil by the first cancel call
	cause    error                 // set to non-nil by the first cancel call
}

// 返回cancelCtx中key对应的值
func (c *cancelCtx) Value(key any) any {
	if key == &cancelCtxKey {
		return c
	}
	return value(c.Context, key)
}

// 返回 c.done 的 只读channel 类型，通过其是否关闭判断context是否被cancel
func (c *cancelCtx) Done() <-chan struct{} {
	d := c.done.Load() // 原子性，避免读到半初始化的对象与指令重排导致的问题
	if d != nil {
		return d.(chan struct{})
	} // 典型的「双重检查（Double-Check）+ 无锁快路径」优化，用来避免每次调用 Done() 都加锁 
    // Double-Checked Locking（双重检查锁定）
	c.mu.Lock()
	defer c.mu.Unlock()
	d = c.done.Load()
	if d == nil {
		d = make(chan struct{})
		c.done.Store(d) // 懒初始化（lazy initialization）
        // 很多 context 在其生命周期中根本不会有人调用 Done()，提前创建 channel 只会造成无意义的内存开销。
	}
	return d.(chan struct{})
}

func (c *cancelCtx) Err() error {
	// An atomic load is ~5x faster than a mutex, which can matter in tight loops.
	if err := c.err.Load(); err != nil {
		// Ensure the done channel has been closed before returning a non-nil error.
		<-c.Done() // 行为一致性高于实现便利，只要Err()返回错误，所有 select { case <-ctx.Done(): } 一定能退出
        // channel 已关闭 立即返回零值 struct{}{}
        // 选择“关闭 channel”而不是“发一个值”，因为后者只能唤醒一个 goroutine，无法广播；	
        // “取消”是状态，不是事件，语义不对；并且前者不消耗内存（不用 buffer），不可逆转（cancel 只能一次），不阻塞发送者
		return err.(error)
	}
	return nil
}

// 返回 Context名+".WithCancel"后缀
func (c *cancelCtx) String() string {
	return contextName(c.Context) + ".WithCancel"
}
```

```go
var closedchan = make(chan struct{}) // 全局已关闭 channel，复用，用于判断context是不是关闭的
// 所以初始化包就关闭之
func init() {
	close(closedchan)
}

// 从父 context 中找出「真正的、可安全挂载 子cancelCtx 的那个底层 *cancelCtx」，同时防止绕过用户自定义的 Done() 实现。
// 从而实现安全级联cancel
func parentCancelCtx(parent Context) (*cancelCtx, bool) {
	done := parent.Done() // 先拿到父 context 的 done channel
	if done == closedchan || done == nil {
		return nil, false
	} // 快速排除不可能挂载 子 cancelCtx 的情况
    // done == nil -> parent 是 Background() / TODO()，根本没有取消能力
    // done == closedchan -> parent 已经永久关闭（比如已经被 cancel 且 done 被替换成全局已关闭 channel）
	p, ok := parent.Value(&cancelCtxKey).(*cancelCtx) // 从 parent 的 Value 链里找 *cancelCtx
	if !ok {
		return nil, false
	}
	pdone, _ := p.done.Load().(chan struct{})
	if pdone != done {
		return nil, false
	} // 核心校验：父 context 的 done channel 和找到的 cancelCtx 的 pdone channel是否一致
    // 如果不一致 → 说明被包装/篡改过(不安全) → 不信任它 → 返回 false，例如：
    // type myCtx struct {
    //  context.Context
    // }
    // 用户自定义的 Done() 实现
    // func (m *myCtx) Done() <-chan struct{} {
    // // 返回了一个和底层 cancelCtx 不同的 channel
    // return myChannel
    // }
    // 此时 父 context cancel 时，关闭的是内部 cancelCtx.done，用户监听的是 myChannel
    // 用户永远收不到取消信号！
    
	return p, true // 校验通过，返回真正的父 cancelCtx
}

// 从 父 Context 的底层 *cancelCtx 的列表中 删除 自身
func removeChild(parent Context, child canceler) {
	if s, ok := parent.(stopCtx); ok {
		s.stop() // 取消所有子节点，并清空 children
        // stopCtx 是一个“绕过 parentCancelCtx 校验、直接操作内部 cancelCtx”的逃生通道
        // 更底层、更直接、不依赖 Done() 一致性
        // 只要父 context 在内部实现了 stop()，就直接用最快的方式断开关系，不需要走 Value 查找 + 一致性校验
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

```go
// cancelCtx 用于自身 cancel，将 done channel 关闭，
// 级联关闭子 context， 
// 从 父 Context 的底层 *cancelCtx 的列表中 删除 自
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
	} // cancelCtx 的 自身err不为空，代表已被 cancel
	c.err.Store(err)
	c.cause = cause
	d, _ := c.done.Load().(chan struct{})
	if d == nil {
        // done channel 是懒初始化的，如果无人调用过Done()，则 d == nil
		c.done.Store(closedchan)
        // 而 cancel 之后，如果有人第一次调用 Done()，它应该立即感知到"已关闭"，故设置为一个全局已关闭的 channel
        // 否则 Done() 会新建一个未关闭的 channel，调用方不能 判断其已被cancel
	} else {
		close(d) // 关闭已有的 channel，唤醒所有等待者
	}
	for child := range c.children {
		// NOTE: acquiring the child's lock while holding parent's lock.
		child.cancel(false, err, cause)
        // 参数removeFromParent，当自身cancel时，children即将置空，
        // 故内部递归调用，以级联cancel子context时，设为false，不用再返回来从children中删除
	}
	c.children = nil
	c.mu.Unlock() // 刻意不用 defer 来缩小临界区
    // 否则执行后续的removeChild 虽然不会死锁（不同对象），但锁的持有时间被不必要地拉长了
	if removeFromParent {
		removeChild(c.Context, c) // 从 父 Context 的底层 *cancelCtx 的列表中 删除 自身
	}
}
```

```go
// 返回 context 的 cause，如果未被cancel，则为nil
// 为 父 cancelCtx 的 cause 或 自生 err
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
		// The parent cancelCtx doesn't have a cause,
		// so c must have been canceled in some custom context implementation.
	}
	// We don't have a cause to return from a parent cancelCtx,
	// so return the context's error.
	return err
}
```

```go
// 
type withoutCancelCtx struct {
	c Context
}

// 创建一个派生 context，它继承父 context 的 Value，但父 context 被取消时，它不会被取消
func WithoutCancel(parent Context) Context {
	if parent == nil {
		panic("cannot create context from nil parent")
	}
	return withoutCancelCtx{parent}
}

func (withoutCancelCtx) Deadline() (deadline time.Time, ok bool) {
	return  // 无截止时间
}

func (withoutCancelCtx) Done() <-chan struct{} {
	return nil  // 永远不会触发
}

func (withoutCancelCtx) Err() error {
	return nil  // 永远不报错
}

func (c withoutCancelCtx) Value(key any) any {
	return value(c, key)    // 透传 Value
}

func (c withoutCancelCtx) String() string {
	return contextName(c.c) + ".WithoutCancel"
}
```
在 Go 里，"取消"只是 context 的一个逻辑状态，不是对象的销毁。context 对象本身和普通 Go 对象一样：只要还有引用 → 就活着；没有引用了 → GC 回收。
Err() 说"我取消了"，Value() 说"我的数据还在"。两者互不干扰。
当 没有任何人引用这条 context 链​ 时，Value 才真正"消失"：
```go
ctx, cancel := context.WithCancel(context.Background())
ctx = context.WithValue(ctx, "userID", "12345")

cancel()
ctx = nil  // 没有引用了

// 此时整条链（valueCtx → cancelCtx → Background）都可以被 GC
```

```go
// 每次 WithValue 返回新 context。
// context.Value 不应该被滥用作参数传递，它只适合传 traceID、userID 这类少量数据。量少，O(N) 就不是问题。
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

func stringify(v any) string {
	switch s := v.(type) {
	case stringer:
		return s.String()
	case string:
		return s
	case nil:
		return "<nil>"
	}
	return reflectlite.TypeOf(v).String() // 只输出类型名，既安全又轻量
}

func (c *valueCtx) String() string {
	return contextName(c.Context) + ".WithValue(" +
		stringify(c.key) + ", " +
		stringify(c.val) + ")"
}

func (c *valueCtx) Value(key any) any {
	if c.key == key {
		return c.val
	}
	return value(c.Context, key)
}

// O(N) 链表遍历，不用 map 是为了不可变、无锁、轻量
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
				// This implements Cause(ctx) == nil
				// when ctx is created using WithoutCancel.
				return nil
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
			return c.Value(key)
            // 	当 context 链中出现了标准库没覆盖的用户自定义 context 类型时
            // 尊重用户自定义的 Value() 实现，不跳过
		}
	}
}
```
两类Err()，被取消、超时
```go
var Canceled = errors.New("context canceled")

var DeadlineExceeded error = deadlineExceededError{}

type deadlineExceededError struct{}

func (deadlineExceededError) Error() string   { return "context deadline exceeded" }
func (deadlineExceededError) Timeout() bool   { return true }
func (deadlineExceededError) Temporary() bool { return true }
```

```go
func withCancel(parent Context) *cancelCtx {
	if parent == nil {
		panic("cannot create context from nil parent")
	}
	c := &cancelCtx{}
	c.propagateCancel(parent, c) // 将 c注册到 父 context 的 *cancelCtx 中
	return c
}

type CancelFunc func()

// 将 c注册到 父 context 的 *cancelCtx 中 返回 c 和 cancel 方法
func WithCancel(parent Context) (ctx Context, cancel CancelFunc) {
	c := withCancel(parent)
	return c, func() { c.cancel(true, Canceled, nil) }
}

type CancelCauseFunc func(cause error)

// 取消并携带cause
// err 是给用户看的"取消原因"（向后兼容），
// cause 是给开发者/库用的"根因"（诊断用）。
// 两者可能不同，所以需要分开存。
func WithCancelCause(parent Context) (ctx Context, cancel CancelCauseFunc) {
	c := withCancel(parent)
	return c, func(cause error) { c.cancel(true, Canceled, cause) }
}
```
1、不只用一个 err 为了向后兼容，永远返回标准错误`context.Canceled` 或 `context.DeadlineExceeded`
2、Cause() 可以携带更丰富的诊断信息

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
		c.deadline.String() + " [" +
		time.Until(c.deadline).String() + "])"
}

// 在cancel后要停掉定时器，防止到达deadline后再次调用cancel
func (c *timerCtx) cancel(removeFromParent bool, err, cause error) {
	c.cancelCtx.cancel(false, err, cause)
	if removeFromParent {
		// Remove this timerCtx from its parent cancelCtx's children.
		removeChild(c.cancelCtx.Context, c)
	}
	c.mu.Lock()
	if c.timer != nil {
		c.timer.Stop() // 提前终止异步执行的AfterFunc，见WithDeadlineCause
		c.timer = nil
	}
	c.mu.Unlock()
}
```

```go
func WithDeadline(parent Context, d time.Time) (Context, CancelFunc) {
	return WithDeadlineCause(parent, d, nil)
}

// 将 c注册到 父 context 的 *cancelCtx 中，若已超时，则cancel之 并返回 c 和 cancel 方法
// 若未超时，则在时间到达后，由 c.timer 控制的异步协程执行 cancel，可调用c.timer.Stop()提前终止异步执行
func WithDeadlineCause(parent Context, d time.Time, cause error) (Context, CancelFunc) {
	if parent == nil {
		panic("cannot create context from nil parent")
	}
	if cur, ok := parent.Deadline(); ok && cur.Before(d) {
		// The current deadline is already sooner than the new one.
		return WithCancel(parent)
	}
	c := &timerCtx{
		deadline: d,
	}
	c.cancelCtx.propagateCancel(parent, c)
	dur := time.Until(d)
	if dur <= 0 {
		c.cancel(true, DeadlineExceeded, cause) // deadline has already passed
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

func WithTimeout(parent Context, timeout time.Duration) (Context, CancelFunc) {
	return WithDeadline(parent, time.Now().Add(timeout))
}

func WithTimeoutCause(parent Context, timeout time.Duration, cause error) (Context, CancelFunc) {
	return WithDeadlineCause(parent, time.Now().Add(timeout), cause)
}
```

```go
type afterFuncer interface {
	AfterFunc(func()) func() bool
}

type afterFuncCtx struct {
	cancelCtx
	once sync.Once // either starts running f or stops f from running
	f    func()
}

func (a *afterFuncCtx) cancel(removeFromParent bool, err, cause error) {
	a.cancelCtx.cancel(false, err, cause)
	if removeFromParent {
		removeChild(a.Context, a) // 子 context 和 afterFuncCtx 的关系不是通过 children map 维护的，
        // 而是通过 AfterFunc 回调。
	}
	a.once.Do(func() {
		go a.f() // 开协程防止其他人调用cancel()被阻塞
	})
}
```

```go
// 将 child注册到parent中
func (c *cancelCtx) propagateCancel(parent Context, child canceler) {
	c.Context = parent

	done := parent.Done()
	if done == nil {
		return // parent is never canceled
	} // 若 parent 为 context.Background() / 永不取消的 context 
    // 直接返回，无事可做

	select {
	case <-done:
		child.cancel(false, parent.Err(), Cause(parent))
		return
        // parent 已被取消，立即取消 child
	default:
	}

	if p, ok := parentCancelCtx(parent); ok {
		// parent is a *cancelCtx, or derives from one.
		p.mu.Lock()
		if err := p.err.Load(); err != nil {
			// parent has already been canceled
			child.cancel(false, err.(error), p.cause)
		} else {
			if p.children == nil {
				p.children = make(map[canceler]struct{})
			}
			p.children[child] = struct{}{}
		}
		p.mu.Unlock()
		return
        // 把 child 注册进 p.children map
	}

    // 如果有人写了自己的 context 类型，并且知道如何高效注册"取消时回调"
    // （比如它不是基于 Done() channel、没法 select 的实现），
    // 就可以暴露 AfterFunc(func()) func() bool 方法，
    // propagateCancel 就会用它，而不是走最后的兜底逻辑
	if a, ok := parent.(afterFuncer); ok {
		// parent implements an AfterFunc method.
		c.mu.Lock()
		stop := a.AfterFunc(func() {
			child.cancel(false, parent.Err(), Cause(parent))
		})
		c.Context = stopCtx{
			Context: parent,
			stop:    stop,
		}
		c.mu.Unlock()
		return
	}

	goroutines.Add(1)
	go func() {
		select {
		case <-parent.Done():
			child.cancel(false, parent.Err(), Cause(parent))
		case <-child.Done():
		}
	}()
```

```go
func AfterFunc(ctx Context, f func()) (stop func() bool) {
	a := &afterFuncCtx{
		f: f,
	}
	a.cancelCtx.propagateCancel(ctx, a)
	return func() bool {
		stopped := false
		a.once.Do(func() {
			stopped = true
		})
		if stopped {
			a.cancel(true, Canceled, nil)
		}
		return stopped
	}
}
```