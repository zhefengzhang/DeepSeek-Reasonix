# 主流程：idea → ship（从想法到交付）

> 这是仓库中最核心的路径。大部分功能开发都走这条路。

---

## 全流程图

```
                    ┌──────────────────┐
                    │   /setup-matt-   │  ← 先跑一次配置：issue tracker + 标签 + 文档结构
                    │   pocock-skills  │
                    └────────┬─────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                    Stage 1: 磨刀                             │
│                                                             │
│  /grill-with-docs (有代码库)   或   /grill-me (无代码库)       │
│       │                                    │                 │
│       ├─ 构建共享语言 → CONTEXT.md         │                 │
│       ├─ 记录架构决策 → ADRs               │                 │
│       └─ 一个 session 内完成，不清上下文   │                 │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        ▼
            ┌─────────────────────┐
            │  需要原型验证吗？      │
            │  (逻辑/UI不确定)     │
            └──────┬──────┬───────┘
                  YES    NO
                   │     │
                   ▼     │
          /handoff →      │
          /prototype →    │
          /handoff ←      │
                   │     │
                   ▼     ▼
┌─────────────────────────────────────────────────────────────┐
│                    Stage 2: 规划                             │
│                                                             │
│  /to-spec ──→ 把讨论合成 PRD (不重复提问)                     │
│     │                                                       │
│     └── 确认测试 seam (接口边界)                             │
│          │                                                   │
│          ▼                                                   │
│  /to-tickets ──→ 拆成 tracer-bullet 工单                     │
│     ├─ 每个工单是垂直切片 (穿过多层，不是水平切一层)            │
│     ├─ 每个工单声明阻塞边 (blocking edges)                    │
│     └─ 发布到 issue tracker                                  │
│                                                             │
│  关键规则：Stage 1-3 在一个不中断的上下文窗口中完成             │
└───────────────────────┬─────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│                    Stage 3: 执行                             │
│                                                             │
│  /implement ──→ 从队列 frontier 取一个工单                    │
│     │                                                       │
│     ├── /tdd ──→ red → green → refactor（每个垂直切片）      │
│     │    ├─ 先写失败测试                                     │
│     │    ├─ 写最少代码让它通过                                │
│     │    ├─ 重构 (在 review 阶段做，不在 TDD 循环内)          │
│     │    └─ 每个 seam 测试，只测公共接口                       │
│     │                                                        │
│     └── /code-review ──→ 两轴审查                            │
│          ├─ 轴1: Standards（编码标准 + 设计气味）              │
│          └─ 轴2: Spec（是否忠实实现需求）                     │
│               │                                               │
│               ▼                                               │
│          git commit ──→ 当前分支                              │
│                                                               │
│  关键规则：每个 /implement 在新上下文窗口中开始                 │
└─────────────────────────────────────────────────────────────┘
```

---

## 每个阶段的职责和约束

### Stage 0：配置 `/setup-matt-pocock-skills`

**解决的问题**：后续工程技能依赖一个共同的"上下文"——issue tracker 在哪、标签是什么、领域文档放哪。不配好就跑不了。

**三个配置项**（逐个问用户，一次只问一个）：
1. **Issue Tracker** — GitHub Issues / GitLab / Local markdown / Other
2. **Triage Labels** — 五个标准化角色映射到实际标签字符串
3. **Domain Docs** — 单上下文还是多上下文（monorepo用 CONTEXT-MAP.md）

**产物**：`docs/agents/issue-tracker.md` + `triage-labels.md` + `domain.md`

---

### Stage 1：磨刀 — `/grill-with-docs` 或 `/grill-me`

**解决的问题**：你不知道你到底想要什么。Agent 也不知道。双方在对话中逐步对齐。

**约束**：
- 一次只问一个问题（多个问题一起抛会让人困惑）
- 能通过查代码库找到的事实不要问用户，只问**决策**（decisions are mine）
- 直到用户确认"我们达成共识了"才能开始执行

**`grill-with-docs` vs `grill-me` 的区别**：

| | grill-with-docs | grill-me |
|--|----------------|----------|
| 适用场景 | 有代码库 | 无代码库/非代码场景 |
| 副作用 | 写入 CONTEXT.md + ADRs | 什么都不保存 |
| 内部机制 | 调用 /grilling + /domain-modeling | 调用 /grilling |

**`/domain-modeling` 的具体工作**：
- 当用户说了一个与现有词汇冲突的词时，立即指出
- 当用户用模糊词时，提出精确的规范术语
- 用具体场景压力测试领域关系
- 与代码交叉验证
- 立刻更新 CONTEXT.md（不积累）
- 只有当三个条件都满足时才创建 ADR：
  1. 反转成本高
  2. 未来读者会疑惑
  3. 经过了真实权衡

