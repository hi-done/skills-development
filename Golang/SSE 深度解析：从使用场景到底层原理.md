# SSE 深度解析：从使用场景到底层原理

接入 LLM 流式输出时，你一定见过它：模型回复不再等全文生成，而是逐字蹦出来——背后撑起这个体验的，正是 **SSE（Server-Sent Events）**。

SSE 用起来像魔法，几行代码就能让服务器持续推送数据；拆开底层看却非常"土"：**本质上就是一个不关闭的 HTTP 响应**。本文先讲清楚 SSE 是什么、什么时候该用，再给出一份生产级 Go 实现，最后穿透到 TCP / HTTP 层，把它彻底看透。

## 一、SSE 是什么

**SSE（Server-Sent Events）** 是 HTML5 标准的一部分，基于 HTTP 协议，允许服务器**单向、持续地**向浏览器推送文本数据。

客户端通过 `EventSource` API 建立连接，服务器以特殊的 `text/event-stream` 格式持续发送数据。

### 1.1 典型使用场景

| 场景 | 说明 |
| --- | --- |
| **实时通知 / 消息推送** | 社交媒体的新消息提醒、系统告警推送 |
| **实时数据监控** | 服务器 CPU / 内存指标、IoT 设备状态面板 |
| **新闻 / 资讯流** | 实时新闻推送、股票行情（只读行情） |
| **AI 对话流式输出** | ChatGPT 等 LLM 逐字输出回复（目前最火的场景） |
| **日志实时查看** | 前端实时查看后端构建 / 部署日志 |
| **进度条 / 任务状态** | 长时间任务（如文件处理）的进度推送 |

> **核心特征**：这些场景都是**服务器 → 客户端单向推送**，客户端不需要频繁发送数据。

### 1.2 SSE vs WebSocket

| 维度 | SSE | WebSocket |
| --- | --- | --- |
| **协议** | 基于 HTTP（复用 HTTP 连接） | 独立的 `ws://` 协议（HTTP 升级） |
| **通信方向** | **单向**（服务器 → 客户端） | **全双工**（双向同时通信） |
| **数据格式** | 仅文本（可编码 JSON） | 文本 + 二进制（Blob / ArrayBuffer） |
| **连接数限制** | 受 HTTP 连接数限制（浏览器通常 6 个 / 域名） | 无此限制，每个 WS 独立连接 |
| **断线重连** | **浏览器自动重连**（内置） | 需手动实现重连逻辑 |
| **消息格式** | 标准 `text/event-stream` 格式 | 无标准格式，需自行定义协议 |
| **跨域** | 支持 CORS | 支持 CORS |
| **代理 / 防火墙友好** | ✅ 基于 HTTP，穿透性好 | ⚠️ 部分代理 / 防火墙可能拦截 |
| **复杂度** | 简单，几行代码即可用 | 相对复杂，需定义消息协议 |
| **浏览器支持** | IE 不支持，现代浏览器均支持 | 所有现代浏览器（含 IE10+） |

### 1.3 如何选型？

```text
需要双向通信？
├── 是 → WebSocket
│   场景：聊天室、多人协作编辑、在线游戏、实时音视频信令
│
└── 否（只需服务器推送）
    ├── 需要二进制数据？ → WebSocket
    ├── 需要自动重连 / 断点续传？ → SSE（内置支持）
    ├── 需要极致简单？ → SSE
    └── 大多数只读实时场景 → SSE ✅
```

### 1.4 SSE 的核心优势

1. **实现简单**：不需要额外协议库，HTTP 服务器稍作改造即可。
2. **自动重连**：网络抖动后浏览器自动恢复连接，且支持 `Last-Event-ID` 断点续传。
3. **HTTP 友好**：能穿过所有代理、负载均衡、CDN，无需额外端口。
4. **天然适配 LLM**：AI 流式输出几乎都用 SSE（OpenAI API 就是 SSE）。

## 二、五分钟快速上手

先看一个最小可运行的例子。

### 2.1 服务端（Go）

