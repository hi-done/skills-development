# `sonic.Marshal` 和 `json.Marshal`

`sonic.Marshal` 和 `json.Marshal` 的核心区别就一句话：

> **`sonic` 是字节跳动的 Go JSON 库，比标准库 `encoding/json` 快 2~5 倍、内存分配更少，但有一些兼容性限制。**

下面从**性能、原理、兼容性、使用场景**几个维度详细对比：

---

## 一、性能差距（核心差异）

| 指标         | `encoding/json`                 | `sonic`                    | 差距 |
| ------------ | ------------------------------- | -------------------------- | ---- |
| 序列化速度   | 基准                            | **快 2~5 倍**              | 🚀    |
| 反序列化速度 | 基准                            | **快 2~4 倍**              | 🚀    |
| 内存分配     | 多（大量 `interface{}` + 反射） | **少 50%~80%**             | 🧠    |
| 零拷贝       | 不支持                          | **支持 `[]byte` 直接操作** | ⚡    |

benchmark 参考（典型 struct，无特殊类型）：

```
Benchmark_StdJSON_Marshal-8     3.2M ops/s   380 ns/op   256 B/op   3 allocs/op
Benchmark_Sonic_Marshal-8       12.8M ops/s   93 ns/op    64 B/op   1 allocs/op
```

> 4 倍吞吐量提升，内存分配从 3 次降到 1 次。

---

## 二、为什么 sonic 这么快？

### 1. 用 JIT 编译（核心黑科技）

- `encoding/json`：纯 Go 反射，逐字段 runtime 解析
- `sonic`：**启动时把 struct 的序列化逻辑 JIT 编译成机器码**（基于 Go 的 `reflect + asm + 运行时代码生成`），后续直接执行编译好的路径，跳过反射开销

### 2. SIMD 指令加速

- 字符串转义、数字解析等热点路径用 **AVX/AVX2/AVX-512** 指令并行处理
- 需要 **amd64** 或 **arm64** 架构（不支持 386、riscv 等）

### 3. 无反射 / 懒解析

- 反序列化时支持 **lazy loading**，不立即解析整个 JSON，用到哪个字段才解析哪个
- `encoding/json` 必须一次性全部解析完

### 4. 零拷贝

```go
// sonic 支持直接操作 []byte，不产生中间 string 拷贝
node := sonic.Get(data)           // 零拷贝解析
val := node.Get("key").String()  // 延迟取值
```

---

## 三、API 对比（用法几乎一样）

### 基本用法

```go
import (
    "encoding/json"
    "github.com/bytedance/sonic"
)

type User struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
    Tags []string `json:"tags"`
}

u := User{ID: 1, Name: "张三", Tags: []string{"go", "backend"}}

// encoding/json
data1, _ := json.Marshal(u)

// sonic —— API 完全兼容
data2, _ := sonic.Marshal(u)
```

### 反序列化

```go
var u1, u2 User
json.Unmarshal(data, &u1)
sonic.Unmarshal(data, &u2)  // ✅ 一样用
```

### 流式 API（sonic 独有优势）

```go
// sonic 的 Node API —— 不用定义 struct 就能操作 JSON
root, _ := sonic.Get(data)
name, _ := root.Get("name").String()
age, _ := root.Get("age").Int64()
```

---

## 四、兼容性差异（⚠️ 重要）

### sonic 不支持 / 行为不同的地方

| 场景                          | `encoding/json`           | `sonic`                  |
| ----------------------------- | ------------------------- | ------------------------ |
| `time.Time` 自定义格式        | ✅ 实现 `MarshalJSON` 即可 | ⚠️ 部分版本有 bug，需测试 |
| `encoding.TextMarshaler` 接口 | ✅ 自动识别                | ⚠️ 部分类型不识别         |
| 匿名嵌套 struct 的 tag 处理   | ✅                         | ⚠️ 偶有差异               |
| `map[interface{}]interface{}` | ✅（但很少用）             | ❌ 不支持                 |
| `NaN/Inf` 浮点数              | 输出 `"NaN"`              | ❌ 报错或输出异常         |
| Go 1.21+ `omitzero` tag       | ✅                         | ⚠️ 旧版本不支持           |
| `json.RawMessage`             | ✅                         | ✅ 支持                   |

### 架构限制

| 架构    | `encoding/json` | `sonic`             |
| ------- | --------------- | ------------------- |
| amd64   | ✅               | ✅                   |
| arm64   | ✅               | ✅                   |
| 386     | ✅               | ❌                   |
| riscv64 | ✅               | ❌                   |
| loong64 | ✅               | ❌（部分版本实验性） |

如果你在 **树莓派 / 老 x86 32位 / 国产架构** 上跑，sonic 直接编译失败。

---

## 五、什么时候该用 sonic？

### ✅ 适合用 sonic 的场景

| 场景                                      | 理由                         |
| ----------------------------------------- | ---------------------------- |
| **HTTP API 高频序列化**（网关、微服务）   | 吞吐量直接翻倍               |
| **大 JSON 处理**（>100KB）                | 内存分配少，GC 压力小        |
| **配置中心 / 消息队列** 序列化            | 零拷贝 + 懒解析              |
| **MySQL JSON 字段读写**（你前面问的场景） | 性能提升明显                 |
| **高并发服务**，CPU 是瓶颈                | JIT 编译后接近手写序列化代码 |

### ❌ 不适合用 sonic 的场景

| 场景                                  | 理由                                                                 |
| ------------------------------------- | -------------------------------------------------------------------- |
| 内部工具 / CLI                        | 性能无所谓，标准库够用                                               |
| 需要 100% 兼容 `encoding/json` 行为   | sonic 有边缘 case 差异                                               |
| 32 位 / 非主流架构                    | 编译不过                                                             |
| 极度追求稳定性，不想引入 CGO/JIT 依赖 | sonic 依赖 `github.com/bytedance/sonic/loader`（内部用 mmap 做 JIT） |
| 处理不可信的超大 JSON（防 DoS）       | sonic 的懒解析有深度限制但标准库更保守                               |

---

## 六、接入成本

### 最小改动迁移

```go
// 全局替换（最简单）
import json "github.com/bytedance/sonic"
// 后面所有 json.Marshal / json.Unmarshal 自动走 sonic
```

或者只替换热点路径：

```go
import "github.com/bytedance/sonic"

// 原来
data, err := json.Marshal(req)

// 改后
data, err := sonic.Marshal(req)
```

### Go 1.21+ 还有另一个选择：`jsoniter`

| 库              | 速度         | 兼容性 | 架构        | 维护状态       |
| --------------- | ------------ | ------ | ----------- | -------------- |
| `encoding/json` | 基准         | 100%   | 全架构      | ✅ 官方         |
| `sonic`         | ⭐⭐⭐⭐⭐ 最快   | ~95%   | amd64/arm64 | ✅ 字节活跃维护 |
| `jsoniter`      | ⭐⭐⭐⭐ 快 2~3x | ~98%   | 全架构      | ⚠️ 维护较慢     |

---

## 七、一句话总结

|          | `json.Marshal` | `sonic.Marshal`                        |
| -------- | -------------- | -------------------------------------- |
| 速度     | 够用           | **快 3~5 倍**                          |
| 内存     | 一般           | **少很多**                             |
| 兼容性   | 100%           | 95%（注意边缘 case）                   |
| 架构     | 全平台         | amd64 / arm64                          |
| 引入风险 | 无             | JIT + mmap（生产环境已验证大规模使用） |

> **生产建议**：HTTP API / 微服务 / 高频 JSON 处理 → 用 sonic；内部项目 / 工具 → 标准库就够。