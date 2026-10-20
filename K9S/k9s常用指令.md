# K9s 常用指令速查

> 适用版本：K9s v0.40+。不同版本、不同 RBAC 权限下可用的按键会略有差异。
>
> 最重要的一条：**记不住就按 `?`**，它会显示当前视图真正可用的快捷键。
>
> 文中 `Ctrl-X` 指键盘上的 Ctrl 键；macOS 也是 Ctrl，不是 Command。

## 速记卡

| 想做什么 | 按键 / 命令 |
| --- | --- |
| 进入某个资源视图 | `:` 后输入 `po`、`dp`、`svc` 等 |
| 过滤当前列表 | `/` |
| 查看详情 | `d` |
| 查看 YAML | `y` |
| 查看日志 | `l` |
| 进入容器 | `s` |
| 编辑资源 | `e` |
| 删除资源 | `Ctrl-D` |
| 端口转发 | `Shift-F` |
| 切换全部命名空间 | `0` |
| 返回 / 清除过滤 | `Esc` |
| 当前视图帮助 | `?` |

## 1. 启动与 CLI 参数

| 命令 | 说明 |
| --- | --- |
| `k9s` | 启动，沿用上次的 context 和 namespace |
| `k9s -n kube-system` | 指定命名空间启动 |
| `k9s -A` | 所有命名空间启动 |
| `k9s -c pod` | 启动后直接进入 Pod 视图 |
| `k9s -c deploy -n prod` | 指定视图 + 命名空间 |
| `k9s --context prod-cluster` | 指定 kubeconfig context |
| `k9s --kubeconfig ./kubeconfig` | 指定 kubeconfig 文件 |
| `k9s --readonly` | 只读模式，禁用编辑/删除等修改操作，生产巡检推荐 |
| `k9s --write` | 强制可写，覆盖配置里的只读设置 |
| `k9s -r 1` | 刷新间隔，单位秒，可写小数 |
| `k9s --headless` | 隐藏顶部 Header |
| `k9s --logoless` | 隐藏 Logo |
| `k9s --crumbsless` | 隐藏面包屑 |
| `k9s --splashless` | 跳过启动画面 |
| `k9s --screen-dump-dir ./dumps` | 指定屏幕快照保存目录 |
| `k9s info` | 查看配置、日志、快照等路径，排障先看它 |
| `k9s version` | 查看版本 |
| `k9s help` | 查看全部命令行参数 |

## 2. 命令模式（冒号命令）

按 `:` 进入命令模式，输入资源名、别名或完整命令后回车。命令模式支持自动补全：

| 按键 | 作用 |
| --- | --- |
| `↑` / `↓` | 在候选命令间移动 |
| `Tab` / `→` / `Ctrl-F` | 接受当前候选 |
| `Ctrl-W` / `Ctrl-U` | 清空已输入内容 |
| `Esc` | 退出命令模式 |

### 2.1 常用资源视图

| 命令 | 资源 |
| --- | --- |
| `:po` / `:pod` | Pod |
| `:dp` / `:deploy` | Deployment |
| `:rs` | ReplicaSet |
| `:sts` | StatefulSet |
| `:ds` | DaemonSet |
| `:job` | Job |
| `:cj` / `:cronjob` | CronJob |
| `:svc` | Service |
| `:ep` | Endpoints |
| `:ing` | Ingress |
| `:cm` | ConfigMap |
| `:secret` | Secret |
| `:sa` | ServiceAccount |
| `:pvc` / `:pv` | PersistentVolumeClaim / PersistentVolume |
| `:sc` | StorageClass |
| `:hpa` | HorizontalPodAutoscaler |
| `:netpol` | NetworkPolicy |
| `:no` / `:node` | Node |
| `:ns` | Namespace |
| `:ev` | Event |
| `:crd` | CustomResourceDefinition |
| `:ctx` | kubeconfig Context |
| `:rbac` | RBAC 规则 |
| `:helm` | Helm Chart / Release |
| `:xray <po/svc/dp/rs/sts/ds>` | 资源拓扑视图 |
| `:pulses` / `:pu` | 集群健康面板 |
| `:screendump` / `:sd` | 已保存的屏幕快照 |
| `:aliases` | 当前版本所有资源别名 |