```go
package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func sseHandler(w http.ResponseWriter, r *http.Request) {
	// 1. 设置 SSE 必需的响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// 2. 拿到 Flusher（用于手动冲刷数据）
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	log.Println("client connected")

	// 3. 持续推送（注意：响应永远不结束）
	for i := 0; ; i++ {
		select {
		case <-r.Context().Done():
			// 客户端断开连接
			log.Println("client disconnected")
			return
		default:
		}

		// 4. 按 SSE 格式写入：data: + 内容 + 双换行
		fmt.Fprintf(w, "data: message %d\n\n", i)
		flusher.Flush()

		time.Sleep(1 * time.Second)
	}
}

func main() {
	http.HandleFunc("/events", sseHandler)

	log.Println("server started at :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
```

### 2.2 客户端

**终端验证：**

```sh
curl http://localhost:8080/events
```

**浏览器：**

```javascript
const es = new EventSource("http://localhost:8080/events");
es.onmessage = e => console.log(e.data);
```

## 三、消息格式：后端怎么写，前端怎么转

前面的 demo 里每一帧都写着 `data: xxx\n\n`，这个 `data:` 前缀和双换行到底是什么规则？后端能写哪些字段，前端又会把它们变成什么？这一节一次讲清楚。

### 3.1 后端写入格式

SSE 协议只定义了五种"行的种类"，后端做的事情就是按 `字段名: 值` 逐行写：

| 字段 | 作用 | 示例 |
| --- | --- | --- |
| `data` | 消息内容（真正的"消息体"） | `data: {"usage": 73.2}` |
| `event` | 事件类型，缺省为 `message` | `event: cpu` |
| `id` | 消息 ID，浏览器记录，重连时回传 | `id: 100` |
| `retry` | 自动重连间隔（毫秒） | `retry: 3000` |
| `:` 开头 | 注释行，不触发任何事件，常用于心跳 | `: ping` |

一条"字段齐全"的消息长这样（**字段顺序无所谓**）：

```text
id: 100
event: cpu
retry: 3000
data: {"usage": 73.2}

```

关键是最后那个**空行**——它才是"一条消息结束"的标志。

**Go 中的几种典型写法：**

```go
// 1. 普通文本
fmt.Fprintf(w, "data: %s\n\n", "hello")

// 2. 携带 id 与事件类型
fmt.Fprintf(w, "id: %d\nevent: cpu\ndata: %s\n\n", seq, payload)

// 3. 多行内容：拆成多条 data 行，前端会以 \n 拼接
fmt.Fprintf(w, "data: line1\ndata: line2\n\n")

// 4. 心跳注释（保活 NAT / 代理）
fmt.Fprintf(w, ": ping\n\n")

// 5. 告知浏览器重连间隔（不触发任何事件）
fmt.Fprintf(w, "retry: 3000\n\n")

flusher.Flush() // 每写完一条消息都要冲刷
```

> **实际业务中**，`data` 几乎总是 JSON：后端 `json.Marshal` 后写入，前端 `JSON.parse` 还原，这就是最常见的一对"写入 → 转换"。

### 3.2 后端写入规则清单

1. **每条消息以空行结尾**（`\n\n`）。少了这个空行，前端会一直等，一条消息也收不到；
2. **多行内容必须拆成多条 `data:` 行**。直接把含换行的字符串塞进一条 `data:`，换行后的内容会被前端当成新字段解析；
3. **统一 UTF-8 编码**，这是协议保证的唯一安全编码；
4. **字段名拼错不会报错**：未知字段会被静默忽略。比如把 `event` 拼成 `envent`，前端只会按默认的 `message` 事件处理，非常难排查；
5. **`id` 不能包含 null 字符（`\0`）**，也不要过长（浏览器对记录长度有限制）；
6. **`retry` 只需发一次**，浏览器会长期记住这个间隔（默认约 3 秒）。

### 3.3 前端转换（一）：EventSource 自动解析

后端按规则写入的字段，`EventSource` 会自动"翻译"成事件和属性：

