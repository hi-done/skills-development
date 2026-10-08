# Kafka 必背 30 道面试题及标准回答（Golang 后端）

> **适用岗位**：Golang 后端 / 中间件研发 / 分布式系统方向
> **版本基准**：Kafka 2.8 ~ 4.0（经典考点以 3.x 为主，4.0 的架构巨变单独标注）
> **客户端视角**：franz-go / IBM Sarama / segmentio-kafka-go
> **使用建议**：按「架构 → 生产者 → Broker/副本 → 消费者 → 高级实战」递进复习，每题的 **追问** 是面试官深挖的常见方向。

---

## 目录

- 一、基础与架构（Q1-Q6）：核心概念、整体架构、高吞吐原理、分区设计、日志存储、零拷贝
- 二、生产者（Q7-Q12）：发送流程、分区策略、acks、幂等、事务 EOS、消息丢失
- 三、Broker 与副本机制（Q13-Q18）：ISR/LEO/HW、Leader 选举、副本同步、重复消费、消费积压、顺序性
- 四、消费者与消费组（Q19-Q24）：Rebalance、KIP-848、分配策略、Offset 提交、位点存储、Golang 消费者实战
- 五、高级特性与选型（Q25-Q30）：KRaft、共享组、压缩、分区数与调优、MQ 选型、跨机房容灾
- 附录：核心参数速查表 · 版本演进时间线 · 进阶加分题 · 答题框架

---

## 一、基础与架构（Q1-Q6）

### Q1. Kafka 是什么？核心概念与应用场景

**标准回答：**

Kafka 是一个**分布式、多分区、多副本、基于发布/订阅模式**的消息队列（MQ），同时定位为**分布式流处理平台**。由 LinkedIn 开发（作者 Jay Kreps），2011 年开源后捐给 Apache 基金会。

核心概念（面试高频追问点）：

| 概念 | 含义 |
| ---- | ---- |
| Producer | 生产者，向 Broker 发送消息 |
| Consumer | 消费者，从 Broker 拉取消息 |
| Consumer Group | 消费者组，组内分区互斥消费，实现并行 + 容错 |
| Broker | Kafka 服务节点，一个集群由多个 Broker 组成 |
| Topic | 主题，消息的逻辑分类，如 `order-events` |
| Partition | 分区，Topic 的物理分片，是**并行**与**有序**的最小单位 |
| Replica | 副本，Leader 对外读写、Follower 同步数据 |
| Offset | 消息在分区内单调递增的唯一编号 |

四大核心应用场景：

1. **消息系统**：系统解耦、异步通信、流量削峰
2. **日志聚合**：收集各服务日志统一投递到数仓 / ES
3. **流式处理**：作为 Flink / Spark / Kafka Streams 的上下游管道
4. **用户行为追踪与监控指标**：埋点数据、运营指标实时采集

**追问：Kafka 到底是 MQ 还是流处理平台？**

两者皆是。作为 MQ 用时负责消息缓冲与分发；作为流平台时可配合流计算引擎做实时处理，且支持消息回放（seek offset），这是它比传统 MQ 更适合大数据场景的关键。

---

### Q2. Kafka 的整体架构与 Controller 的职责

**标准回答：**

整体架构分三层：

- **客户端层**：Producer / Consumer
- **服务层**：Broker 集群，Topic 分为多个 Partition 分布在各 Broker 上
- **元数据层**：3.x 及之前用 ZooKeeper；4.0 起用 KRaft（元数据自管理）

**Controller 的职责**（集群中一个 Broker 被选为 Controller）：

- 分区 Leader 选举（分区 Leader 故障时主导切换）
- 分区重分配（Partition Reassignment）
- 集群元数据变更处理与通知各 Broker

**ZooKeeper 的职责**（≤ 3.x，面试常问）：

- Broker 注册与存活探测（临时节点，会话断开自动摘除）
- Controller 选举（抢占创建 `/controller` 临时节点）
- 存储 Topic / 分区分配 / 配置 / ACL 等元数据

4.0 起 ZooKeeper 被彻底移除，元数据由 KRaft 自管理（详见 Q26）。

**追问：Controller 挂了会怎样？**

（ZK 模式下）ZooKeeper 感知会话超时后重新选举新 Controller，通常需秒级；期间分区 Leader 切换、Topic 创建等管理操作暂停，但数据读写不受影响。KRaft 下 Controller 故障切换为亚秒级。

---

### Q3. Kafka 为什么吞吐这么高？

**标准回答：**

六点原因（面试按这个顺序说最稳）：

1. **顺序写磁盘**：消息以 append-only 方式追加到日志文件尾部，磁盘顺序写速度接近内存（600MB/s+），避免了随机寻道的 I/O 放大。
2. **PageCache 页缓存**：读写都尽量利用操作系统页缓存，不经过 JVM 堆，避免 GC 压力与对象拷贝，且进程重启后缓存依然有效。
3. **零拷贝**：消费路径用 `sendfile` / `transferTo`，数据从页缓存直达网卡（详见 Q6）。
4. **批量发送 + 压缩**：Producer 端把消息聚合成 batch 后再发送（`batch.size` / `linger.ms`），可配合压缩（zstd 等），大幅减少网络请求数与字节数。
5. **分区并行**：Topic 多分区分布在不同 Broker，生产与消费都可水平扩展。
6. **稀疏索引**：定位消息先二分查稀疏索引（O(logN)），再小范围顺序扫描日志段，读取效率高。

**追问：为什么 Kafka 不靠 fsync 也能保证可靠？**

Kafka 的可靠性设计是**多副本机制**而非单机刷盘。默认由 OS 异步刷盘（可配 `log.flush.*` 强制），单机断电理论上会丢页缓存中未落盘的数据；但副本因子 ≥ 3 且 acks=all 时，数据已在多个 Broker 落地，可容忍单机故障。fsync 会破坏顺序写吞吐优势，所以官方不推荐频繁 fsync。

---

### Q4. Topic 与 Partition 的关系及分区的作用

**标准回答：**

- **Topic 是逻辑概念**，**Partition 是物理存储单元**：一个分区对应磁盘上的一个目录（`topic-partition`），目录内是连续的 append 日志。
- **分区内消息强有序**（按 offset 追加），**不同分区之间无序**。
- 分区是**并行度的基本单位**：
  - 生产端可同时向多个分区写入；
  - 消费端**一个分区同一时刻只能被同组的一个消费者消费**，所以消费并行度上限 = 分区数。
- 分区带来**水平扩展能力**：存储容量与吞吐随分区数增长分摊到多台 Broker。
- **分区数只能增加不能减少**：减少分区会破坏既有 key 的分区映射与顺序语义，社区至今未支持。

**追问：增加分区会有什么副作用？**

1. 默认按 `hash(key) % 分区数` 分区，扩容后**老 key 的映射关系改变**（`hash % 新分区数` 变了），跨扩容量级点的同 key 消息可能落到不同分区，顺序性只在新分区数下成立；
2. 若消费者按 key 做本地状态（如聚合），扩容后需要重新评估；
3. 建议从业务上规划好分区数，或使用自定义分区策略（如一致性哈希）预留扩展性。

