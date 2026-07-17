# Context Inventory

为 AI Agent 提供已读文档清单，减少不必要的 `read_file` 重读，降低 token 浪费。

## 问题

AI Agent 在处理文档密集型任务（小说创作、技术写作、调研报告等）时，倾向于直接调用 `read_file` 重读文档，而不先判断该文档是否已在对话上下文窗口中。这导致：

- **Token 浪费**：每次不必要重读消耗输入 token（文档内容回传）+ 输出 token（工具调用参数）。5000 行章节文件单次重读约耗费 15,000–25,000 tokens。
- **缓存命中率低**：重读内容占用上下文窗口，加速 compaction 触发。
- **用户体验下降**：等待不必要的工具调用完成，响应变慢。

### 根因

模型在调用 `read_file` 之前，没有任何机制告诉它"这个文件已经在你的上下文里了"。模型要自己扫描 conversation history 来做这个判断，成本很高，所以默认选择了"重读"。

## 设计决策

### 方案对比

| 方案 | 每轮开销 | 准确度 | 侵入面 | 维护成本 |
|---|---|---|---|---|
| A. 全量扫描消息历史 | O(n)，n 随对话增长 | ✅ | 低 | 低 |
| B. 事件驱动（Sink 拦截） | O(1) | ✅ | 高 — `boot.go` + `controller.go` + Sink 包装 | 中 — 需处理 Dispatch/Result 关联、并发安全、session resume 重建 |
| **C. 增量消息扫描（选用）** | O(新增消息)，≈ O(8) | ✅ | **最低** — 仅 `internal/control/` | **最低** |

### 选择方案 C 的理由

1. **效率等价于事件驱动**：正常对话每轮新增 ~8 条消息，增量扫描几乎无开销。
2. **健壮性优于事件驱动**：自动检测 compact / session reset（`len(msgs) < scanUpTo` → 全量重建），无需额外重建逻辑。
3. **侵入面远小于事件驱动**：事件驱动需要包装 Sink、处理 Dispatch/Result 关联、解决并发安全、在 session resume 时重建状态。增量扫描不触碰 Sink、事件系统、Agent 内部。
4. **不触碰 cache-sensitive 路径**：方案 B 需要在 `boot.go` 中包装 Sink（`internal/boot/*` 在 `check-cache-impact.sh` 中标记为 cache-sensitive）。方案 C 所有改动在 `internal/control/` 内（非 cache-sensitive）。

## 架构

### 组件关系

```
Controller.readInv (readFileTracker)
  │
  ├─ scanDelta(msgs)  ← 在 Compose() 开头调用
  │   └─ 只扫描 msgs[scanUpTo:]
  │       如果 len(msgs) < scanUpTo → compact，全量重建
  │
  ├─ build() → "<context-inventory>…</context-inventory>"
  │   └─ 在 Compose() 结尾注入到 turn tail
  │
  └─ files map[string]*fileState
       ├─ path: 文件路径
       ├─ readOffset / readLimit: 最后读取范围
       └─ stale: 读取后是否被编辑
```

### State Machine

每个文件在 `files` map 中的状态转换：

```
                    read_file 成功
    [不存在] ────────────────────→ [tracked: stale=false]
                                        │
                    edit_file /         │ read_file 成功
                    write_file /        │ (resets stale)
                    multi_edit /        │
                    move_file /         │
                    delete_range /      │
                    delete_symbol /     │
                    notebook_edit       │
                                        ↓
                                  [tracked: stale=true]
```

### 追踪的工具

| 工具 | 作用 | 提取的路径字段 |
|---|---|---|
| `read_file` | 记录读取范围，重置 stale | `path` |
| `write_file` | 标记 stale | `path` |
| `edit_file` | 标记 stale | `path` |
| `multi_edit` | 标记 stale | `path` |
| `delete_range` | 标记 stale | `path` |
| `delete_symbol` | 标记 stale | `path` |
| `notebook_edit` | 标记 stale | `path` |
| `move_file` | 标记 source + destination 路径 stale | `source_path`, `destination_path` |

### 注入位置

`<context-inventory>` 块注入在 Compose 的 turn tail 中，**不触碰 cache-stable prefix**：

```
[goal block]              ← 如果有 active goal
[plan mode marker]         ← 如果在 plan mode
[guidance]                 ← 如果有 guidance prompt
[response/reasoning lang]  ← 语言偏好
[memory updates]           ← 内存更新（已有）
[background jobs]          ← 后台任务完成通知（已有）
[context inventory]        ← 新增：读文件清单
[user's actual text]       ← 用户的实际输入
```

注入的块格式（示例）：