---

### Stage 2：规划 — `/to-spec` → `/to-tickets`

**`/to-spec` 解决**：把已经讨论清楚的东西写成正式文档。**不重新提问**（这条是这个技能和其他"帮你写文档"工具的核心区别：不采访你，只综合已知信息）。

- 问题陈述 → 解决方案 → 用户故事 → 实现决策 → 测试决策 → 超范围 → 补充说明
- 其中实现决策**不包含具体文件路径或代码片段**（会过时）
- 原型产出的关键代码片段（状态机、reducer、schema）可以内联

**`/to-tickets` 解决**：把计划拆成可执行的工单。

**核心概念：tracer-bullet（曳光弹）工单**
- 每个工单是**垂直切片**：贯穿 schema → API → UI → 测试，不是水平切一层
- 一个完成的工单是可演示/可验证的
- 每个工单的大小 = 一个干净的上下文窗口能装下
- 每个工单声明**阻塞边**（blocking edges）

**例外：wide refactor**
当一次机械变更（如重命名列、修改共享类型）的波及范围横跨整个代码库时，不要强行塞进 tracer bullet。改用 **expand–contract**：
1. Expand：新旧并存，不破坏任何东西
2. Migrate：按波及范围分批迁移
3. Contract：无人使用后删除旧的

---

### Stage 3：执行 — `/implement` → `/tdd` → `/code-review`

**`/implement` 解决**：从工单队列中取一个能开始的（所有依赖已完成），用 TDD 实现，最后审查。

**`/tdd` 解决**：给 TDD 流程一个结构化参考。

关键约束：
- **先确认 seam**（测试接口边界）：在写任何测试之前，先写下要在哪些 seam 测试，等待用户确认
- **只测 seam**：测公共接口，不测内部实现
- **Red before green**：先写失败测试，再写刚好让它通过的代码
- **One slice at a time**：一个 seam，一个测试，一个最小实现
- **重构不在 TDD 循环内**：属于 review 阶段
- **垂直切片**：不要水平切（不要先写所有测试再写所有实现）
- **Expected values 必须来自独立的事实源**：已知正确的字面量、手算的结果、规格说明

**`/code-review` 解决**：两个并行子 agent 分别审查标准和规格。两轴审查：
1. **Standards 轴**：是否遵循编码标准 + Fowler 设计气味检查
2. **Spec 轴**：是否忠实实现了工单/PRD

---

## 分支路径

### 当需要原型验证时

在 Stage 1 和 Stage 2 之间有一个分支：

```
/grill-with-docs → /handoff（压缩上下文）→ [新 session]
    → /prototype（扔掉的代码，回答一个问题）
    → /handoff（带回学到的知识）→ 回到主流程的 /to-spec
```

**`/prototype` 解决**：通过写扔掉的代码来回答一个设计问题。

两种分支：
1. **LOGIC**（逻辑/状态模型）：构建一个交互式终端应用来测试状态机
2. **UI**（界面）：生成多个完全不同风格的 UI 变体，通过 URL 参数切换

铁律：
- 从第一天起就是扔掉的（命名为 PROTOYPE，让人一看就知道不是生产代码）
- 一个命令就能跑
- 默认不持久化
- 跳过所有 polish（没测试、没错误处理、没抽象）
- 每次操作后输出完整状态
- 回答完问题后要么删掉，要么把验证过的决策吸收进正式代码

### 当问题已经堆积——triage 入径

```
/triage → 状态机：
  unlabeled → needs-triage → needs-info / ready-for-agent / ready-for-human / wontfix
```

**`/triage` 解决**：把 issue 队列系统化地处理成 agent-ready 状态。

这不是普通标签管理，是一套完整的状态机：
- 两个**分类角色**：bug / enhancement
- 五个**状态角色**：needs-triage → needs-info → ready-for-agent → ready-for-human → wontfix
- 每个 issue 必须同时有一个分类角色 + 一个状态角色
- 验证 claim（对 bug 先按步骤复现）
- 需要补充信息时调用 /grilling + /domain-modeling
- 每次 triage 评论必须以 `> *This was generated by AI during triage.*` 开头

### 当 bug 出现了——diagnosing-bugs 入径

**`/diagnosing-bugs` 解决**：对顽固 bug 提供一个规范的诊断循环。

这个技能是 model-invoked 的，意味着 agent 在遇到 bug 时可以自主触发它。流程包括：复现 → 最小化 → 假设 → 仪表化 → 修复 → 回归测试。

---