---

### Q5. Kafka 的日志存储结构与清理策略

**标准回答：**

**分段（Segment）存储**：

- 每个分区目录下按 Segment 切分日志，默认单个 Segment 1GB（`log.segment.bytes`）；
- 每个 Segment 由三类文件组成，文件名以该段**起始 offset**（20 位数字）命名：
  - `.log`：消息本体（包含 batch 头、CRC 校验）
  - `.index`：偏移量索引（稀疏索引，默认每写入约 4KB 记一条）
  - `.timeindex`：时间戳索引，支持按时间查找消息

**读取流程**：

1. 根据 offset 用二分查找定位到所在 Segment；
2. 在 `.index` 中二分找到不大于目标 offset 的最近索引项 → 得到物理位置；
3. 从该位置顺序扫描 `.log` 文件找到目标消息。

**清理策略**（`cleanup.policy`）：

| 策略 | 行为 | 场景 |
| ---- | ---- | ---- |
| `delete`（默认） | 按时间（`log.retention.hours` 默认 168h=7 天）或大小（`log.retention.bytes`）删除旧 Segment | 普通消息 |
| `compact` | 按 key 保留最新一条，后台 Log Cleaner 周期性合并 | 位点 topic、状态快照类 |

过期检查由 `log.retention.check.interval.ms`（默认 5 分钟）周期性触发，删除以 Segment 为最小单位。

**追问：Kafka 消息被消费后会删除吗？**

不会。这是 Kafka 与其他 MQ 的本质区别之一——消息删除只由**保留策略**（时间/大小/压缩）决定，与消费进度无关，因此支持任意消费者重复回放（seek）。

---

### Q6. 什么是零拷贝？Kafka 如何应用它？

**标准回答：**

**传统文件传输**（read + write）需要 **4 次用户态/内核态上下文切换 + 4 次数据拷贝**：

1. DMA 把磁盘数据拷到内核页缓存；
2. CPU 拷贝：页缓存 → 用户缓冲区；
3. CPU 拷贝：用户缓冲区 → Socket 发送缓冲区；
4. DMA 拷贝：Socket 缓冲区 → 网卡。

**零拷贝（sendfile）** 把步骤 2、3 省掉：

1. DMA：磁盘 → 页缓存；
2. DMA gather：页缓存 → 网卡（数据不进用户态）。

即 **2 次上下文切换 + 2 次拷贝，且无 CPU 参与的数据搬运**。Java 中对应 `FileChannel.transferTo()`。

**Kafka 的应用**：消费者 fetch 数据时，Broker 直接把 `.log` 段文件的数据通过零拷贝路径发送给消费者，避免经过 JVM 堆。

**注意事项**：

- 开启 **SSL/TLS 时零拷贝失效**（数据需要加解密，必须进用户态）；
- 消息格式转换（老版本 down-conversion）时同样失效，退化为堆内拷贝；
- 生产路径不用零拷贝：Broker 收到消息需要校验 CRC、可能重压缩，数据本来就要进入用户态处理。

**追问：`mmap` 和 `sendfile` 的区别？**

`mmap` 把文件映射到用户态虚拟地址空间，读写仍会经过页缓存，适合随机访问场景；`sendfile` 适合"文件 → 网络"的顺序传输，Kafka 消费路径选的是 sendfile，而索引文件的读写用的是 mmap。

---

## 二、生产者（Q7-Q12）

### Q7. Producer 发送一条消息的完整流程

**标准回答：**

消息从 `send()` 到真正发出，经过两阶段（主线程 + Sender 线程）：

**主线程阶段**：

1. **拦截器**（Interceptor，Java 客户端提供；Go 客户端通常在业务层做类似切面）；
2. **序列化**：Key / Value 序列化为字节数组（Go 中常用 JSON / Protobuf / MsgPack）；
3. **分区器**（Partitioner）计算目标分区（见 Q8）；
4. 写入 **RecordAccumulator**（默认 32MB 缓冲）：按分区维护一个队列，消息按批（batch）聚合。

**Sender 线程阶段**：

5. 从 accumulator 取出**就绪的 batch**（满足 `batch.size` 或 `linger.ms` 到期）；
6. 按目标 Broker 分组，批量发往**分区 Leader 所在的 Broker**（每个连接最多 `max.in.flight.requests.per.connection`（默认 5）个未确认请求）；
7. Leader 写入本地日志 → Follower 同步 → 按 `acks` 配置返回响应；
8. 触发用户 Callback；失败则按 `retries` 重试，总时长受 `delivery.timeout.ms`（默认 120s）约束。

**Go 视角**：Sarama 由 `AsyncProducer` 的后台 goroutine 完成第 5-8 步；franz-go 由内部 produce 循环 + 客户端级 batching 完成。所谓"同步发送"，本质是发送后阻塞等待结果 channel。

**追问：`buffer.memory` 满了会怎样？**

`send()` 会阻塞（`max.block.ms` 默认 60s 超时后抛异常）。高吞吐场景要么加大缓冲，要么监控阻塞时间；Go 客户端对应的是内部缓冲/channel 容量与阻塞策略。

---

### Q8. 生产者分区策略与粘性分区

**标准回答：**

分区规则优先级：

1. **指定了 partition** → 直接使用；
2. **未指定 partition 但指定了 key** → `murmur2(key) % 分区数`（负数先取绝对值），保证**同 key 恒落同分区**；
3. **都未指定** → 使用默认分区器。

默认分区器的演进（Java 客户端）：

| 时期 | 策略 | 问题 |
| ---- | ---- | ---- |
| 2.4 之前 | RoundRobin 轮询 | 每个分区一个小 batch，请求数多、吞吐低 |
| 2.4 起 | **粘性分区 StickyPartitioner** | 随机选一个分区连续发，直到 batch 满（`batch.size`）或 `linger.ms` 到期再切换，显著减少小批量请求 |
| 3.3 起 | **自适应粘性分区 AdaptivePartitioner** | 结合分区负载与延迟动态切换，进一步优化 |

**Go 视角**：franz-go 提供 `kgo.UniformBytesPartitioner`、`kgo.StickyPartitioner` 等常用策略；Sarama 默认 `NewHashPartitioner`，可传入自定义分区器实现。

**追问：如何自定义分区规则？**

业务需要"同一租户/同一订单族落同一分区"等规则时：Java 实现 `Partitioner` 接口；franz-go 实现 `Partitioner` 接口（`ForTopic` 方法）；Sarama 传入实现 `Partitioner` 接口的构造函数。注意自定义规则要保证**分区数变化时的兼容性**。

---

### Q9. acks 机制与消息可靠性配置

**标准回答：**

`acks` 决定 Producer 认为"发送成功"的判定标准：