| 后端写入 | 前端行为 |
| --- | --- |
| `data: hello\n\n` | 触发 `onmessage`，`e.data === "hello"` |
| `data: a\ndata: b\n\n` | `e.data === "a\nb"`（多行自动拼接） |
| `event: cpu\ndata: {...}\n\n` | 触发 `addEventListener("cpu")` 的回调（不再走 `onmessage`） |
| `id: 11\ndata: x\n\n` | `e.lastEventId === "11"`，重连时自动通过 `Last-Event-ID` 回传 |
| `retry: 3000\n\n` | 更新自动重连间隔为 3 秒，不触发事件 |
| `: ping\n\n` | 不触发任何事件，仅保活连接 |

对应的前端代码：

```javascript
const es = new EventSource("/events");

// event 缺省 → 走 onmessage
es.onmessage = e => {
  const msg = JSON.parse(e.data); // e.data 永远是字符串，需自行转对象
  console.log("message:", msg);
};

// event: cpu → 走自定义事件监听
es.addEventListener("cpu", e => {
  const { usage } = JSON.parse(e.data);
  updateChart(usage);
});

es.onopen = () => console.log("connected");
es.onerror = () => console.log("error（浏览器将按 retry 间隔自动重连）");
```

转换的关键点就三条：

- **`e.data` 永远是字符串**：多行 `data:` 会用 `\n` 拼接，JSON 需要自己解析；
- **`event` 字段决定由谁接收**：写了 `event: cpu` 就必须用 `addEventListener("cpu", ...)` 接，`onmessage` 收不到；
- **重连不用写代码**：`onerror` 触发后浏览器自动按 `retry` 间隔重连，想停止只能手动 `es.close()`。

### 3.4 前端转换（二）：fetch 手动解析

`EventSource` 有两个硬限制：

- 只能发 **GET** 请求，不支持 POST；
- **不能自定义请求头**，带不了 `Authorization`。

所以 LLM 对话（POST + Bearer Token）、需要传复杂参数的场景，普遍改用 `fetch` + `ReadableStream` 手动读流，自行解析 SSE 格式。解析逻辑就是把 3.1 的写入规则反过来执行：

1. 按行读取，累积 `data:` 行的内容（多行以 `\n` 拼接）；
2. 遇到**空行** → 一条消息结束，派发出去并清空缓冲；
3. `event:` / `id:` 一并记录，其余行忽略。

```javascript
async function connectSSE(url, onEvent) {
  const resp = await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "Accept": "text/event-stream",
      "Authorization": "Bearer <token>",
    },
    body: JSON.stringify({ prompt: "你好" }),
  });

  const reader = resp.body.getReader();
  const decoder = new TextDecoder(); // 默认 UTF-8
  let buffer = "";

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;

    // stream: true → 处理跨 chunk 的多字节字符（中文必需）
    buffer += decoder.decode(value, { stream: true });

    let idx;
    while ((idx = buffer.indexOf("\n\n")) !== -1) {
      const raw = buffer.slice(0, idx); // 一条完整消息
      buffer = buffer.slice(idx + 2);   // 残余留给下一个 chunk

      let eventName = "message";
      const dataLines = [];

      for (const line of raw.split("\n")) {
        if (line.startsWith("data:")) {
          dataLines.push(line.slice(5).trimStart());
        } else if (line.startsWith("event:")) {
          eventName = line.slice(6).trim();
        }
      }

      if (dataLines.length > 0) {
        onEvent(eventName, dataLines.join("\n"));
      }
    }
  }
}

// 使用
connectSSE("/v1/chat/stream", (event, data) => {
  if (data === "[DONE]") return; // "[DONE]" 是 OpenAI 惯例，不是 SSE 协议
  console.log(event, JSON.parse(data));
});
```

手动解析有三个容易踩的坑：