提示：按 `Ctrl-A` 可以直接弹出资源别名列表，不用记全。

### 2.2 命令参数组合

```text
:pod                          # 当前命名空间
:pod kube-system              # 指定命名空间
:pod /api                     # 按名称过滤（支持正则）
:pod app=nginx,env=prod       # 按标签过滤
:pod @prod-cluster            # 在另一个 context 中查看（会切换当前 context）
:ctx prod-cluster             # 直接切换 context
:xray dp                      # Deployment → ReplicaSet → Pod 拓扑
:xray dp default              # 指定命名空间
:q                            # 退出 K9s
```

`@context` 写法会真的切换当前 context，操作生产集群时先看清楚顶部显示的是哪个集群。

### 2.3 过滤器

在资源列表直接按 `/` 进入过滤模式：

```text
/fred             # 正则过滤，支持 fred|blee
/! fred           # 反向过滤，保留不匹配的内容
/-l app=web       # 按标签过滤
/-f ngin          # 模糊查找
```

`Esc` 清除过滤并返回。

## 3. 通用快捷键

### 3.1 导航

| 按键 | 作用 |
| --- | --- |
| `↑` / `k` | 上移 |
| `↓` / `j` | 下移 |
| `g` / `G` | 跳到顶部 / 底部 |
| `PgUp` / `PgDn` | 翻页 |
| `Tab` / `Shift-Tab` | 切换焦点（部分视图支持） |
| `Enter` | 进入选中的资源 |
| `Esc` | 返回上一级 / 清除过滤 |
| `?` | 当前视图帮助 |

### 3.2 资源操作（大部分视图通用）

