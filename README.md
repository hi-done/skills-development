# skills-development

个人技术笔记与技能积累仓库，按主题分类整理开发中踩过的坑、读过的论文和沉淀的工程实践。

## 目录结构

```
.
├── AI_papers/     # AI 论文清单
├── Docker/        # Docker 配置
├── Git/           # Git 使用与版本规范
├── Golang/        # Go 语言笔记
├── Linux/         # Linux 常用操作
├── Mysql/         # MySQL 实践笔记
├── browser/       # 常用网址收藏
└── vscode/        # VSCode 配置
```

## 内容索引

### AI_papers

| 文件 | 内容 |
| ---- | ---- |
| [research.md](AI_papers/research.md) | 大模型必读论文清单：按年份梳理 2017~2025 关键论文（Transformer → DeepSeek-R1），并按架构、训练、对齐、推理、开源生态分类附 arXiv 链接，含 10 篇极简通读链 |

### Docker

| 文件 | 内容 |
| ---- | ---- |
| [配置.md](Docker/配置.md) | `daemon.json` 代理配置：通过设置 HTTP 代理解决镜像拉取失败问题，避免镜像源频繁失效 |

### Git

| 文件 | 内容 |
| ---- | ---- |
| [git常用指令.md](Git/git常用指令.md) | Commit 规范前缀表（feat/fix/BREAKING CHANGE 等与版本号联动）、远程关联、打标签推送、clone 代理设置 |
| [版本号规则.md](Git/版本号规则.md) | 语义化版本号（SemVer）详解：MAJOR/MINOR/PATCH 递增规则、预发布版本、版本后缀大全、npm/Go Modules 中的应用 |

### Golang

| 文件 | 内容 |
| ---- | ---- |
| [marshal.md](Golang/marshal.md) | `sonic.Marshal` vs `json.Marshal` 对比：性能差距、JIT/SIMD 加速原理、兼容性限制（架构、边缘 case）、使用场景选型 |
| [runtime.Callers.md](Golang/runtime.Callers.md) | `runtime.CallersFrames` 行号来源：编译期嵌入的 DWARF 调试信息、PC→行号查表机制、`-ldflags="-s -w"` 剥离的影响 |
| [协程池.md](Golang/协程池.md) | 协程池的本质：当底层资源（DB/Redis/IO/下游服务）存在上限时，必须池化以限制并发度，"并发度 ≠ 请求数" |
| [协程池、令牌桶、信号量.md](Golang/协程池、令牌桶、信号量.md) | 三种并发控制手段对比：控制目标、核心差异、适用场景、组合使用与选型决策树 |
| [方法不能加类型参数.md](Golang/方法不能加类型参数.md) | Go 泛型语言限制：方法不能加类型参数（只能给类型加），替代方案（类型级泛型、组合拆分）、官方设计原因 |

### Linux

| 文件 | 内容 |
| ---- | ---- |
| [常用操作.md](Linux/常用操作.md) | 查找并清理孤儿 gopls 进程、`ps` 命令列含义与 STAT 状态码解析 |

### Mysql

| 文件 | 内容 |
| ---- | ---- |
| [零行时的并发不超限.md](Mysql/零行时的并发不超限.md) | 地址上限并发控制：方案一 MySQL 行锁/间隙锁（RR 下 0 行也能锁，含索引对锁行为的影响）；方案二 Redis + Lua 原子计数（工业级高并发标准） |

### browser

| 文件 | 内容 |
| ---- | ---- |
| [常用网址.md](browser/常用网址.md) | 后端 / AI / 前端 / 工具 常用网址收藏 |

### vscode

| 文件 | 内容 |
| ---- | ---- |
| [setting.yaml](vscode/setting.yaml) | VSCode 配置：Go 开发性能优化（关闭 staticcheck/vulncheck 等高 CPU 项、gopls 轻量诊断配置）、code-runner 设置 |
