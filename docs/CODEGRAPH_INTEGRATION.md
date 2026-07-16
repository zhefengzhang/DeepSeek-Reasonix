# CodeGraph 集成工作报告

> 日期：2026-07-16

将 CodeGraph 集成进 Reasonix 项目，与 Understand-Anything 知识图谱配合，形成「结构导航 + 源码查询」双图谱协作工作流。

---

## 1. 背景：为什么需要 CodeGraph

### 现有的 Understand-Anything 能做什么

- 把项目文件、模块、类、函数扫描成知识图谱，存入 `.understand-anything/knowledge-graph.json`
- 提供 `understand_search` 工具，按结构查询（模块在哪、文件属于哪个层级、谁调用了谁）
- 输出元数据摘要（节点名称、类型、关系），不带实际源码

### Understand-Anything 的局限

当任务需要精确的源码细节——比如「CopyButton 在 TurnActions 的 JSX 中具体在第几行」「这个函数的完整参数列表是什么」——`understand_search` 只能说「TurnActions 定义在 Message.tsx，是 function」，然后 Agent 必须用 grep 搜行号、read_file 打开文件、再人工匹配上下文。

这种 grep + read 往返在本次收藏功能的开发中反复发生：单次 Transcript.tsx 的 workspaceRoot prop drilling 修改消耗了 15+ 轮工具调用。

### CodeGraph 补上那一块

CodeGraph 用 tree-sitter 解析源码，存入 SQLite 数据库。它的 `codegraph_explore` 工具一次调用返回 verbatim 逐行源码（与 Read 工具同比特等价），同时附带调用链和 blast radius。一次查询替代了之前的多次 grep + read_file 往返。

---

## 2. 双图谱分工

| 场景 | 工具 | 返回内容 |
|------|------|---------|
| 项目骨架、模块发现、架构导航 | `understand_search` | 节点名称、类型、关系摘要 |
| 源码细节（函数体、参数、JSX 结构） | `codegraph_explore` | 带行号的完整源码 + 调用链 + 影响面 |
| 跨文件调用链 + 动态分派 | `codegraph_explore` | React re-render / JSX child / callback 自动桥接 |
| 精确文本搜索 | `grep` | 仅当两个图谱工具都不可用时降级 |

### 标准工作流

```
收到任务
  │
  ├─ understand_search  定位模块在哪
  │
  ├─ codegraph_explore  拿相关符号的完整源码（替代 grep+read 循环）
  │
  └─ grep / read_file  仅两个图谱都不能用时降级
```

### 涉及的配置文件

| 文件 | 改动 |
|------|------|
| `REASONIX.md` | 新增 Code Intelligence 决策表 + 强制工作流 + 反模式清单 |
| `internal/understand/prefix.go` | TurnHint 自动检测 CodeGraph 目录，追加工具提示 |
| `reasonix.toml` | 注册 codegraph 插件 |
| `.cgcignore` | 排除 node_modules、构建产物、.codegraph 自身等 |

---

## 3. 安装与配置（小白版）

### 前提

- 电脑上有 Node.js（`node --version` 有输出即可）
- 项目根目录有 `reasonix.toml`

### 步骤 1：安装 CodeGraph

```bash
npm install -g @colbymchenry/codegraph

# 验证
codegraph version
# 应该输出 1.4.1
```

### 步骤 2：在项目根目录建索引

```bash
cd 你的项目目录
codegraph init
```

首次索引约 30 秒。完成后检查：

```bash
codegraph status
```

正常输出类似：

```
Files:     1,054
Nodes:     22,655
Edges:     79,048
```

### 步骤 3：排除不需要索引的目录

在项目根目录创建或编辑 `.cgcignore`，内容参考：

```
node_modules/
/dist/
/build/
/MyReasonixRelease/
/.codegraph/
/.understand-anything/
*.test
*.out
```

这会大幅减少索引时间。

### 步骤 4：注册到 Reasonix

在 `reasonix.toml` 末尾添加。**注意把路径替换成你自己的实际路径**：

先找到路径：