| acks | 语义 | 风险 | 典型场景 |
| ---- | ---- | ---- | ---- |
| `0` | 发出即认为成功 | 网络抖动 / Leader 故障即丢 | 指标、日志等可容忍丢失的数据 |
| `1` | Leader 写入本地日志即返回 | Leader 宕机且 Follower 未同步时有丢失窗口 | 一般业务 |
| `-1`（= `all`） | **ISR 中所有副本写入成功才返回** | 延迟升高；ISR 退化时保护失效（需配合 min.insync.replicas） | 订单、支付等核心链路 |

关键组合（必背）：

- `acks=all` 等的是 **ISR 内所有副本**，不是所有副本；
- 必须配合 `min.insync.replicas`（Broker / Topic 级，默认 1，生产建议 2）：当 **ISR 大小 < min.insync.replicas** 时，生产请求被**拒绝**（`NotEnoughReplicasException`），宁可不可写也不丢数据；
- `retries` 默认无限重试（`2147483647`），受 `delivery.timeout.ms` 总时长约束；
- Kafka **3.0+** 中 Producer 默认 `acks=all` 且 `enable.idempotence=true`（老版本默认 acks=1）。

**追问：`acks=all` + `min.insync.replicas=1` 有什么问题？**

等价于 acks=1 的可靠性：ISR 只剩 Leader 一个副本时也不会拒绝写入，此时 Leader 宕机照样丢消息。**正确姿势**：`replication.factor ≥ 3` + `min.insync.replicas ≥ 2` + `acks=all` 三者组合使用。

---

### Q10. 幂等生产者的原理

**标准回答：**

**要解决的问题**：Producer 重试导致的**消息重复**——写入成功但 ack 丢失，重试后同一消息被写入两次。

**核心机制**：

1. 幂等开启后，Producer 从 Broker 获取 **PID**（Producer ID，会话内唯一）；
2. 每条消息携带 **`(PID, Partition, SequenceNumber)`**，每个分区内 Sequence 从 0 单调递增；
3. Broker 端按 `(PID, Partition)` 维护最近写入的 Sequence，只接受 `seq == lastSeq + 1` 的写入：
   - `seq <= lastSeq`：判定为**重复**，直接丢弃并返回成功；
   - `seq > lastSeq + 1`：判定为**乱序**，返回 `OutOfOrderSequenceException`。

**保证边界**：

- 只保证**单生产者会话 + 单分区**的幂等；PID 随会话重启变化，**不跨会话**、**不跨分区**；
- 需要跨会话 / 跨分区原子性时使用**事务**（Q11）；
- 开启幂等后 `max.in.flight ≤ 5` 仍能保证有序（Broker 会缓存乱序到达的 batch 并按序应用）；
- 3.0+ 默认开启（`enable.idempotence=true`）。

**Go 视角**：franz-go 中 `kgo.EnableIdempotentWrite()`（或 `kgo.RequiredAcks(kgo.AllISRAcks())` 自动联动）；Sarama 中 `Config.Producer.Idempotent = true` 且需 `Net.MaxOpenRequests = 1`。

**追问：幂等能保证 Exactly-Once 吗？**

不能单独保证。幂等只解决"生产端重试重复"，消费端重复消费、跨会话重复仍需事务或下游幂等兜底。

---

### Q11. 事务与 Exactly-Once 语义的实现

**标准回答：**

Kafka 事务解决两类问题：

1. **跨分区、跨会话的原子写入**（要么全成功，要么全回滚）；
2. **consume-transform-produce** 模式下的精确一次（消费位点提交也纳入事务）。

**核心机制**：

- 用户指定 **`transactional.id`**（跨会话稳定）→ Broker 据此分配新的 PID，并通过 **epoch 递增 fence 掉旧实例**（僵尸 Producer 的写入被拒绝）；
- **两阶段提交**：`beginTransaction()` → 发送消息 / `sendOffsetsToTransaction()`（把消费位点一并提交）→ `commitTransaction()` 或 `abortTransaction()`；
- **事务协调器**（TransactionCoordinator）管理事务状态，事务元数据写入内部 topic `__transaction_state`（默认 50 分区，compact）；
- 消费端需设置 **`isolation.level=read_committed`**：只消费已提交事务的消息，过滤未提交与已回滚消息；引入 **LSO（Last Stable Offset）**——消费位点停在第一个未完成事务之前，保证不读到"悬而未决"的事务。

**注意**：Exactly-Once 的本质是"**幂等 + 事务**"，且以 Kafka 为链路时才完整成立；下游若是 DB/HTTP 等其他系统，需业务侧幂等（唯一键、upsert）兜底。

**Go 视角**：franz-go 提供完整事务 API（`BeginTransaction` / `EndTransaction` + `read_committed`）；Sarama 从 v1.38 起才支持事务，API 较为繁琐。

---

### Q12. Kafka 会丢消息吗？如何避免

**标准回答：**

会。必须分**三端**排查（面试按此结构答最加分）：

**1. Producer 端**

- `acks=0/1` 或异步发送不检查回调结果 → 配置 `acks=all` 并处理 callback error；
- `retries=0` → 使用默认重试 + 合理的 `delivery.timeout.ms`；
- 缓冲区满（`buffer.memory` 32MB）被阻塞/丢弃 → 监控发送阻塞，必要时加大缓冲。

**2. Broker 端**

- `replication.factor=1` → 单副本无冗余，故障必丢 → 至少 3；
- `min.insync.replicas=1` 配合 acks=all 保护失效 → 设 2；
- `unclean.leader.election.enable=true` → 允许非 ISR 副本当 Leader，丢数据 → 保持 false；
- 单机断电丢页缓存数据 → 靠多副本跨机架（`rack` 感知）兜底，而不是依赖 fsync。

**3. Consumer 端**

- 自动提交 + "先提交后处理"（at-most-once）→ 改为**处理成功后手动提交**；
- 多线程处理时提交位点与处理进度错配 → 按分区粒度管理位点，处理完才提交。

**一句话模板**：`acks=all + replication.factor=3 + min.insync.replicas=2 + 手动提交 + 端到端幂等`。

**追问：Kafka 0.11 之前的 HW 机制为什么可能丢数据？**

老版本消费者仅依据 HW 判断可见数据，Leader 切换时若新 Leader 的 HW 尚未更新到最新，已消费确认过的数据可能被"回退"，同时副本的日志截断可能丢弃已写数据。KIP-320 引入 **Leader Epoch**，由 Epoch 校验副本与消费者的日志一致性，缓解该问题。

---

## 三、Broker 与副本机制（Q13-Q18）

### Q13. ISR、LEO、HW 三者的关系（必背）

**标准回答：**

三个概念及关系：

- **LEO（Log End Offset）**：每个副本自己下一条待写入消息的 offset（即日志末尾 + 1）；
- **HW（High Watermark）**：**ISR 中所有副本 LEO 的最小值**，是"已提交"边界；**消费者只能消费 offset < HW 的消息**；
- **ISR（In-Sync Replicas）**：与 Leader 保持同步的副本集合（包含 Leader 自身），其余为 OSR。

**ISR 的进出规则**：

- Follower 落后：最后同步时间距今超过 `replica.lag.time.max.ms`（默认 30s）→ 被踢出 ISR；
- 2.5 之前用"落后消息条数"（`replica.lag.max.messages`）判断，低流量时会误判，已废弃；
- Follower 追上 Leader 的 LEO 后可重新加入 ISR。

