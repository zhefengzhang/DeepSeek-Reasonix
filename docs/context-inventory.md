# Context Inventory

为 AI Agent 提供已读文档清单 + 工具调用硬护栏，减少不必要的 `read_file` 重读和 `grep` 替代 codegraph 的滥用，降低 token 浪费。

## 问题

### read_file 重读

AI Agent 在处理文档密集型任务时，倾向于直接调用 `read_file` 重读文档，而不先判断该文档是否已在对话上下文窗口中。

### grep 替代 codegraph

系统 prompt 和 tool description 中明确写了 `grep` 是 fallback 工具（优先用 `understand_search` / `codegraph_explore`），但 AI 在编码惯性下仍然频繁绕过 codegraph 直接调用 grep。

### 软指令失效

两种情况下，prompt 指令（`<context-inventory>` 和 tool description）都是软指令——AI 在复杂任务中容易脱敏忽略。需要**硬护栏**：工具执行前拦截，人工审批。

## 设计决策

### 方案对比

| 方案 | 每轮开销 | 准确度 | 侵入面 | 维护成本 |
|---|---|---|---|---|
| A. 全量扫描消息历史 | O(n)，n 随对话增长 | ✅ | 低 | 低 |
| B. 事件驱动（Sink 拦截） | O(1) | ✅ | 高 | 中 |
| **C. 增量消息扫描（选用）** | O(新增消息)，≈ O(8) | ✅ | **最低** | **最低** |

### 选择方案 C 的理由

1. **效率等价于事件驱动**：正常对话每轮新增 ~8 条消息，增量扫描几乎无开销。
2. **健壮性优于事件驱动**：自动检测 compact / session reset（`len(msgs) < scanUpTo` → 全量重建）。
3. **不触碰 cache-sensitive 路径**：核心改动在 `internal/control/` 内（非 cache-sensitive，除了一次性的 `config.go` 常量）。

## 架构

### 组件关系

```
Controller.readInv (readFileTracker)
  │
  ├─ scanDelta(msgs)  ← 在 Compose() 开头调用
  │   ├─ 清空 inTurn（本轮追踪复位）
  │   └─ 只扫描 msgs[scanUpTo:]
  │       如果 len(msgs) < scanUpTo → compact，全量重建
  │
  ├─ build() → "<context-inventory>…</context-inventory>"
  │   └─ 在 Compose() 结尾注入到 turn tail
  │
  ├─ files map[string]*fileState      ← 跨轮追踪
  │
  └─ inTurn map[string]*fileState    ← 本轮追踪（防止同一轮内重读漏拦）
```

### State Machine

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

### 同一轮内重读（inTurn 追踪）

`scanDelta` 在 Compose 时清空 `inTurn`。PreCheck 闭包在首个 `read_file` 调用时通过 `trackInTurn` 记录；同一轮内第二次 `read_file` 同路径时，`lookup` 先查 `inTurn` 命中 → 触发审批。

```
Compose() → scanDelta → 清空 inTurn，更新 files
  ↓
Turn N:
  read_file("a.md") #1 → lookup → 未命中 → trackInTurn → 放行
  read_file("a.md") #2 → lookup → inTurn 命中 → ask=true → 审批 ✅
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

### 系统 Prompt 策略

在 `internal/config/config.go` 中新增 `ContextInventoryPolicy` 常量，拼接到系统 prompt 中（和 `UserDecisionPolicy`、`LanguagePolicy` 同级）：

```
Context re-read rule: when a <context-inventory> block appears in the turn,
files listed without ⚠️ are already in context and unchanged — do not re-read
them. Files marked ⚠️ stale were edited after read and may need refreshing.
Files not listed are not in context.
```

~35 tokens，一次前缀缓存失效后永久稳定。

## 硬护栏：PreCheck 审批

### 实现机制

利用 `permission.Gate.PreCheck` 回调，在 `Policy.Decide()` 之前检查。Controller 在 `newInteractiveGate()` 中注入闭包，根据工具名分发：

```go
// internal/control/controller.go — newInteractiveGate()
gate.PreCheck = func(toolName string, args json.RawMessage) (bool, string) {
    switch toolName {
    case "read_file":
        // 解析 path → 查 c.readInv.lookup(path)
        // 命中且未 stale → ask=true, subject="re-read ... (Lx-Ly)"
        // 未命中 → trackInTurn 记录 → ask=false
    case "grep":
        // codegraph 已连接 → ask=true
        // 未连接 → ask=false
    }
}
```

### read_file 审批流程

```
AI: read_file("chapters/ch1.md")
     │
     ▼