| 按键 | 作用 | 备注 |
| --- | --- | --- |
| `d` | Describe | 查看事件、状态、挂载等信息 |
| `y` | 查看 YAML | |
| `e` | 编辑 | 只读模式或没有 edit 权限时不可用 |
| `Ctrl-D` | 删除 | 需要确认；批量删除前先标记 |
| `c` | 复制资源名 | |
| `n` | 复制命名空间 | |
| `w` | 跳到该资源所在命名空间 | 仅命名空间级资源 |
| `Ctrl-R` | 刷新 | |
| `Space` | 标记当前行 | 可配合 `Ctrl-D` 批量删除 |
| `Ctrl-Space` | 范围标记 | |
| `Ctrl-\` | 清除所有标记 | |
| `Ctrl-S` | 保存当前视图快照 | 用于保存现场，路径见 `k9s info` |
| `Ctrl-Z` | 显示 / 隐藏故障提示 | 不是挂起进程 |
| `Ctrl-W` | 宽列 / 窄列切换 | 看全部列时很有用 |
| `Shift-N` | 按名称排序 | |
| `Shift-A` | 按年龄排序 | |
| `Shift-S` | 按状态排序 | |
| `Shift-O` | 按当前选中列排序 | |

### 3.3 命名空间切换

| 按键 | 作用 |
| --- | --- |
| `0` | 所有命名空间 |
| `1` - `9` | 快速切换到收藏的命名空间 |
| `:ns` 后按 `u` | 打开命名空间列表并切换 |
| `w` | 跳到选中资源所属的命名空间 |

收藏命名空间在 context 配置里维护，数字键按收藏顺序分配。

## 4. 各资源视图专属快捷键

### 4.1 Pod

| 按键 | 作用 |
| --- | --- |
| `Enter` | 进入容器列表 |
| `l` | 查看日志 |
| `p` | 查看上一次容器日志（排查 CrashLoopBackOff 必备） |
| `s` | 进入容器 Shell |
| `a` | Attach 到容器 |
| `Shift-F` | 端口转发 |
| `f` | 查看当前端口转发 |
| `o` | 跳到该 Pod 所在的 Node |
| `Ctrl-K` | 强制 Kill，无二次确认 |
| `Ctrl-D` | 删除，有确认 |

### 4.2 Deployment / StatefulSet / DaemonSet / Job

| 视图 | 按键 | 作用 |
| --- | --- | --- |
| Deployment | `s` | 扩缩容 |
| Deployment | `r` | 滚动重启（等价 `kubectl rollout restart`） |
| Deployment | `z` | 查看关联的 ReplicaSet |
| StatefulSet | `s` / `r` | 扩缩容 / 滚动重启 |
| DaemonSet | `r` | 滚动重启 |
| Deployment / StatefulSet / DaemonSet | `Shift-F` | 端口转发 |
| Job | `l` | 查看日志 |
| 以上工作负载 | `e` / `y` / `d` | 编辑 / YAML / 详情 |

### 4.3 Service / Ingress

| 按键 | 作用 |
| --- | --- |
| `Shift-F` | 端口转发 |
| `b` | Benchmark 压测开关（Service） |
| `d` / `y` / `e` | 详情 / YAML / 编辑 |

### 4.4 Node

| 按键 | 作用 |
| --- | --- |
| `c` | Cordon，禁止新 Pod 调度 |
| `u` | Uncordon，恢复调度 |
| `r` | Drain，驱逐节点上的 Pod |
| `s` | 打开节点 Shell（需要开启 nodeShell 特性） |
| `d` / `y` | 详情 / YAML |

`r` Drain 属于高风险操作，会驱逐 Pod。执行前确认副本数、PDB 和业务影响。

### 4.5 其他常用视图

| 视图 | 按键 | 作用 |
| --- | --- | --- |
| Namespace | `u` / `Enter` | 切换使用该命名空间 |
| Context | `Enter` | 切换 context |
| Context | `r` / `Ctrl-D` | 重命名 / 删除 context 配置 |
| Secret | `x` | 解码 Secret 内容 |
| Secret | `u` | 查看被谁使用（UsedBy） |
| PVC | `u` | 查看被谁使用 |
| ServiceAccount | `Enter` | 查看权限规则 |
| ServiceAccount | `u` | 查看被谁使用 |
| CronJob | `t` | 手动触发一次 Job |
| CronJob | `s` | Suspend / Resume |
| Helm Chart | `r` | 查看 Release 列表 |
| Helm History | `r` | 回滚到指定版本 |
| XRay | `Enter` | 跳转到选中资源 |
| XRay | `l` / `p` | 日志 / 上一次日志 |
| XRay | `s` / `a` | Shell / Attach |
| XRay | `d` / `y` | 详情 / YAML |

### 4.6 容器视图

在 Pod 视图按 `Enter` 进入容器列表后：

| 按键 | 作用 |
| --- | --- |
| `s` | 进入容器 Shell |
| `a` | Attach |
| `Shift-F` | 端口转发 |
| `f` | 查看当前端口转发 |

## 5. 日志视图

从 Pod 或容器按 `l` 进入日志视图：

| 按键 | 作用 |
| --- | --- |
| `0` | Tail 模式，持续跟随最新日志 |
| `1` | Head 模式，从头开始 |
| `2` | 最近 1 分钟 |
| `3` | 最近 5 分钟 |
| `4` | 最近 15 分钟 |
| `5` | 最近 30 分钟 |
| `6` | 最近 1 小时 |
| `s` | 自动滚动开关 |
| `f` | 全屏 / 退出全屏 |
| `w` | 自动换行开关 |
| `t` | 显示 / 隐藏时间戳 |
| `a` | 在单容器 / 所有容器日志间切换 |
| `Shift-C` | 清屏 |
| `/` | 过滤日志，输入关键字或正则后回车，`Esc` 退出 |
| `m` | 标记日志行 |
| `c` | 复制日志 |
| `Ctrl-S` | 保存日志到文件 |
| `Esc` / `q` | 返回 |

日志视图也可以按 `?` 查看当前版本的全部按键。

终端小知识：如果按 `Ctrl-S` 后终端像卡住，按 `Ctrl-Q` 恢复（终端流控导致，不是 K9s 问题）。

## 6. 日常操作配方

### 6.1 排查 CrashLoopBackOff

```text
:po                    # 进入 Pod 视图
/api                   # 按名称过滤（或 / 输入关键字）
d                      # 看事件和状态
l                      # 看当前日志
p                      # 看上一次崩溃日志
e                      # 需要时直接改配置
```

### 6.2 进入容器排查

```text
:po → 选中 Pod → Enter → 选中容器 → s
```

如果容器内没有 shell，可以尝试 `a` Attach，或使用临时调试容器。

### 6.3 临时端口转发

```text
:svc（或 :po）→ 选中目标 → Shift-F → 选择端口 → 输入本地端口
```

按 `f` 可以查看当前已建立的转发。

### 6.4 扩容与滚动重启

```text
:dp → 选中 Deployment → s   # 扩缩容
:dp → 选中 Deployment → r   # 滚动重启
```

### 6.5 节点维护

```text
:no → 选中节点 → c   # Cordon，停止调度新 Pod
:no → 选中节点 → r   # Drain，驱逐现有 Pod
# 维护完成后
:no → 选中节点 → u   # Uncordon，恢复调度
```

### 6.6 手动触发 CronJob

```text
:cj → 选中 CronJob → t
```

### 6.7 查看 Secret 明文

```text
:secret → 选中 Secret → x
```

Secret 属于敏感信息，只在自己有权限的环境查看，避免截图或外传。

### 6.8 查看资源拓扑

```text
:xray dp               # Deployment → ReplicaSet → Pod
:xray svc              # Service 关联的 Pod
:xray dp default       # 指定命名空间
```

### 6.9 快速定位 Pod 所在节点

```text
:po → 选中 Pod → o
```

### 6.10 生产环境只读巡检

```bash
k9s --readonly
```

或者在 context 配置中单独把生产 context 设为只读，避免误删误改。

## 7. kubectl 与 K9s 对照

| 目的 | kubectl | K9s |
| --- | --- | --- |
| 查看 Pod | `kubectl get pods -A` | `:po` → `0` |
| 查看详情 | `kubectl describe pod xxx` | 选中后 `d` |
| 查看 YAML | `kubectl get pod xxx -o yaml` | 选中后 `y` |
| 编辑资源 | `kubectl edit deploy xxx` | 选中后 `e` |
| 查看日志 | `kubectl logs -f xxx` | 选中后 `l` |
| 查看上次日志 | `kubectl logs -p xxx` | 选中后 `p` |
| 进入容器 | `kubectl exec -it xxx -- sh` | 选中后 `s` |
| Attach 容器 | `kubectl attach -it xxx` | 选中后 `a` |
| 端口转发 | `kubectl port-forward svc/xxx 8080:80` | 选中后 `Shift-F` |
| 删除资源 | `kubectl delete pod xxx` | 选中后 `Ctrl-D` |
| 强制删除 Pod | `kubectl delete pod xxx --now` | Pod 视图按 `Ctrl-K` |
| 扩容 | `kubectl scale deploy xxx --replicas=3` | Deployment 视图按 `s` |
| 滚动重启 | `kubectl rollout restart deploy xxx` | Deployment 视图按 `r` |
| 排空节点 | `kubectl drain node-1` | Node 视图按 `r` |
| 禁止调度 | `kubectl cordon node-1` | Node 视图按 `c` |
| 恢复调度 | `kubectl uncordon node-1` | Node 视图按 `u` |
| 手动触发 CronJob | `kubectl create job --from=cronjob/xxx <job-name>` | CronJob 视图按 `t` |

## 8. 配置与自定义

### 8.1 配置目录

| 系统 | 默认目录 |
| --- | --- |
| Windows | `%LOCALAPPDATA%\k9s` |
| Linux | `~/.config/k9s` |
| macOS | `~/Library/Application Support/k9s` |

以 `k9s info` 输出为准；也可以通过环境变量 `K9S_CONFIG_DIR` 指定配置目录。

主要文件：

| 文件 | 作用 |
| --- | --- |
| `config.yaml` | 主配置：刷新间隔、只读、日志、UI 等 |
| `aliases.yaml` | 自定义资源别名 |
| `hotkeys.yaml` | 自定义快捷键 |
| `plugins.yaml` | 自定义插件命令 |
| `views.yaml` | 自定义资源列 / 视图 |
| `clusters/<cluster>/<context>/config.yaml` | 单个 context 的配置，比如只读、皮肤、默认 namespace |

`hotkeys.yaml` 位于 K9s 数据目录，其余文件默认位于配置目录；可以用 `k9s info` 确认实际路径。

### 8.2 config.yaml 常用配置

```yaml
k9s:
  readOnly: false          # true = 禁用编辑/删除
  refreshRate: 2           # 刷新间隔（秒）
  ui:
    enableMouse: true      # 开启鼠标
    headless: false        # 隐藏顶部 Header
    noIcons: false         # 终端图标显示异常时设为 true
  logger:
    tail: 100              # 进入日志视图默认拉取行数
    buffer: 5000           # 日志缓冲区行数