**HW 的推进过程**：

1. Follower 向 Leader 发 fetch（带上自己的 LEO）；
2. Leader 用各副本（含自身）LEO 的最小值更新候选 HW，并随 fetch 响应返回；
3. Follower 收到后更新自己的 HW。

**意义**：HW 之前的消息已至少在 ISR 全部副本落地，Leader 故障时新 Leader 一定包含这些数据，不会丢。

**追问：HW 机制有什么已知缺陷？**

HW 更新是**异步**的：所有副本只在 fetch 环节推进 HW。极端场景（Follower 重启先按 HW 截断日志、再向 Leader 拉取）可能出现短暂的数据"回退"与不一致；KIP-320 的 Leader Epoch + 离线副本校验用于检测和缓解此类日志不一致。

---

### Q14. Leader 选举与 Unclean Leader Election

**标准回答：**

**正常选举**：分区 Leader 故障后，Controller 感知并**从 ISR 中挑选存活副本**成为新 Leader（优先选择……实际实现按 ISR 顺序选择第一个可用副本），并通知所有相关 Broker 与客户端元数据更新。

**ISR 全挂时的抉择**（`unclean.leader.election.enable`）：

| 配置 | 行为 | 后果 | 定位 |
| ---- | ---- | ---- | ---- |
| `false`（默认） | 等待 ISR 中第一个副本恢复后再选 Leader | 分区**长时间不可用** | CP：一致性优先 |
| `true` | 从 OSR（非同步副本）中强行选 Leader | 恢复可用，但**必然丢消息**（选出的副本数据较旧） | AP：可用性优先 |

**工程建议**：订单 / 支付等核心数据保持 `false`；埋点、日志类可容忍丢数据的场景可评估开启。

KRaft 模式下分区 Leader 选举仍由 Controller 主导，但 Controller 本身通过 **Raft Quorum** 选主与复制元数据，故障切换从秒级降到亚秒级（见 Q26）。

**追问：为什么默认从 ISR 选 Leader？**

ISR 副本的 LEO 接近 Leader，数据最完整；从 ISR 选举能保证**已提交（HW 之前）的数据不丢**。从 OSR 选举一定丢数据，所以默认关闭。

---

### Q15. 副本同步机制：AR、ISR、OSR 与故障恢复

**标准回答：**

**集合关系**：AR（Assigned Replicas，所有副本）= ISR + OSR。

**同步方式**：Follower **主动拉取（fetch）**，而不是 Leader 推送——天然具备**背压**能力（Follower 处理不过来就少拉，Leader 不会被打垮）。

**故障恢复路径**：

| 故障 | 处理 |
| ---- | ---- |
| Follower 卡顿 / 宕机 | 超时踢出 ISR（进 OSR），恢复后持续追 LEO，追上后重新入 ISR |
| Leader 宕机 | Controller 从 ISR 选新 Leader；幸存 Follower 按新 Leader 的日志状态**截断多余数据**（Leader Epoch 校验一致点）后开始同步 |
| Leader + 部分副本同时挂 | 若剩余 ISR 非空 → 正常选举；若 ISR 全挂 → 决定是否 Unclean 选举 |

**落盘**：Kafka 默认不强制 fsync（由 OS 异步刷盘），持久性由**多副本**保障；`log.flush.*` 系列参数官方建议保持默认。

**追问：为什么 Follower 用"拉"而不是 Leader "推"？**

1. 简化 Leader 逻辑，Leader 不需要跟踪每个 Follower 的进度；
2. 天然背压，Follower 慢不会拖垮 Leader；
3. 便于批量合并（一次 fetch 拿一批），吞吐更高。

---

### Q16. Kafka 会重复消费吗？如何实现幂等消费

**标准回答：**

**会。** 重复消费的典型来源：

1. **Producer 重试**：已写入成功但 ack 丢失，重试导致 Broker 侧出现两条相同消息 → 用幂等生产 / 事务解决（Q10 / Q11）；
2. **Consumer 位点回退**：at-least-once 语义下，"处理完但提交 offset 前"崩溃 / 被踢出组，重启后从旧位点重读；
3. **Rebalance**：分区被转交给别的消费者，新消费者从已提交位点重新拉取（包含已处理但未提交的部分）；
4. **网络分区**：offset 提交失败或连接闪断，实际位点与服务端不一致。

**结论**：at-least-once 下重复**不可避免**，业务必须**幂等消费**：

- **数据库唯一键**：幂等表 / 唯一索引 + `INSERT IGNORE` / `ON CONFLICT DO NOTHING` / upsert；
- **Redis 去重**：`SETNX`（注意 TTL 与原子性）做消息 ID 去重；
- **状态机校验**：订单状态流转只接受合法前置状态（如"已支付 → 已发货"只能发生一次）；
- **事务型落库**：消息 ID 与业务写库在同一事务中提交。

**Go 视角**：worker pool 并发消费时，务必保证"**处理成功后**才提交该分区位点"，避免先提交后处理或错位提交。

**追问：Exactly-Once 能彻底杜绝重复吗？**

Kafka 事务 + read_committed 能保证**Kafka 内部链路**不重不丢；但一旦下游是外部系统（DB / HTTP / 第三方），必须在 Kafka 事务边界外做幂等兜底，端到端 EOS 是"工程组合"而非单一开关。

---

### Q17. 消费积压如何排查与处理（实战必考）

**标准回答：**

**监控指标**：Lag = LEO − ConsumerOffset（每分区），常用 kafka-exporter / Burrow / JMX 采集并告警。

**排查顺序**：

1. **消费者侧**：处理逻辑是否慢（下游 DB / RPC 超时）、GC 频繁、死锁阻塞、**是否频繁 Rebalance**（日志中 `Rebalance` 关键字）；
2. **并行度**：消费者数是否 < 分区数（此时加消费者无效，每加一个都闲置）；
3. **热分区**：key 分布倾斜导致个别分区 lag 极高，而整体不均衡；
4. **生产侧突增**：流量洪峰或上游重试风暴。

**处理手段（由轻到重）**：

- 优化消费逻辑：批量处理、并发 worker、连接池、异步化、设置超时；
- 提升并行度：消费者扩到 ≤ 分区数；仍不够则评估**扩分区**（注意 key 顺序语义与消费端适配）；
- 临时取舍：非关键消息跳过 / 转储到旁路队列异步补处理；
- 紧急手段：调大 `fetch.min.bytes` / 单批拉取量，缩短端到端处理链路。

**预防**：容量规划（生产峰值 × 处理余量）、Lag 告警、消费者幂等（允许安全重放）、死信队列（DLQ）兜底毒消息。

**Go 视角**：最常见根因是 goroutine 泄漏或下游调用**没有 context 超时**（一个卡住的请求拖垮整条消费链路）；另外同步 commit 阻塞在消费循环里也会显著降低吞吐。

---

### Q18. 如何保证消息的顺序性（必背）