- **中文乱码**：一个汉字可能跨两个 chunk 传输，必须用 `decoder.decode(value, { stream: true })` 让解码器缓存残余字节；
- **帧边界跨 chunk**：`\n\n` 可能一个 `\n` 在前一个 chunk、另一个在后一个 chunk，所以必须先往 `buffer` 里累积、再切消息，不能对单次 `value` 直接解析；
- **`\r\n` 兼容**：协议允许行尾用 `\r\n`（此时事件边界是 `\r\n\r\n`），简单按 `\n\n` 切会漏帧；严谨做法是先把缓冲区里的 `\r\n` 归一化为 `\n`。

> **一句话总结**：后端写入 = `字段: 值` + 空行；前端转换 = 把行攒成事件。`EventSource` 全自动，但受限于 GET；`fetch` 手动解析麻烦一点，换来的是 POST、自定义请求头和随时 `abort` 的控制权。

## 四、生产级 SSE 实现

Demo 够简单，但离生产可用还差几步：

- **每个连接一个 goroutine**：互不阻塞；
- **用 channel 推送数据**：避免多 goroutine 直接写同一个 `ResponseWriter`；
- **用 context 控制退出**：客户端断开后立刻释放资源；
- **支持广播 / 单播**：业务侧按需推送。

下面逐层拆解。

### 4.1 定义 SSE 客户端

```go
type SSEClient struct {
	ID     string
	Writer http.ResponseWriter
	Flush  http.Flusher
	Send   chan string   // 每个客户端独立的发送队列
	Done   chan struct{} // 连接结束信号
}
```

### 4.2 广播管理器（核心）

```go
type SSEManager struct {
	mu      sync.Mutex
	clients map[string]*SSEClient
}

func NewSSEManager() *SSEManager {
	return &SSEManager{
		clients: make(map[string]*SSEClient),
	}
}

func (m *SSEManager) Add(c *SSEClient) {
	m.mu.Lock()
	m.clients[c.ID] = c
	m.mu.Unlock()
}

func (m *SSEManager) Remove(id string) {
	m.mu.Lock()
	delete(m.clients, id)
	m.mu.Unlock()
}

// Broadcast 把消息投递到每个客户端的发送队列。
// 队列满说明该客户端消费过慢，丢弃本条消息，避免拖垮整个广播流程。
func (m *SSEManager) Broadcast(data string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, c := range m.clients {
		select {
		case c.Send <- data:
		default:
		}
	}
}
```

### 4.3 Handler 接入

```go
var manager = NewSSEManager()

func sseHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	client := &SSEClient{
		ID:     uuid.New().String(), // 也可以用自增 ID
		Writer: w,
		Flush:  flusher,
		Send:   make(chan string, 16),
		Done:   make(chan struct{}),
	}

	manager.Add(client)
	defer manager.Remove(client.ID)
	defer close(client.Done)

	go client.writeLoop()

	<-r.Context().Done() // 阻塞直到客户端断开
}

// writeLoop 是该连接唯一的数据写入口
func (c *SSEClient) writeLoop() {
	for {
		select {
		case msg := <-c.Send:
			fmt.Fprintf(c.Writer, "data: %s\n\n", msg)
			c.Flush.Flush()
		case <-c.Done:
			return
		}
	}
}
```

> **注意**：`http.ResponseWriter` 并不是并发安全的。这里每个连接只有 `writeLoop` 一个 goroutine 负责写，广播只负责往 channel 投递，从根上避免了并发写问题。

### 4.4 业务触发推送

```go
go func() {
	for {
		manager.Broadcast("hello world")
		time.Sleep(2 * time.Second)
	}
}()
```

## 五、穿透底层：SSE 的四层实现

把 SSE 拆成 **四层** 来看，一眼就能看透它为什么"看起来像魔法，其实很土"。

### 5.1 第一层：最底层还是 TCP + HTTP

SSE **不是新协议**。

客户端做的事就是：

```http
GET /sse HTTP/1.1
Accept: text/event-stream
```

服务端做的事就是：

```http
HTTP/1.1 200 OK
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
```

然后——**响应体不结束**。

> 普通 HTTP：请求 → 响应 → 关闭  
> SSE：请求 → 响应开始 → 一直写 → 不关

