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

// 从 传入的context 的 Value 链中，找到*cancelCtx，并且校验 *cancelCtx 的 done channel 等于 传入的context 的 done channel
// 返回 *cancelCtx 及 校验结果。该 *cancelCtx 可安全挂载 子 context，实现级联cancel
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
		return nil, false // 防止绕过用户自定义的 Done() 实现。
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
// 要知道，removeChild只会在Context包中被调用
// (c *cancelCtx) cancel、(c *timerCtx) cancel、(a *afterFuncCtx) cancel
// 并且使用时是 parent 为 child.Context
// 因此不会存在拿一个与parent无关的child 来 remove
func removeChild(parent Context, child canceler) {
	if s, ok := parent.(stopCtx); ok {
		s.stop() 
		// 在 propagateCancel中，当用户自定义了 Done()方法，则会为child c 生成stopCtx的parent, c.Context = stopCtx
		// stop()会将child的cancel方法从parent的取消方法的map中delete
		return
	}
	p, ok := parentCancelCtx(parent)
	if !ok {
		return // 如果 parent 的 Value 链中没有*cancelCtx，直接返回
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
	once sync.Once
	f    func() // 在cancel中使用，且在调用stop时不使用，借由 once 实现
}

func (a *afterFuncCtx) cancel(removeFromParent bool, err, cause error) {
	a.cancelCtx.cancel(false, err, cause)
	if removeFromParent {
		removeChild(a.Context, a) // 与 removeChild(a.cancelCtx.Context, a) 等价
		// 不直接使用 a.cancelCtx.cancel(true, err, cause) 
		// 是因为 a.Context 的 *cancelCtx 中 的 children map 里，key 是 a，而非a.cancelCtx
		// 正如此，若 a.Context 被 cancel，才会级联调用 a.cancel()，从而触发 a.f()
	a.once.Do(func() {
		go a.f() // 开协程防止其他人调用cancel()被阻塞
	})
}
```

```go
// 将 child注册到parent中
// 当 创建 WithCancel、WithDeadline等时会在内部调用，且 child 就是 c: c.propagateCancel(parent, c)
// 或者是cancelCtx的上一级: a.cancelCtx.propagateCancel(parent, a)
func (c *cancelCtx) propagateCancel(parent Context, child canceler) {
	c.Context = parent

	done := parent.Done()

	// 情况一
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

	// 情况二
	if p, ok := parentCancelCtx(parent); ok {
		// parent is a *cancelCtx, or derives from one.
		p.mu.Lock()
		if err := p.err.Load(); err != nil {
			// 如果 parent context 的 *cancelCtx 已被取消，则将 child 也取消
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

	// 情况三
	// 当 parent 有 Done()返回 非nil channel，且未关闭，且无cancelCtx 或 自定义了Done() （非cancelCtx 自带的Done()）
	// 其实，一定实现了自定义的Done()，因为若无cancelCtx，则一定有Done()，若有cancelCtx，则也一定有Done()，否则就返回了cancelCtx，直接走上面的逻辑
	// 所以，总结就是实现了自定义的Done()，且channel未关闭
    // 此时无法通过parentCancelCtx获取parent上可挂在的cancelCtx


    // 若 parent 实现了 AfterFunc(func()) func() bool 方法，现用户想根据该context 通过 
	// WithCancel、WithDeadline等派生可级联cancel 的 子context
    // propagateCancel 就会用它，而不是走最后的兜底逻辑
	if a, ok := parent.(afterFuncer); ok {
		// parent implements an AfterFunc method.
		c.mu.Lock()
		stop := a.AfterFunc(func() {
			child.cancel(false, parent.Err(), Cause(parent))
		})
		// AfterFunc中会将child.cancel(false, parent.Err(), Cause(parent))注册到
		// afterFuncs map[*byte]func()
		// 调用时会从 afterFuncs 中 delete 该 child.cancel
		c.Context = stopCtx{
			Context: parent,
			stop:    stop,
		}
		// 此时c 的parent context就是 stopCtx，会应用到后续的removeChild
		// 目的是在parent 取消时，能够触发child.cancel(false, parent.Err(), Cause(parent))，
		// 而主动调用 stop()时，不会触发child.cancel(false, parent.Err(), Cause(parent))，
		// 只会将 AfterFunc 潜在生成的 afterFuncs 中 delete 该 child.cancel
		// 既如此，stopCtx (c.Context) 只会影响一个 child
		// 但 parent 能过够 挂载多个 child，实现级联取消 
		c.mu.Unlock()
		return
	}

	// 情况四
	// 以上均不成功，走起线程监听parent.Done，以调用child.cancel
	goroutines.Add(1)
	go func() {
		select {
		case <-parent.Done():
			child.cancel(false, parent.Err(), Cause(parent))
		case <-child.Done():
		}
	}()
```

// 把一个函数f()关联Context，目的是当该Context取消时，会触发f()
// 但 当调用stop函数时，则终止在Context取消时对于f()的触发
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
			a.cancel(true, Canceled, nil) // 此时不会调用 a.f()，即调用stop方法不会触发a.f()，只会在a自身cancel时触发
		}
		return stopped
	}
}
```

---

propagateCancel 的情况三

**实现 `afterFuncer` 的场景 = 你的取消源不在标准 context 树里，但你希望下游能用标准 context API 派生和响应。** 如果取消源本身已经是 context，直接 `WithCancel` 即可，不需要自定义。

```go
type afterFuncContext struct {
	mu         sync.Mutex
	afterFuncs map[*byte]func()   // 注册表：唯一指针做 key
	done       chan struct{}
	err        error
}

func (c *afterFuncContext) AfterFunc(f func()) func() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err != nil {                    // 已取消：契约要求立即（在自己 goroutine 里）执行
		c.mu.Unlock()
		go f()
		return func() bool { return false }
	}
	k := new(byte)                      // 每次注册一个唯一 key → 多次注册相互独立
	if c.afterFuncs == nil {
		c.afterFuncs = make(map[*byte]func())
	}
	c.afterFuncs[k] = f                 // 只注册，不执行
	return func() bool {                // ← 这个闭包就是 stopCtx.stop
		c.mu.Lock()
		defer c.mu.Unlock()
		_, ok := c.afterFuncs[k]         // 已被取消(map=nil)或已 stop → ok=false
		delete(c.afterFuncs, k)          // 真正的"注销"动作
		return ok                        // true=本次调用阻止了 f 运行
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
		go f()                           // 关键：永远在自己的 goroutine 里调 f
	}
	c.afterFuncs = nil
}
```