```

### 8.3 单独把某个 context 设为只读

编辑 K9s 数据目录下的 `clusters/<cluster>/<context>/config.yaml`：

```yaml
k9s:
  readOnly: true
```

这样即使平时用可写模式启动，进入生产 context 后也不能编辑或删除。

### 8.4 自定义资源别名

编辑 `aliases.yaml`：

```yaml
aliases:
  pp: v1/pods
  dep: apps/v1/deployments
  mypods: pod kube-system app=web
```

之后可以直接输入 `:pp`、`:dep`、`:mypods`。

### 8.5 自定义快捷键

编辑 `hotkeys.yaml`（放在 K9s 数据目录，Windows 通常同为 `%LOCALAPPDATA%\k9s`）：

```yaml
hotKeys:
  shift-0:
    shortCut: Shift-0
    description: 查看 web Pod
    command: pods app=web
  shift-1:
    shortCut: Shift-1
    description: 查看 Deployment
    command: dp
```

保存后会自动重新加载，也可以在 `?` 帮助里看到自定义快捷键。

### 8.6 插件示例

编辑 `plugins.yaml`：

```yaml
plugins:
  pod-logs:
    shortCut: Shift-B
    description: 查看 Pod 日志
    scopes:
      - po
    command: kubectl
    background: false
    args:
      - logs
      - -f
      - $NAME
      - -n
      - $NAMESPACE
```

常用环境变量：`$NAME`、`$NAMESPACE`、`$CONTEXT`、`$CLUSTER`、`$CONTAINER`、`$FILTER`。

## 9. 注意事项

1. **`Ctrl-D` 删除会弹确认框**，`Ctrl-K` 强制 Kill 不会二次确认，生产环境谨慎使用。
2. **只读模式**：`k9s --readonly`，或在 context 配置里设置 `readOnly: true`，是防误操作最有效的方式。
3. **`Ctrl-Z` 不是挂起进程**：在 K9s 里它用于切换故障提示。
4. **`Ctrl-S` 可能触发终端流控**：如果界面卡住，按 `Ctrl-Q` 恢复。
5. **`@context` 会切换当前 context**：`:pod @prod` 这类命令执行后，后续操作都在该 context 下，注意看顶部提示。
6. **按键受权限影响**：没有 edit/delete 权限时，`e`、`Ctrl-D` 不会出现；只读模式下修改类按键也会被禁用。
7. **版本差异**：不同 K9s 版本的按键可能新增或调整，以当前视图 `?` 帮助为准。
8. **Windows 建议使用 Windows Terminal**，传统 cmd 窗口对全屏 TUI 和颜色支持较差。