**标准回答：**

Kafka 只保证**分区内有序**。全局有序必须单分区（吞吐骤降，仅适用于极低吞吐的强顺序场景），工程上的标准做法：

1. **业务 key 设计**：把"需要保序的实体 ID"作为 key（`orderId` / `userId`），同实体落同分区 → 分区内天然有序；
2. **生产者端防重试乱序**：
   - **首选**：开启幂等（PID + Sequence，重试在 Broker 侧被去重，`max.in.flight ≤ 5` 仍有序）；
   - 备选：`max.in.flight.requests.per.connection = 1`（牺牲吞吐，不推荐）；
3. **消费端串行**：同一分区消息串行处理；或用"分区级路由"并发模型——同一分区的消息固定派发给同一个 worker（`partition % N` 取模），并行但保序；
4. **运维注意**：避免频繁 Rebalance 打断处理；消费位点"处理完成后提交"。

**Go 视角**：Sarama 消费者组回调按 Claim 串行，天然单分区顺序；franz-go 的 `PollFetches` 循环单线程拉取，若自行并发处理，必须保证"**同分区串行**"，否则顺序被 worker 并发打破。

**追问：多分区下如何做"跨分区有序"？**

Kafka 不原生支持。可选方案：单分区 topic；或在业务层做跨分区协调（按序合并、水位线等待），复杂度高，通常建议重新审视是否真的需要全局顺序。

---

## 四、消费者与消费组（Q19-Q24）

### Q19. 消费者组与 Rebalance：触发条件与过程

**标准回答：**

**Consumer Group**：多个消费者组成一组协同消费一个或多个 Topic。两条铁律：

1. **组内一个分区同一时刻只能被一个消费者消费**（互斥）；
2. **不同组之间互相独立**，各自消费全量数据（发布-订阅语义）。

**Rebalance（重平衡）**：组内消费者重新分配分区所有权的过程。

**触发条件**（4 个，必背）：

1. 消费者**加入**组（新实例上线 / 扩容）；
2. 消费者**离开**组（主动 close，或崩溃超时被踢出）；
3. 订阅的 Topic **分区数变化**（如扩分区）；
4. 订阅列表变化（正则订阅匹配到了新 Topic）。

**经典 Eager 协议过程**：

1. 任一条件触发 → **全组停止消费**（Stop-the-World）；
2. **JoinGroup**：所有成员向 GroupCoordinator 报到，Coordinator 指定一个 leader 消费者；
3. **SyncGroup**：leader 按分配策略计算方案，经 Coordinator 下发给所有成员；
4. 成员按新分配持有分区，从各自已提交位点恢复消费。

**Eager 协议的三大问题**：

- 全组暂停消费（期间 Lag 上涨）；
- **全量撤销再分配**：即使 90% 的分区没变也要先全部释放；
- 规模瓶颈：组越大、分区越多，重平衡越慢（万级消费者组可达分钟级）。

**老协议下的优化手段**：

- 会话参数匹配业务：`session.timeout.ms`（3.0+ 默认 45s）、`heartbeat.interval.ms`（3s）、`max.poll.interval.ms`（5min，与单批处理耗时匹配，防止处理太慢被踢）；
- 使用 **CooperativeSticky** 分配器（增量撤销，避免全停）；
- **静态成员资格**（static membership，`group.instance.id`）：消费者重启后短时间内不触发重平衡（部分客户端支持，Java 官方客户端支持，Go 需确认客户端版本）。

**追问：Rebalance 与重复消费的关系？**

成员被撤销分区前需要在 revoke 回调中提交已处理位点；若提交不及时（崩溃/超时），新持有者从旧位点拉取 → 重复消费。所以"处理完尽快提交 + 幂等消费"是配套的。

---

### Q20. KIP-848 新一代消费组协议（4.0 重点）

**标准回答：**

**核心变化：协调逻辑从客户端转移到 Broker 端**。由服务端的 GroupCoordinator 直接计算分区分配并下放给消费者，客户端只负责"上报状态 + 接收分配"。

**与经典协议对比**：

| 维度 | Classic（Eager） | KIP-848 新协议 |
| ---- | ---- | ---- |
| 分配计算方 | 客户端 leader 消费者 | Broker 端 Coordinator |
| 重平衡粒度 | 全组 Stop-the-World | **增量式**：只调整变动分区，未变分区继续消费 |
| 成员管理 | 心跳线程 + max.poll.interval 判定 | 统一心跳接口，服务端跟踪成员状态 |
| 规模化 | 万级组可达数十秒 | **亚秒级**，十万级消费者 |
| 客户端启用 | 默认 | `group.protocol=consumer`（3.7 预览，4.0 GA，官方计划 5.0 默认） |

**价值**：滚动发布、弹性扩缩容场景下的"rebalance 风暴"大幅缓解；分配以**状态机**方式推进（revoking → assigning），分区迁移平滑，未受影响的分区**不中断消费**。

**Go 视角**：franz-go 对新协议跟进最快；Sarama / kafka-go 目前以 classic 协议为主。新旧客户端可以并存（不同组各自选择协议），但**同一个组内**应保持一致。

**追问：新协议下 max.poll.interval.ms 还管事吗？**

新协议移除了这一概念——成员活跃性由服务端按心跳管理，"处理太慢导致被踢出组"的经典故障模式被弱化；但消费端仍要控制单批处理时长，只是不再因此触发全组重平衡。

---

### Q21. 分区分配策略有哪些？怎么选

**标准回答：**

| 策略 | 原理 | 优缺点 |
| ---- | ---- | ---- |
| **Range** | 按 Topic 逐个将分区按范围均分 | 简单；订阅多 Topic 时排序靠前的消费者多分，易倾斜 |
| **RoundRobin** | 所有 Topic 的分区排序后轮询分配 | 更均匀；成员变动时迁移范围大 |
| **Sticky** | 尽量沿用上次分配，只迁移必要分区 | 减少迁移与重平衡开销 |
| **CooperativeSticky** | Sticky + **增量协作撤销**（KIP-429） | 避免全组 Stop-the-World，**推荐默认** |

**选择建议**：

- 默认用 CooperativeSticky；老客户端至少用 Sticky；
- Go 客户端：Sarama 提供 Range / RoundRobin / Sticky（`BalanceStrategy`），franz-go 与 Java 客户端语义对齐；
- 注意同一组内**所有成员使用相同分配策略**，混用会退化为默认行为。

**追问：为什么 Range 会倾斜？**

Range 对每个 Topic 单独按"消费者字典序"切分：分区数不能被消费者数整除时，排在前面的消费者每次都多拿一个；若订阅列表不同（C1 订阅 T1+T2，C2 只订阅 T2），T1 全部分区都会给 C1。业务层的**热 key 倾斜**（某 key 流量巨大）与分配策略无关，需要在 key 设计上打散。

---

### Q22. Offset 提交方式与三种消费语义

**标准回答：**

**提交方式**：