PreCheck: path 在 inventory 或 inTurn，未 stale → ask=true
     │
     ▼
Gate.Check() → approve() → gateApprover.ApproveWithReason()
     │
     ▼
Controller.requestApproval() → ApprovalRequest 事件
     │
     ▼
用户弹窗: "read_file: re-read chapters/ch1.md — already in context (L1-L2000)"
     ├─ [Allow Once]     → 执行 read_file
     ├─ [Always Allow]   → 执行 + 记住规则
     └─ [Deny]           → 用户可输入拒绝理由
```

### grep 审批流程

```
AI: grep("pattern", "path")
     │
     ▼
PreCheck: codegraph MCP 已连接 → ask=true
     │
     ▼
用户弹窗: "grep: grep is a fallback — is codegraph_explore or understand_search available instead?"
     ├─ [Allow Once]     → 执行 grep
     ├─ [Always Allow]   → 执行 + 记住规则
     └─ [Deny]           → 用户可输入拒绝理由
```

grep 仅当 codegraph MCP 已连接时才拦截（`c.mcp.registry().Get("mcp__codegraph__codegraph_explore")`）。未连接 codegraph 的用户不受影响。

### 拒绝理由（denyReason）

当用户点击 Deny 时，ApprovalModal 展开一个可选的文本输入框。用户输入理由后确认 Deny，理由通过以下链路传递给 AI：

```
DenyWithReason(id, reason)
  → approvalReply.denyReason
  → gateApprover.ApproveWithReason → reason 返回值
  → Gate.Check: "the user declined: {reason} — do not retry; ask how to proceed"
  → AI 收到拒绝理由，可调整策略
