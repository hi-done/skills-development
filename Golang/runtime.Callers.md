# `runtime.CallersFrames` 怎么知道源文件行号

`runtime.CallersFrames` 返回的 `runtime.Frame` 中包含 `Line` 字段，**不是运行时"猜"的，而是来自 Go 的调试信息（DWARF）**，这些信息在**编译期**就已经嵌入到二进制文件中了。

## 详细机制

### 1. `runtime.Callers` 拿到的是什么？

```go
pcs := make([]uintptr, 32)
n := runtime.Callers(skip, pcs)
```

`pcs` 里存的是 **程序计数器（PC，Program Counter）** 的数组——每个 `uintptr` 就是一个返回地址，指向当前 goroutine 调用栈上每一帧**即将返回的下一条指令的地址**。

这些地址是**纯机器码地址**，本身不包含文件名和行号。

### 2. `runtime.CallersFrames` 做了什么？

```go
frames := runtime.CallersFrames(pcs[:n])
```

这个函数把 PC 地址数组转换成 `Frames` 迭代器。关键在于 **PC → 源码位置 的映射关系从哪来？**

### 3. 映射关系来自编译期嵌入的 DWARF 调试信息

Go 编译器（`gc`）在编译每个包时，会把以下信息写入二进制的 **DWARF 段**（`.debug_line` 等 ELF section）：

| 调试信息内容 | 作用 |
|---|---|
| 每个函数的起止 PC 范围 | 知道某个 PC 落在哪个函数里 |
| PC → 源文件 + 行号的映射表 | 知道某个 PC 对应源码第几行 |
| 函数名、文件名 | 供 `frame.Function`、`frame.File` 使用 |

这个映射表大致长这样（概念上）：

```
PC 范围              文件              行号
0x401000-0x401050   main.go           10
0x401050-0x401100   main.go           15
0x401100-0x401200   log.go            42
...
```

### 4. `frames.Next()` 查表过程

每次调用 `frames.Next()`，内部大致做这些事：

```
1. 取出下一个 PC 值
2. 在 DWARF 的 PC→行号映射表中二分查找，找到这个 PC 落在哪个范围
3. 从映射表中取出对应的：
   - frame.File   (源文件路径)
   - frame.Line   (行号)
   - frame.Function (函数名，含包路径)
   - frame.Entry  (函数入口 PC)
4. 封装成 runtime.Frame 返回
```

所以 **`frame.Line` 不是运行时计算出来的，而是编译期就确定好、运行时查表查出来的。**

---

## 一个关键细节：为什么是"即将返回的下一条指令"？

`runtime.Callers` 捕获的每个 PC 实际上是 **被调用函数执行完后，返回后要执行的下一条指令地址**。

```
func A() {
    B()   // ← 调用 B
}
```

当在 `B` 内部调用 `runtime.Callers` 时，栈上 A 帧对应的 PC 指向的是 `B()` 调用之后的那条指令。DWARF 映射表会把这条指令映射回 **`B()` 被调用的那一行**——这正好是你想要的"调用者行号"。

---

## 如果剥离了调试信息会怎样？

| 情况 | 结果 |
|---|---|
| 正常 `go build` | ✅ `frame.File`、`frame.Line` 都有值 |
| `go build -ldflags="-s -w"` | ❌ `frame.File` 为空字符串，`frame.Line` 为 0 |
| 生产环境 Docker 多阶段构建（常 strip） | ❌ 同样丢失 |

> `-s` 去掉符号表，`-w` 去掉 DWARF 调试信息。两者都会导致 `runtime.CallersFrames` 查不到行号。

---

## 验证一下

可以用下面这段代码快速验证：

```go
package main

import (
	"fmt"
	"runtime"
)

func foo() {
	pc, file, line, ok := runtime.Caller(0)
	fmt.Printf("PC=%x, file=%s, line=%d, ok=%v\n", pc, file, line, ok)
}

func main() {
	foo()
}
```

然后用 `go build -ldflags="-s -w"` 编译再运行，会发现 `file` 为空、`line` 为 0。

---

## 总结一句话

> **`frame.Line` 能返回行号，是因为 Go 编译器在二进制中嵌入了 DWARF 调试信息，将每个机器指令地址映射到源码文件和行号，`runtime.CallersFrames` 只是在运行时查这张表。**