1. **自动提交**：`enable.auto.commit=true`，每 `auto.commit.interval.ms`（默认 5s）随 poll 提交**上一批返回消息的位点**。时间点不可控：可能未处理先提交（丢）或处理完崩溃来不及提交（重）；
2. **手动同步提交** `commitSync`：处理完成后提交，失败阻塞重试，可靠但增加延迟；
3. **手动异步提交** `commitAsync`：不阻塞，失败走回调；注意重试时的"旧位点覆盖新位点"问题（用提交序号比对防护）。

**三种消费语义**：

| 语义 | 做法 | 风险 |
| ---- | ---- | ---- |
| at-most-once | **先提交位点，再处理** | 处理失败 → 丢消息 |
| at-least-once | **先处理，再提交位点**（生产推荐） | 崩溃/超时 → 重复消费，需业务幂等 |
| exactly-once | 事务 + `read_committed`，或下游幂等 | 实现复杂；跨外部系统仍需幂等 |

**生产组合拳**：手动提交 + 批量处理完成后提交 + 业务幂等 = 可靠的"最少一次"。

**Go 视角**：franz-go 用 `AutoCommitMarks` / `CommitRecords` 精确控制"处理完成才提交"；Sarama 关心 `Consumer.Offsets.AutoCommit.Enable` 开关与 `MarkMessage` 的语义差异。

---

### Q23. __consumer_offsets 是什么？位点怎么重置

**标准回答：**

- Kafka **0.9 起**，消费位点从 ZooKeeper 迁移到内部 topic **`__consumer_offsets`**（默认 **50 个分区**，`cleanup.policy=compact`）；
- 存储结构：Key = `(group, topic, partition)`，Value = `(offset, metadata, commitTimestamp)`；`hash(group)` 决定写落到哪个分区；
- 查询位点：`kafka-consumer-groups.sh --describe --group <g>`；API 如 franz-go `FetchOffsets`；
- **位点重置**：
  - 无位点或位点越界时按 **`auto.offset.reset`**：`earliest` / `latest` / `none`（抛异常，需业务处理）；
  - 手动重置：`--reset-offsets --to-earliest / --to-datetime / --to-offset / --shift-by`（要求该组**没有活跃消费者**）；
- 注意：`__consumer_offsets` 的分区数创建后**不可更改**，配置过小会成为大量消费组位点写入的热点。

**追问：位点越界的经典场景？**

消费组离线时间超过 `retention`，数据已被删除 → 已提交位点指向不存在的 offset → 按 `auto.offset.reset` 重新定位，可能触发大量回溯消费（Lag 突增）或直接跳到 latest 丢数据处理窗口。业务上要对"长期离线的组"做预案。

---

### Q24. 如何设计一个可靠的 Golang 消费者（实战）

**标准回答：**

设计要点清单（面试按此展开）：

1. **生命周期**：`signal.NotifyContext` 捕获 SIGTERM → cancel context → 停止拉取 → 处理完在途消息 → 提交位点 → 关闭连接；
2. **位点**：手动提交，处理成功后提交；批处理时"整批成功才提交"；
3. **幂等**：业务唯一键去重（DB 唯一索引 / Redis SETNX），允许安全重放；
4. **并发模型**：单 goroutine 拉取 + worker pool 处理；**同分区消息路由到同一 worker**（按 partition 取模）保证分区内有序；
5. **错误处理**：区分可重试（网络/超时）与不可重试（数据错误）；毒消息反复失败进**死信队列（DLQ）**后提交位点，避免阻塞整批；
6. **监控**：处理延迟、Lag、Rebalance 次数、失败率、DLQ 积压；
7. **优雅停机**：给在途处理设定超时预算，防止 SIGTERM 后无限等待。

代码骨架（franz-go）：

```go
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

	client, err := kgo.NewClient(
		kgo.SeedBrokers("kafka-1:9092", "kafka-2:9092"),
		kgo.ConsumerGroup("order-group"),
		kgo.ConsumeTopics("order-events"),
		kgo.DisableAutoCommit(),    // 手动提交位点
		kgo.BlockRebalanceOnPoll(), // poll 期间阻止分区被收走，与提交配合
	)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	for {
		fetches := client.PollFetches(ctx)
		if ctx.Err() != nil { // 收到退出信号，优雅结束
			break
		}
		fetches.EachError(func(t string, p int32, err error) {
			// 记录分区级错误，必要时延迟重试
		})
		fetches.EachRecord(func(r *kgo.Record) {
			handle(r) // 业务处理（幂等设计）
		})
		// 所有记录处理完成后统一提交，保证 at-least-once
		client.CommitUncommittedOffsets(ctx)
	}
}
```

**追问：为什么用 BlockRebalanceOnPoll？**

poll 到的一批消息正在处理时，若发生 Rebalance 把分区转走，位点提交会失败、消息可能被其他消费者重复消费。`BlockRebalanceOnPoll` 让"处理 + 提交"完成后再允许重新分配，牺牲一点重平衡速度，换取处理一致性。

---

## 五、高级特性与选型（Q25-Q30）

### Q25. KRaft 是什么？4.0 为什么彻底移除 ZooKeeper（必考）

**标准回答：**

**KRaft**（Kafka Raft metadata mode，KIP-500 提出）用 Kafka **自身实现的 Raft 共识算法**管理元数据，取代 ZooKeeper。

**为什么抛弃 ZooKeeper**（4 点）：

1. **运维复杂度**：需要额外部署/监控/升级一个独立 ZK 集群，且两个系统的安全、调参、版本升级互相牵连；
2. **扩展性瓶颈**：ZK 采用**全量元数据推送**模型，社区实践中分区数上限约 20 万；KRaft 支持**百万级分区**；
3. **故障恢复慢**：Controller 切换依赖 ZK 会话超时（秒级到分钟级）；KRaft 基于 Raft 选举，**亚秒级**完成；
4. **架构割裂**：元数据一致性在 ZK（ZAB 协议）、消息一致性在 Kafka，两套模型；KRaft 统一为 Raft。

**KRaft 核心原理**：

- 元数据存储为**单分区内部 topic `__cluster_metadata`** 的 Raft 日志；
- **Controller Quorum**（1/3/5 节点）选举 Active Controller 并复制日志，其他 Controller 作为 Follower；
- Broker 作为**观察者**实时同步元数据日志到本地，Controller 故障时无需全量重新拉取；
- 支持元数据**快照（Snapshot）**，加速启动与故障恢复（从分钟级优化到秒级）。

**版本节奏**：2.8 预览 → 3.3 生产可用 → 3.5 ZK 模式标记弃用 → **4.0 彻底移除**（不再支持 ZK 模式启动，也不支持从 ZK 直接升级，必须先升到 3.x 完成迁移）。

**追问：Controller 和 Broker 能合并部署吗？**

可以（Combined 模式，同一进程两种角色），适合小规模集群降低部署成本；生产大规模推荐**分离部署** Controller Quorum，避免元数据操作与数据吞吐相互干扰。

---

### Q26. 共享组（Share Group）与队列模型（4.0 新特性）

**标准回答：**