```

无理由时行为不变：`"the user declined this tool call — do not retry it"`。

### 不影响正常使用

- **read_file**：PreCheck 仅在文件已在 inventory/inTurn 且未 stale 时触发。首次读取和 stale 文件重读不受影响。
- **grep**：仅在 codegraph MCP 已连接时触发。未连接或 YOLO 模式不拦截。
- **YOLO 模式**：`bypassAllowsLocked` 自动放行，PreCheck 不弹窗。

## 文件清单

| 文件 | 改动 | 说明 |
|---|---|---|
| `internal/control/inventory.go` | **新增** | `readFileTracker`：`inTurn` 同轮追踪、`lookup`、`trackInTurn`、`scanDelta`、`build` |
| `internal/control/inventory_test.go` | **新增** | 14 个测试用例覆盖全部状态组合 |
| `internal/control/controller.go` | 修改 | `readInv` 字段；PreCheck 闭包（read_file + grep）；`codeGraphConnected()`；`approvalReply.denyReason`；`DenyWithReason()`；`requestApprovalWithReason` 新签名 |
| `internal/control/input.go` | 修改 | Compose 调用 `scanDelta` + 注入 inventory |
| `internal/config/config.go` | 修改 | `ContextInventoryPolicy` 常量 |
| `internal/boot/boot.go` | 修改 | 拼接 `ContextInventoryPolicy` 到 sysPrompt |
| `internal/permission/permission.go` | 修改 | `Gate.PreCheck` 字段；`Check()` PreCheck 集成；拒绝消息格式 |
| `internal/control/port.go` | 修改 | `Approvals` 接口新增 `DenyWithReason` |
| `internal/control/turn_orchestrator.go` | 修改 | `requestApproval` 调用适配新签名 |
| `internal/control/controller_test.go` | 修改 | 适配新 `requestApproval` 签名 |
| `internal/control/yolo_test.go` | 修改 | 同上 |
| `internal/agent/gate_test.go` | 新增 | `TestGatePreCheckSameTurnReRead`、`TestGatePreCheckGrepBlockedWhenCodeGraphConnected` |
| `internal/permission/permission_test.go` | 新增 | `TestGatePreCheckReroutesToApprover` |
| `desktop/app.go` | 修改 | `DenyWithReason()` + `DenyWithReasonTab()` |
| `desktop/frontend/src/lib/bridge.ts` | 修改 | `DenyWithReason` 接口 + mock |
| `desktop/frontend/src/lib/useController.ts` | 修改 | `denyWithReason` 回调 |
| `desktop/frontend/src/components/ApprovalModal.tsx` | 修改 | Deny 按钮 → 拒绝理由输入框 + Confirm/Cancel |
| `desktop/frontend/src/App.tsx` | 修改 | `onAnswer` 接线 `denyWithReason` |
| `desktop/frontend/src/locales/` | 修改 | `approval.denyReasonPlaceholder`、`approval.confirmDeny`（en/zh/zh-TW） |

## 缓存安全

`<context-inventory>` 注入在新的 user message 中（turn tail），不在 prefix cache 覆盖范围内。系统 prompt + 历史消息的缓存不受影响。

- `internal/control/*` **不在** cache-sensitive 列表中 → 核心改动不影响缓存
- `internal/config/config.go` **在** cache-sensitive 列表中 → 新增常量导致一次性 prefix cache 失效，之后永久稳定

## 后续计划

1. **Compact 集成**：compact 后之前读取的文件可能不在上下文中，需标记。
2. **子 Agent 追踪**：子 agent 的文件读取在父 tracker 中不可见。
3. **文件变更检测**：结合 mtime 检测外部修改。

## 测试覆盖

### inventory_test.go（14 个用例）

| 测试 | 覆盖场景 |
|---|---|
| `TestReadFileTrackerEmptyHistory` | 空历史 → 空清单 |
| `TestReadFileTrackerSingleRead` | 单次 read_file → 一条记录 |
| `TestReadFileTrackerReadThenEdit` | read → edit_file → stale |
| `TestReadFileTrackerReadThenMultiEdit` | read → multi_edit → stale |
| `TestReadFileTrackerReadThenWrite` | read → write_file → stale |
| `TestReadFileTrackerEditThenReReadResetsStale` | read → edit → read → 重置 stale |
| `TestReadFileTrackerMoveFileMarksBothStale` | read a, read b → move a→b → both stale |
| `TestReadFileTrackerIncrementalScan` | 分批扫描验证增量 |
| `TestReadFileTrackerCompactRebuilds` | compact 后自动重建 |
| `TestReadFileTrackerOnlyReadsListed` | write-only 不出现 |
| `TestReadFileTrackerMultipleReadSameFile` | 同文件多次读取 → 最后 range |
| `TestReadFileTrackerLookup` | lookup：空、命中、stale 排除、re-read 重置 |
| `TestReadFileTrackerSameTurnReRead` | 同轮重读：inTurn 命中 + scanDelta 清除 |
| `TestPreCheckClosureWithTracker` | PreCheck 闭包 + 真实 readFileTracker |

### gate_test.go（3 个新增用例）

| 测试 | 覆盖场景 |
|---|---|
| `TestGatePreCheckSameTurnReRead` | Agent → executeOne → Gate → PreCheck 完整链路 |
| `TestGatePreCheckGrepBlockedWhenCodeGraphConnected` | grep 拦截 approve/deny 两条路径 |
| `TestGatePreCheckReroutesToApprover` | PreCheck 命中/未命中/拒绝/记住 四条审批路径 |