底层链路就是：

```text
TCP 三次握手 → TLS（如果是 https）→ HTTP 请求 → HTTP 响应头 → 响应体一直写
```

### 5.2 第二层：HTTP 层的 Chunked Transfer

HTTP/1.1 中，如果响应不知道总长度，就不能写 `Content-Length`，而要使用：

```http
Transfer-Encoding: chunked
```

线上抓包大概长这样：

```http
HTTP/1.1 200 OK
Content-Type: text/event-stream
Transfer-Encoding: chunked

1b\r\n
data: hello\n\n\r\n
1c\r\n
data: world\n\n\r\n
...
0\r\n\r\n   ← 结束（SSE 里基本永远不发这个）
```

每一条 SSE 消息，对外就是 HTTP 的一个 chunk。

**在 Go 里，`w.Write(...)` + `Flusher.Flush()` 的本质是：**

- 把数据写进内核 socket 缓冲区；
- 让 HTTP 层把当前字节当 chunk 发出去；
- 不等"响应结束"。

### 5.3 第三层：应用层的"文本帧格式"

TCP / HTTP 只负责"把字节传过去"，但**怎么算一条消息？**

SSE 的规则超级简单：

```text
field: value
field: value

← 两个换行 = 一条消息结束
```

例如：

```text
id: 10
event: cpu
data: {"usage": 73.2}

id: 11
data: {"usage": 74.0}

```

浏览器 `EventSource` 内部做的事就是：

1. 从 TCP 流里读字节；
2. 按行解析；
3. 遇到空行 `\n\n` → 组装成一条 event；
4. 触发 `onmessage` / `addEventListener('cpu', ...)`；
5. 继续读，不关连接。

以冒号开头的行是注释，常用于心跳：

```text
: ping

```

浏览器会忽略它，但能保活 NAT / 代理。

### 5.4 第四层：EventSource 帮你做的三件大事

这是 SSE 与"自己用 fetch 读流"最大的区别。

**① 自动重连**

连接断了（TCP 断开 / 代理超时 / 服务端重启），`EventSource` 会自动重发：

```http
GET /sse
Last-Event-ID: 11
```

你代码里一行重连逻辑都不用写。

**② Last-Event-ID 断点续传**

如果服务端发过：

```text
id: 11
data: xxx
```

浏览器会记住 `11`，重连时自动带上：

```http
Last-Event-ID: 11
```

服务端就可以只发 12 之后的数据。

**③ 消息顺序 & 事件类型**

- 顺序：TCP 保证顺序，SSE 不再额外排序；
- 事件类型：`event: cpu` → `addEventListener('cpu', ...)`；
- 默认事件：`message`。

## 六、SSE vs WebSocket：底层分水岭

| 层 | SSE | WebSocket |
| --- | --- | --- |
| 建连 | 普通 HTTP GET | HTTP + `Upgrade: websocket` |
| 握手后 | 还是 HTTP 响应 | 变成 WebSocket 协议（RFC 6455） |
| 数据单位 | 文本行（`data: ...\n\n`） | 二进制 / 文本帧（opcode、mask、len） |
| 方向 | 服务器 → 客户端 | 全双工 |
| 代理友好 | ✅ 非常友好 | ⚠️ 要支持 Upgrade |
| 重连 | 浏览器原生 | 自己写 |

WebSocket 握手后，HTTP 已经"退场"；而 SSE 从头到尾都在 HTTP 里。

## 七、总结：一句话看穿 SSE

> **SSE = 普通 HTTP 响应 + 不关连接 + chunked 传输 + 双换行为消息边界 + 浏览器自动重连。**

你用 Go 写的：

```go
w.Header().Set("Content-Type", "text/event-stream")
w.(http.Flusher).Flush()
```

翻译成底层语言就是：

> "别等响应结束，现在就把这段 UTF-8 文本塞进 TCP 发送队列，浏览器你继续听着。"

至此，SSE 不再是魔法——它只是把"一个不关闭的 HTTP 响应"用到了极致。