KIP-932 引入 **Share Group（共享组）**，把"队列"语义引入 Kafka，解决传统消费者组的两个限制：分区与消费者绑定（消费者数 ≤ 分区数）、同一分区不能被多个消费者并行消费。

**核心机制**：

- 组内**多个消费者可同时消费同一分区**——并行粒度从"分区"降到"记录"；
- Broker 维护**记录级锁**：一条消息被某消费者取出后加锁（租约 + 超时），未确认前其他消费者不可见，超时后重新投递；
- 客户端**逐条 ACK / NACK**（不再是整体提交 offset）；
- 默认为 at-least-once，配合业务幂等可接近精确一次。

**与消费者组对比**：

| 维度 | Consumer Group | Share Group |
| ---- | ---- | ---- |
| 并行粒度 | 分区 | 记录 |
| 消费者上限 | ≤ 分区数 | 远超分区数 |
| 确认方式 | offset 提交 | 逐条 ACK / NACK + 记录锁 |
| 顺序保证 | 分区内有序 | 无顺序保证 |
| 典型场景 | 流处理、高吞吐管道 | 任务队列、处理耗时差异大、弹性扩缩容 |

**版本状态**：4.0 引入（预览特性）；Go 生态中 franz-go 已提供实验性支持。

**追问：共享组会取代消费者组吗？**

不会。消费者组仍是**流处理 / 多播 / 有序消费**的主模型；共享组只补充"任务分发"类场景（弱顺序、耗时不均、消费者数弹性大，类似 SQS 的定位）。

---

### Q27. 消息压缩机制与算法选型

**标准回答：**

- 压缩发生在 **Producer**（`compression.type`），作用于**整批消息**（batch）；
- **Broker 默认保持 Producer 的压缩方式，不解压**（0.11 新消息格式之后），消费者端负责解压；
- 收益 = 网络传输 ↓ + 磁盘占用 ↓ + 有效吞吐 ↑；代价 = CPU 换（Producer 压缩、Consumer 解压）；
- batch 越大压缩率越高 → 压缩需与 `batch.size` / `linger.ms` 搭配调优。

**算法对比**：

| 算法 | 压缩比 | 速度 | 备注 |
| ---- | ---- | ---- | ---- |
| gzip | 高 | 慢 | 兼容性最好 |
| snappy | 中 | 快 | 通用平衡 |
| lz4 | 中 | 很快 | 低延迟场景 |
| **zstd** | 高 | 较快 | 2.1+ 支持，压缩比与速度兼得，**当前首选** |

**注意点**：

- 确认客户端与 Broker 版本支持所选算法（zstd 需 2.1+）；
- `max.message.bytes` 限制的是**压缩后**的大小，压缩后仍超限会被拒绝；
- Broker 端 recompress（`compression.type` 配为非 producer 值）有额外 CPU 开销，非必要不改。

**追问：压缩会影响消费端吗？**

会。解压 CPU 开销转移到消费者，且只能**整批解压**（无法只解一条）。Go 客户端注意所选压缩算法的实现依赖（部分算法需要 CGO / 纯 Go 实现的差异）。

---

### Q28. 分区数怎么定？有哪些核心调优参数

**标准回答：**

**分区数估算**（两维度取大）：

1. **吞吐维度**：分区数 ≥ 目标吞吐 / 单分区可达吞吐（经验值单分区约 10MB/s 量级，视 batch 与压缩而定，保守取值）；
2. **并行度维度**：分区数 ≥ 消费峰值并行度（消费者数量上限）；

再向上取整到合适的数值，并**预留增长空间**（只能增不能减）。

**权衡（分区不是越多越好）**：分区越多 → 元数据/文件句柄/内存开销越大、Rebalance 越慢、端到端延迟略增；旧 ZK 集群级上限约 20 万分区（KRaft 支持百万级），单 Broker 建议控制在数千以内。

**核心调优参数**（完整默认值见附录 A）：

| 端 | 参数 | 建议 |
| ---- | ---- | ---- |
| Producer | `batch.size` / `linger.ms` | 增大换取吞吐（如 64KB~1MB / 5~100ms） |
| Producer | `compression.type` | zstd |
| Producer | `acks` / `enable.idempotence` | all / true |
| Broker | `num.io.threads` / `num.network.threads` | 按 CPU 核数调整 |
| Broker | `log.segment.bytes` / 保留策略 | 按数据量规划 |
| Consumer | `fetch.min.bytes` / `max.partition.fetch.bytes` | 提高拉取批量效率 |
| Consumer | `max.poll.records` | 控制单批处理时长（老协议防踢） |

**追问：分区 key 怎么设计？**

按"业务并发单元"划分（如 `orderId`、`userId`），保证同实体有序；避免 key 分布倾斜产生热分区；分区数属于**架构决策**，上线前评估好，事后只能扩不能缩。

---

### Q29. Kafka / RabbitMQ / RocketMQ 如何选型

**标准回答：**

| 维度 | Kafka | RabbitMQ | RocketMQ |
| ---- | ---- | ---- | ---- |
| 模型 | 分区日志、发布订阅 | AMQP 队列、复杂路由 | 队列 / 发布订阅 |
| 吞吐 | 十万级+ | 万级 | 十万级 |
| 延迟 | ms 级 | 微秒~ms 级（最低） | ms 级 |
| 顺序 | 分区内有序 | 队列内有序 | 队列内有序 |
| 延迟消息 | 无（需自研/框架） | 无（插件） | **原生多级延迟** |
| 消息回溯 | **支持**（offset 回放） | 消费即删 | 支持（按时间/位点） |
| 事务 | 支持（事务 + 幂等） | 弱 | 事务消息 |
| 典型场景 | 日志、流处理、大数据管道 | 企业集成、复杂路由、低延迟小消息 | 电商交易、延迟场景 |

**选型话术**：

- 高吞吐、流式处理、与大数据生态（Flink / Spark / Connect）对接 → **Kafka**；
- 复杂路由（Exchange 类型多）、低延迟、协议丰富（AMQP / MQTT / STOMP）→ **RabbitMQ**；
- 交易场景、需要延迟消息、国内生态与运维体系 → **RocketMQ**。

**追问：为什么日志/大数据场景几乎都选 Kafka？**

生态完整（Connect / Streams / Flink 集成）+ 吞吐量级 + 消息保留与回放能力 + 水平扩展性，四者综合无对手。

---

### Q30. 跨机房容灾怎么设计？MirrorMaker 2 是什么

**标准回答：**

**思路分两层**：

1. **集群内副本跨机房（不推荐）**：异地网络 RTT 高，易导致 ISR 抖动、HW 推进缓慢、生产延迟拉高，仅适合同城机房（RTT < 2ms）；
2. **集群间复制（推荐）**：用 **MirrorMaker 2**（基于 Kafka Connect）把源集群数据异步复制到目标集群。

**MirrorMaker 2 能力**：

- Topic 复制（支持改名、正则过滤）；
- **消费位点同步**（offset 转换），主备切换后消费者接续消费；
- 心跳 / 检查点 topic，支持监控复制延迟；
- 支持**双向复制**（active-active 双活）。