```xml
<context-inventory>
Already in context — do NOT re-read unless marked stale:
  chapters/ch1.md (L1-L2000)
  notes/characters.md (L1-L800) ⚠️ stale — was edited after read
</context-inventory>
```

块自身携带指令（self-describing），即使项目没有 `REASONIX.md` 也能生效。

### 系统 Prompt 策略

在 `internal/config/config.go` 中新增 `ContextInventoryPolicy` 常量，拼接到系统 prompt 中（和 `UserDecisionPolicy`、`LanguagePolicy` 同级）：

```
Context re-read rule: when a <context-inventory> block appears in the turn,
files listed without ⚠️ are already in context and unchanged — do not re-read
them. Files marked ⚠️ stale were edited after read and may need refreshing.
Files not listed are not in context.
```

~35 tokens，一次前缀缓存失效后永久稳定。

## 文件清单

| 文件 | 改动 | 说明 |
|---|---|---|
| `internal/control/inventory.go` | **新增** | `readFileTracker` 结构体、`scanDelta`、`scanMessages`、`apply`、`build`、3 个参数解析器 |
| `internal/control/inventory_test.go` | **新增** | 11 个测试用例覆盖全部状态组合 |
| `internal/control/controller.go` | +1 行 | Controller struct 新增 `readInv readFileTracker` 字段 |
| `internal/control/input.go` | +7 行 | Compose 开头调用 `scanDelta`，结尾注入 inventory |
| `internal/config/config.go` | +6 行 | 新增 `ContextInventoryPolicy` 常量 |
| `internal/boot/boot.go` | +1 行 | 拼接 `ContextInventoryPolicy` 到 sysPrompt |

## 缓存安全

### 为什么 turn tail 不影响 prefix cache

DeepSeek 的自动 prefix caching 缓存请求的**最长公共前缀**：

```
[system prompt] [user₁] [assistant₁] [user₂] [assistant₂] ... [userₙ]
                                                    ↑ 缓存到这里
                                                    [new userₙ₊₁]
                                                          ↑ 新的，不缓存
```

`<context-inventory>` 注入在新的 user message 中，而新 user message 不在 prefix cache 覆盖范围内。系统 prompt + 历史消息的缓存不受影响。

### check-cache-impact.sh 分析

- `internal/control/*` **不在** cache-sensitive 列表中 → 改动 1–3 不影响缓存
- `internal/config/config.go` **在** cache-sensitive 列表中 → 新增常量导致一次性 prefix cache 失效，之后永久稳定
- `internal/boot/boot.go` **在** cache-sensitive 列表中 → 新增策略拼接行随常量一起稳定

### 验证方法

```bash
# 构建两次，比对系统 prompt
go test ./internal/boot/ -run TestBuildWithoutMemoryLeavesPromptUnchanged
# ✅ PASS — 系统 prompt 在无变化配置下保持字节稳定
```

## 后续计划

1. **Compact 集成**：当对话经历 compaction 后，之前读取的文件内容可能已不在上下文中。需要在 compact 事件触发时标记所有条目为"可能已过期"。
2. **子 Agent 追踪**：`task` / `read_only_task` 子 agent 的文件读取目前在父 agent 的 `readFileTracker` 中不可见。可以考虑将子 agent 的读取结果汇总到父 tracker 中。
3. **文件内容变更检测**：结合文件系统 mtime，自动检测文件是否在推理器外部被修改。

## 测试覆盖

`internal/control/inventory_test.go` 包含 11 个测试用例：

| 测试 | 覆盖场景 |
|---|---|
| `TestReadFileTrackerEmptyHistory` | 空历史 → 空清单 |
| `TestReadFileTrackerSingleRead` | 单次 read_file → 一条记录，无 stale 标记 |
| `TestReadFileTrackerReadThenEdit` | read → edit_file → stale |
| `TestReadFileTrackerReadThenMultiEdit` | read → multi_edit → stale |
| `TestReadFileTrackerReadThenWrite` | read → write_file → stale |
| `TestReadFileTrackerEditThenReReadResetsStale` | read → edit → read → 重置 stale |
| `TestReadFileTrackerMoveFileMarksBothStale` | read a, read b → move a→b → both stale |
| `TestReadFileTrackerIncrementalScan` | 分批扫描，验证增量不丢数据 |
| `TestReadFileTrackerCompactRebuilds` | compact 后消息缩小 → 自动全量重建 |
| `TestReadFileTrackerOnlyReadsListed` | write-only 不出现在清单中 |
| `TestReadFileTrackerMultipleReadSameFile` | 同文件多次读取 → 最后一次的 range 胜出 |
