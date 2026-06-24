全部完成，`tsc --noEmit` 零错误。

## Token Economy 模式——实施完成

### 默认行为变更

**现在 `reasonix code` 默认以 `economy` 模式启动。** 用户无需任何配置即可享受 token 节省。

### 使用的改动文件

| 文件 | 改动 |
|---|---|
| `src/tools/connect-source.ts` | **新建** — `connect_tool_source` 工具，按需激活可选工具源 |
| `src/code/setup.ts` | `buildCodeToolset` 支持 `tokenMode` 选项，可选工具源延迟注册 |
| `src/config.ts` | 新增 `TokenMode` 类型、`loadTokenMode()`，默认返回 `"economy"` |
| `src/cli/commands/acp.ts` | 接入 `loadTokenMode()` |
| `src/cli/commands/code.tsx` | 接入 `loadTokenMode()` |
| `src/cli/commands/desktop.ts` | 两处接入 `loadTokenMode()` |

### 节省效果

Economy 模式下，首次请求的工具 schema 从 ~25 个工具（约 15-30K tokens）减少到核心编码工具（约 10-12 个，约 6-8K tokens）。可选工具源（skills、web、plan、scaffold、java_source、code_query、memory）在 AI 首次调用 `connect_tool_source` 时按需激活，每次激活代价为一次 cache miss，但后续请求保持缓存命中。

按 100 轮对话的中等会话估算，首次请求节省约 **8-20K tokens**。若缓存命中，即使用 `deepseek-v4-flash` 的 cache-hit 价格 $0.0028/1M tokens，100 轮对话可节省约 **$0.004-0.012** 的成本。

### 使用方式

无需配置：
```bash
reasonix code  # 默认 economy 模式
```

如需切换回全量模式：
```bash
REASONIX_TOKEN_MODE=full reasonix code
```
或在 `~/.reasonix/config.json` 中设置 `"tokenMode": "full"`。