**局限**：本质是"消费再生产"的**异步复制**，RPO > 0；跨集群不保证 Exactly-Once，需要业务幂等兜底。

**典型架构**：

- **同城双活**：两集群互备，生产者就近写 / 双写，消费者就近读，切换靠 DNS / 网关；
- **两地三中心**：本地双可用区 + 异地灾备，MM2 异步同步，定期演练；
- **单元化**：按业务单元拆分，各单元自闭环 + 中心异步汇聚。

**追问：为什么不用 Kafka 自带副本做跨城容灾？**

`acks=all` 要求 ISR 全部落盘，跨城复制延迟无法满足高吞吐写入；且跨机房 ISR 裁决在脑裂场景下很脆弱（KRaft 的 quorum 同样怕跨城延迟）。异步集群复制（接受可控 RPO）是工程上的成熟答案。

---

## 附录 A：核心参数速查表（面试默写版）

> 默认值以官方 Java 客户端 / Broker 口径为准。

**Producer 侧**：

| 参数 | 默认值 | 说明 |
| ---- | ---- | ---- |
| `acks` | `all`（3.0+） | 0 / 1 / all |
| `enable.idempotence` | `true`（3.0+） | 幂等生产 |
| `retries` | `2147483647` | 重试次数（受 delivery.timeout.ms 约束） |
| `max.in.flight.requests.per.connection` | `5` | 幂等开启时仍保序 |
| `batch.size` | `16KB` | 批次大小 |
| `linger.ms` | `0` | 发送等待（建议 5~100） |
| `buffer.memory` | `32MB` | 本地缓冲 |
| `delivery.timeout.ms` | `120000` | 发送总超时 |
| `compression.type` | `none` | 建议 zstd |
| `max.request.size` | `1MB` | 单请求上限 |

**Broker 侧**：

| 参数 | 默认值 | 说明 |
| ---- | ---- | ---- |
| `num.partitions` | `1` | 自动建 Topic 默认分区数 |
| `default.replication.factor` | `1` | 生产建议 3 |
| `min.insync.replicas` | `1` | 生产建议 2 |
| `unclean.leader.election.enable` | `false` | 保持 false |
| `replica.lag.time.max.ms` | `30s` | ISR 踢出判定 |
| `log.segment.bytes` | `1GB` | Segment 大小 |
| `log.retention.hours` | `168` | 7 天 |
| `log.retention.check.interval.ms` | `5min` | 清理检查周期 |
| `message.max.bytes` | `1MB` | 单消息上限（压缩后大小） |

**Consumer 侧**：

| 参数 | 默认值 | 说明 |
| ---- | ---- | ---- |
| `enable.auto.commit` | `true` | 生产建议关闭，改手动 |
| `auto.commit.interval.ms` | `5000` | 自动提交间隔 |
| `auto.offset.reset` | `latest` | earliest / latest / none |
| `fetch.min.bytes` | `1` | 增大可提升吞吐 |
| `fetch.max.bytes` | `~50MB` | 单次 fetch 上限 |
| `max.partition.fetch.bytes` | `1MB` | 单分区单次 fetch 上限 |
| `max.poll.records` | `500` | 单次 poll 条数 |
| `session.timeout.ms` | `45s`（3.0+） | 会话超时 |
| `heartbeat.interval.ms` | `3s` | 心跳间隔 |
| `max.poll.interval.ms` | `5min` | 处理间隔上限（老协议） |
| `isolation.level` | `read_uncommitted` | 事务消费改 read_committed |

---

## 附录 B：Kafka 版本演进时间线

| 时间 | 版本 | 关键事件 |
| ---- | ---- | ---- |
| 2011 | - | Apache 开源 |
| 2012 | 0.8 | 引入副本机制 |
| 2015 | 0.9 | 新 Consumer API；位点迁移至 `__consumer_offsets` |
| 2017 | 0.11 | 幂等 Producer、事务、Exactly-Once、新消息格式（batch + CRC） |
| 2017 | 1.0 | 稳定性里程碑 |
| 2018 | 2.1 | zstd 压缩支持 |
| 2020 | 2.4 | 粘性分区（KIP-480） |
| 2021 | 2.8 | KRaft 预览（KIP-500） |
| 2021 | 3.0 | `acks=all` + 幂等默认开启；废弃 Java 8 |
| 2022 | 3.3 | KRaft 生产可用 |
| 2023 | 3.5 | ZooKeeper 模式弃用 |
| 2023 | 3.7 | KIP-848 新版消费组协议客户端预览 |
| 2025 | 4.0 | **KRaft only（移除 ZK）**；KIP-848 GA；KIP-932 共享组预览；Broker 要求 Java 17 |

---

## 附录 C：进阶加分题（简答）

**1. Kafka 怎么实现延迟消息？**

Kafka 无原生延迟消息。常见方案：延时 topic（按延迟级别分 topic）+ 消费端时间轮重投（未到期写回）；外部调度（Redis ZSet / 定时任务）到点后再投 Kafka；或直接选 RocketMQ（原生多级延迟）。

**2. 日志压缩（Compaction）适合什么场景？**

按 key 保留最新值、后台合并旧段，适合**状态/快照类**数据：消费位点、CDC 变更日志、维表重建。删除 key 用 tombstone（value=null，保留一段 delete.retention.ms 后清除）。

**3. Kafka 监控体系怎么搭？**

JMX 指标 → kafka-exporter → Prometheus → Grafana/告警。核心指标：**Consumer Lag**、UnderReplicatedPartitions（ISR 收缩）、OfflinePartitions、请求 P99、磁盘使用、Controller 状态、Rebalance 次数。

**4. 从 3.x 升级到 4.0 要注意什么？**

必须先把 ZooKeeper 模式迁移到 KRaft（3.x 提供双写迁移工具）；4.0 移除了旧协议版本（协议基线 ≥ Kafka 2.1），老客户端需先升级；滚动升级，先 Broker 后客户端，确保兼容后再升。

**5. 为什么 Kafka 选择"多副本"而不是"强制刷盘"保证持久性？**

fsync 会破坏顺序写吞吐（机械盘尤其明显），且单机刷盘仍防不住整机故障；多副本把持久性下移到集群层面，单机故障由其他副本兜底，兼顾吞吐与可靠性（见 Q3 / Q15）。

---

## 附录 D：面试答题框架（话术模板）

回答任何一道 Kafka 题，按这个套路组织可稳定发挥：

1. **结论先行**：一句话给出核心答案（定调）；
2. **结构化展开**：2~4 个要点，能用表格/分点就用；
3. **给出配置组合**：带出生产参数（`acks=all + min.insync.replicas=2 + ...`），体现落地经验；
4. **Go 视角补充**：客户端（franz-go / Sarama）的行为差异与坑，形成差异化加分；
5. **说明边界**：什么场景不适用、有什么代价（如"分区内有序，全局无序"）。

> **文档维护提示**：Kafka 4.x 仍在快速迭代（KIP-848 默认化、KIP-932 GA、新延迟消息提案），建议每半年对照官方 Release Notes 修订一次本文档。

