# Handoff: 为 Reasonix 加入 Understand-Anything

**时间**：2026-07-09
**上下文压缩触发**：80% 阈值
**下一会话焦点**：评估并设计 Understand-Anything 集成方案

---

## 1. 本会话完成的工作

### Headroom 功能修复与增强（大量）
- **无效配置根除**：删除 CompressToolResults、ProtectErrors、MinTokens、CCR 四个无效字段，覆盖 config、View、Save、launcher、前端 UI、locale 全部层面
- **CCR 逻辑修正**：从无效 `HEADROOM_CCR_ENABLED` 翻转为 `HEADROOM_NO_CCR`（因安装旧 headroom 不兼容，后又全部删除）
- **Timeout 修复**：`HEADROOM_REQUEST_TIMEOUT` 从未传递给代理 → 修复 env var + launcher CLI arg，coding 预设从 120s 改为 300s
- **KeepAlive**：退出时保持代理运行 + 重启自动复用
- **Settings 持久化**：10+ headroom 配置字段 UI 改为惰性初始化（从 SettingsView 读取）
- **Status 显示**：Session 数据回退 lifetime、移除 warming、删除所有 cost/金额显示
- **Kompress 支持**：GPU 后端配置 + launcher 传递
- **代码审计**：下载 headroom 0.30.0 源码，对全部 env var 和 CLI flag 做了交叉比对

### 全局计划模式 + Guidance Prompt
- **PlanModeDefault** 新增前后端全部链路：config → boot.go → SettingsView → 前端 UI toggle → SetPlanModeDefault Wails binding → createTabEntryWithID 初始化 → SetCollaborationModeForTab 立即生效
- **GuidancePrompt**：完整的 per-session 编辑系统，通过 `<guidance>` 块注入每条消息前，持久化到 BranchMeta `.meta` 文件
- **Shift+Ctrl+P** 快捷键 + intent menu 编辑入口
- **Resume/SetSessionPath** 双路径覆盖 BranchMeta 恢复
- **bridge.ts Proxy** 修复：缺失方法不再返回 undefined 崩溃

### 其他修复
- **YOLO 下 remember 工具**：不再强制 Ask approval
- **plan mode 持久化**：执行完成后 `SetPlanMode(true)` 保持模式激活
- **修改计划面板焦点抢夺**：删除 `inputRef.current?.focus()`
- **Skill 系统 frontmatter**：00-INDEX.md 和 CONTRIBUTING-SKILLS.md 补 description

---

## 2. Understand-Anything 评估结果

已完成对该仓库的初步分析：

- **核心能力**：将代码库转化为 JSON 知识图谱 + React Dashboard
- **Token 影响**：净利好——一次性生成（~15-30K tokens）替换每次会话的重复搜索（5-15K tokens/次）
- **与 Headroom 协同**：正面——图谱走缓存（稳定块），对话走压缩，管道互补
- **接入路径**：作为 Reasonix builtin tool 实现

详细报告已写入 `OtherPackage/understand-anything-access-report.md`（未成功写入磁盘，内容在会话中）。

---

## 3. 下一会话的关键决策项

| # | 决策 | 状态 |
|---|------|------|
| 1 | Understand-Anything 是否接入 | 评估中 |
| 2 | 作为 builtin tool 还是独立 CLI | 待定 |
| 3 | 知识图谱与现有记忆系统的交互模型 | 待设计 |
| 4 | Dashboard 嵌入 Desktop 还是独立 serve | 待定 |

---

## 4. 建议调用的技能

- `/grill-with-docs` — 对 Understand-Anything 接入方案做对齐和设计决策
- `/to-spec` — 如果确认接入，写出 Spec
- `/to-tickets` — 拆解为可执行的工单

---

## 5. 关键代码引用

| 文件 | 本轮改动类型 |
|------|-------------|
| `internal/config/config.go` | 大量删除无效字段 + RequestTimeout 修正 |
| `desktop/headroom_sidecar.go` | env var + launcher 重构 + CCR 修正 + timeout 传递 |
| `internal/control/controller.go` | PlanModeDefault + GuidancePrompt + YOLO memory |
| `internal/control/turn_orchestrator.go` | plan mode 持久化 |
| `internal/control/input.go` | guidance block 注入 |
| `desktop/frontend/src/components/SettingsPanel.tsx` | 大量 UI 状态初始化修复 |
| `desktop/frontend/src/components/Composer.tsx` | guidance prompt editor + ready 依赖 + 去 disabled |
| `desktop/frontend/src/components/StatusBar.tsx` | 移除 cost 显示 |
| `desktop/frontend/src/components/ApprovalModal.tsx` | 删除焦点抢夺 |
| `desktop/frontend/src/lib/bridge.ts` | Proxy handler 修复 |
| `internal/agent/branch.go` | GuidancePrompt 持久化 |