```bash
# 找到 npm 全局目录
npm root -g
# 输出类似：C:\Users\你的用户名\AppData\Roaming\npm\node_modules

# 在这个目录下找到以下两个文件：
#   @colbymchenry/codegraph/node_modules/@colbymchenry/codegraph-win32-x64/node.exe
#   @colbymchenry/codegraph/node_modules/@colbymchenry/codegraph-win32-x64/lib/dist/bin/codegraph.js
```

然后在 `reasonix.toml` 末尾添加（路径改成你找到的实际路径）：

```toml
[[plugins]]
name    = "codegraph"
command = "C:/Users/你的用户名/AppData/Roaming/npm/node_modules/@colbymchenry/codegraph/node_modules/@colbymchenry/codegraph-win32-x64/node.exe"
args    = ["--liftoff-only", "C:/Users/你的用户名/AppData/Roaming/npm/node_modules/@colbymchenry/codegraph/node_modules/@colbymchenry/codegraph-win32-x64/lib/dist/bin/codegraph.js", "serve", "--mcp"]
trusted_read_only_tools = ["codegraph_explore"]
```

### 步骤 5：重启 Reasonix

重启后 MCP 面板应显示 codegraph 为「已连接」。

---

## 4. 踩过的坑

### 坑 1：`.mcp.json` 的 tier 字段被静默忽略

**现象**：`.mcp.json` 里写 `"tier": "eager"`，CodeGraph 始终按后台懒加载处理，工具永远不注册。

**原因**：Reasonix 的 `.mcp.json` 解析代码只支持 Claude Code 兼容的字段（command、args、env、url），不支持 Reasonix 独有的 tier、timeout、trust 等字段。

**解决**：改用 `reasonix.toml` 的 `[[plugins]]` 注册。

### 坑 2：Windows 下 MCP 连接永远卡在「正在连接」

**现象**（耗时最长，试了 9 种方案）：

| 尝试 | 命令格式 | 结果 |
|------|---------|------|
| 1 | npx 包装 | ❌ 启动交互式安装器 |
| 2 | npx 包装 + serve --mcp | ❌ |
| 3 | codegraph 裸命令（PATH） | ❌ |
| 4 | node + npm-shim.js | ❌ |
| 5 | node.exe 绝对路径 + npm-shim.js | ❌ |
| 6 | 加 tier: eager | ❌（.mcp.json 不支持） |
| 7 | 调用 connect 工具触发懒加载 | ❌ |
| 8 | reasonix.toml + eager tier | ❌ |
| 9 | **bundled node.exe + codegraph.js 绝对路径** | ✅ |

**验证过程**：

写了一个极简 Python MCP echo 服务器（~30 行，stdin/stdout JSON-RPC），在 Reasonix 里立即连接成功。这证明**Reasonix 本身的通道没问题**，问题出在 CodeGraph 的启动链。

**根因**：

```
CodeGraph 的启动链路：
  codegraph.cmd → node.exe → npm-shim.js → spawnSync({stdio:'inherit'})
                                                    ↓↓
                                         bundled_node.exe + codegraph.js
                                                    ↓
                                          实际 MCP 服务器

失败环节：spawnSync({stdio:'inherit'})
  在 Go exec.Command 的管道 stdin/stdout 环境下，
  子进程继承的文件描述符无法正常传递 MCP 握手信号。

成功路径（绕过 spawnSync）：
  bundled_node.exe → codegraph.js → 直接响应 MCP（零中间层）
```

### 坑 3：索引太慢

**现象**：`codegraph init` 扫描了 node_modules、发布构建产物目录、甚至自己的索引目录（循环扫描），索引时间膨胀到数分钟。

**解决**：`.cgcignore` 加了排除规则后，索引文件数降到 1,054。

---

## 5. 工具名说明

CodeGraph 的工具在 Reasonix 里的实际调用名是：

> `mcp__codegraph__codegraph_explore`

Reasonix 会给所有 MCP 工具自动加 `mcp__<服务器名>__` 前缀，防止不同服务器的工具重名。

---

## 6. 日常维护

- 更新：`npm update -g @colbymchenry/codegraph`
- 重建索引：`codegraph init`（全新）或 `codegraph sync`（增量）
- 查看索引状态：`codegraph status`
- 新增大型目录后，加到 `.cgcignore`
