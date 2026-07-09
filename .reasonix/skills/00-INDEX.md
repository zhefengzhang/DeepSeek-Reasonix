---
name: 00-INDEX
description: Skill index mapping every engine skill, its trigger, and its primary deliverable. Scan before unfamiliar work to pick the right playbook, not for execution.
disable-model-invocation: true
---
# Reasonix 工程技能索引

> 一套面向 Go 程序开发的 AI Agent 技能，基于 Matt Pocock 的 skills 设计哲学。
> 安装位置：`.reasonix/skills/<name>/SKILL.md`
> 调用方式：`/<name>`（user-invoked）或 agent 自动触发（model-invoked）

---

## 核心工作流（idea → ship）

```
/grill-with-docs     →  /to-spec    →   /to-tickets    →   /implement
(对齐想法+建共享语言)    (合成Spec)      (拆成工单)          (TDD构建+审查)
```

---

## 词汇层（model-invoked — Agent 可自主调用）

| 技能 | 触发词 | 作用 |
|------|--------|------|
| `domain-modeling` | 遇到模糊术语、需要更新 CONTEXT.md 时 | 维护项目共享语言 |
| `codebase-design` | 设计/讨论模块接口时 | 深模块设计词汇 |

## 对齐（user-invoked — 需要你主动输入 `/name`）

| 技能 | 什么时候用 | 主要产出 |
|------|-----------|---------|
| `grill-with-docs` | 每次做改动之前 | CONTEXT.md + ADRs + 对齐确认 |
| `grill-me` | 非代码场景或快速对齐 | 仅对话，不写文档 |

## 规划（user-invoked）

| 技能 | 什么时候用 | 主要产出 |
|------|-----------|---------|
| `to-spec` | 讨论清楚后 | PRD/Spec 文档 |
| `to-tickets` | 有 spec 后 | `.scratch/<feature>/tickets.md` |

## 执行（mixed）

| 技能 | 调用方式 | 什么时候用 |
|------|---------|-----------|
| `implement` | user-invoked | 从工单队列取一个实现 |
| `tdd` | model-invoked | 每个垂直切片：red→green→refactor |
| `code-review` | model-invoked | 提交前审查 diff |
| `diagnosing-bugs` | model-invoked | 遇到顽固 bug 时 |

## 运维（mixed）

| 技能 | 调用方式 | 什么时候用 |
|------|---------|-----------|
| `improve-codebase-architecture` | user-invoked | 每几天跑一次预防代码熵增 |
| `prototype` | model-invoked | 不确定设计方案时写扔掉的代码验证 |
| `handoff` | user-invoked | 当前上下文快满了需要跨 session |
| `setup-reasonix-skills` | user-invoked | 首次配置项目时（创建目录、模板） |

---

## 快速参考

- **不确定用什么技能？** → 先跑 `/grill-with-docs`，它会在过程中驱动其他技能
- **技能太多记不住？** → 核心记住两个：`/grill-with-docs`（开始前）和 `/implement`（开始后）
- **所有技能都是 Markdown，可以随时读源文件改内容**
