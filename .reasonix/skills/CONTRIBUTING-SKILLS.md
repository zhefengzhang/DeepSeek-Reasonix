---
name: CONTRIBUTING-SKILLS
description: Writing conventions for the project skill store — frontmatter format, naming, structure. Read before authoring a new skill.
disable-model-invocation: true
---
# Reasonix 技能编写约定

> 本文档定义本项目 `.reasonix/skills/` 下技能文件的编写规范。
> 灵感来自 Matt Pocock 的 `writing-great-skills` 设计哲学。

---

## 一个技能长什么样

每个技能是一个 `<name>/SKILL.md` 文件，由 YAML frontmatter + Markdown body 组成。

### Frontmatter 字段

```yaml
---
name: my-skill           # 必填。技能标识符，也是 /name 的调用名
description: "..."       # 必填。user-invoked: 给人类看的一行摘要
                         # model-invoked: 给 agent 看的触发词 + 场景描述
disable-model-invocation: true  # 省略 = model-invoked（agent 可自动触发）
                                # true = user-invoked（只能 /name 调用）
run-as: subagent         # 可选。subagent = 隔离子循环执行；省略 = inline
allowed-tools:           # 可选。subagent 模式下限制可用工具
  - read_file
  - grep
model: deepseek-reasoner # 可选。subagent 模式下指定模型
---
```

### Body 结构

```
# 技能名（H1）

一句话说清楚这个技能解决什么问题。

## 步骤（Steps）—— 如果有的话

1. 第一步
2. 第二步
   - 完成标准：可检查的条件

## 参考（Reference）—— 如果适用

定义、规则、事实，agent 在执行过程中查阅。
```

---

## 四条核心原则

### 1. 信息层级决定行为可靠性

按 agent 需要的紧迫程度排列内容：

```
1. 步骤（Steps）—— 必须按顺序执行的，放在最前面
2. 内嵌参考（In-skill Reference）—— 执行过程中查阅的定义和规则
3. 外置参考（Disclosed Reference）—— 通过上下文指针引用的外部文件
```

把不需要的东西推出核心路径。**需要时才加载的材料放到单独的 `.md` 文件中**，agent 只会在需要时读取。

### 2. 完成标准（Completion Criterion）必须可检查

每个步骤后面写清楚「什么时候算做完」。不好的例子：

```
❌ 分析代码库
✅ 分析代码库，直到你能回答：哪些模块可以变得更深？
```

检查标准：
- **可检查**：agent 能确切判断做完了还是没做完
- **穷尽**：不是"列出改动"，是"每个被改的文件都确认过了"

### 3. Leading Word——用模型已知的概念压缩指令

如果有一组意思需要反复表达，**不要写一段话**，找一个模型 pretraining 中已有的词来锚定行为。

例子：
| 不用 | 用 |
|------|----|
| "彻底检查每一条规则" | **relentless** |
| "测试失败了" | **red**（红灯状态） |
| "端到端的功能切片" | **tracer bullet** |
| "代码扔掉的只回答一个问题" | **throwaway** |

Leading word 节省 token，更重要的是让 agent 行为更一致——每次出现这个词它都执行同样的行为模式。

### 4. No-Op 测试——每一行都要经过

写完每句话后问：**相比默认行为，这句话改变了什么？**

如果删掉这句话 agent 的行为不变——它就是 no-op（噪音），删掉。

常见 no-op：
- `be thorough`（默认 agent 已经 thorough-ish）
- `write clean code`（太抽象，agent 不知道具体要做什么）
- 通篇废话但没有任何具体步骤或规则

---

## 命名和目录约定

```
.reasonix/skills/
├── 00-INDEX.md                    # 本索引文件
├── CONTRIBUTING-SKILLS.md         # 本文——编写约定
├── domain-modeling/
│   ├── SKILL.md                   # 技能主体
│   ├── CONTEXT-FORMAT.md          # 外置参考
│   └── ADR-FORMAT.md              # 外置参考
├── codebase-design/
│   ├── SKILL.md
│   ├── DEEPENING.md
│   └── DESIGN-IT-TWICE.md
├── grill-with-docs/
│   └── SKILL.md
...每技能一个文件夹...
```

- 技能名称：kebab-case，只含 `[a-z0-9.-]`
- 引用其他技能用 `/skill-name` 语法，不要跨文件夹链接
- 共享参考文档放在用它的技能文件夹内，其他技能通过调用技能来使用

---

## 关于 skill 间组合

1. **User-invoked 技能可以调用 model-invoked 技能**（通过 `run_skill` 工具）
2. **User-invoked 技能不能调用另一个 user-invoked 技能**（agent 发现不了它）
3. **Model-invoked 技能可以被其他技能和 agent 自动触发**

这意味着：
- 编排层（workflow）用 user-invoked
- 实现层（discipline）用 model-invoked
- 编排层通过调用实现层来组合功能

---

## 写完后自检清单

- [ ] Frontmatter 的 name 和 description 正确填写
- [ ] 如果是 user-invoked，`disable-model-invocation: true`
- [ ] 每个步骤有可检查的完成标准
- [ ] 每句话都通过了 no-op 测试
- [ ] 如果有 leading word，定义清晰且在全文一致使用
- [ ] 没有 negation 句式（不说"不要做 X"，说"做 Y"）
- [ ] 参考内容该推后披露的已推后到引用文件
