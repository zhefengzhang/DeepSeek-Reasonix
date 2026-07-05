# Headroom Windows 兼容性改进记录

> 创建日期：2026-07-02
> 关联代码：`desktop/headroom_sidecar.go` `writeWindowsLauncher()` 函数
> 关联 Issue：headroom proxy 在 Windows Python 3.12+ 上启动失败

## 背景

Headroom proxy (`headroom proxy`) 在 Windows 上通过 uvicorn 启动 HTTP 服务时，传递了
`loop="asyncio:SelectorEventLoop"` 参数给 `uvicorn.run()`。该参数是旧版 uvicorn 的有效
LOOP_SETUPS key，但在 uvicorn 0.34.0+ 中已被简化，仅保留 `"asyncio"`、`"auto"`、`"none"`、
`"uvloop"` 四个 key。

Python 3.12+ 上 Windows 的默认事件循环已从 SelectorEventLoop 改为 ProactorEventLoop。
headroom 在此传递该参数是为了强制使用 SelectorEventLoop，避免 ProactorEventLoop 因
AcceptEx 错误（如 WinError 64 keep-alive RST）关闭监听 socket 的问题。

### 错误栈

```
KeyError: 'asyncio:SelectorEventLoop'
  File "uvicorn/config.py", line 476, in setup_event_loop
    loop_setup = import_from_string(LOOP_SETUPS[self.loop])
```

## 当前方案（Phase 2 实现）

在 `desktop/headroom_sidecar.go` 中，`writeWindowsLauncher()` 生成一个 Python launcher 脚本，
在启动 headroom proxy 之前修补 uvicorn 的 LOOP_SETUPS 字典：

```python
import uvicorn.config
from uvicorn.config import LOOP_SETUPS
LOOP_SETUPS["asyncio:SelectorEventLoop"] = LOOP_SETUPS["asyncio"]
```

这给字典注入了一个别名 key，使 uvicorn 能找到对应的设置函数。

## 已知脆弱点

当前补丁依赖 headroom 和 uvicorn 的三个**内部实现假设**，任何一个被破坏就会再次报错：

| 假设 | 依赖方 | 破坏场景 |
|------|--------|---------|
| headroom 仍传递 `"asyncio:SelectorEventLoop"` 字符串 | headroom server.py | headroom 升级修改了 loop 参数格式 |
| uvicorn 的 LOOP_SETUPS 仍有 `"asyncio"` key | uvicorn config.py | uvicorn 重构 LOOP_SETUPS 结构 |
| uvicorn 的 `setup_event_loop()` 方法签名不变 | uvicorn server.py | uvicorn 改用其他初始化方式 |

## 推荐改进方案（方案 B：全局 policy + 函数代理）

### 原理

从**上层**阻断问题，而不是修补内部字典：

1. **全局设置事件循环 policy** — `asyncio.set_event_loop_policy(WindowsSelectorEventLoopPolicy())`
   确保无论 uvicorn 用什么 loop 配置，实际创建的事件循环都是 SelectorEventLoop。
   `WindowsSelectorEventLoopPolicy` 在 CPython 3.8–3.13 均可用（3.12+ 仅 deprecated 但未删除）。

2. **猴子补丁 `run_server`** — 拦截 `loop` 参数，将 headroom 传递的不可用 key 替换为
   uvicorn 始终支持的 `"asyncio"`。不匹配时透传，不影响其他行为。

### 完整代码

以下代码替换 `writeWindowsLauncher()` 生成的 launcher 内容：

```python
import asyncio
import os
import sys

# 1. 全局设置 SelectorEventLoop 策略（Python 3.8-3.13 可用）
if sys.platform == "win32":
    asyncio.set_event_loop_policy(asyncio.WindowsSelectorEventLoopPolicy())

# 2. 猴子补丁 headroom 的 run_server，拦截 loop 参数
from headroom.proxy.server import run_server as _original_run_server

def _patched_run_server(config, **kwargs):
    if "loop" in kwargs:
        # 无论 headroom 传递什么 loop 字符串，都替换为 uvicorn 始终支持的 "asyncio"
        # 同时全局 policy 确保实际事件循环是 SelectorEventLoop
        kwargs["loop"] = "asyncio"
    return _original_run_server(config, **kwargs)

import headroom.proxy.server
headroom.proxy.server.run_server = _patched_run_server

# 3. 启动 headroom proxy
from headroom.proxy.server import run_server, ProxyConfig

port = int(os.environ.get("HEADROOM_PORT", "8787"))
mode = os.environ.get("HEADROOM_MODE", "token")

config = ProxyConfig(
    host="127.0.0.1",
    port=port,
    mode=mode,
    openai_api_url=os.environ.get("OPENAI_TARGET_API_URL") or None,
    anthropic_api_url=os.environ.get("ANTHROPIC_TARGET_API_URL") or None,
)
run_server(config)
```

### 优势对比

| 维度 | 当前方案 | 方案 B |
|------|---------|--------|
| 依赖 uvicorn 内部字典结构 | ✅ 是 | ❌ 否 |
| 依赖 headroom loop 参数值 | ✅ 是 | ❌ 否（拦截替换） |
| 依赖 `setup_event_loop()` 签名 | ✅ 是 | ❌ 否（参数拦截在外部） |
| headroom/uvicorn 升级后有效 | ❌ 可能失效 | ✅ 仍有效 |
| Python 版本变化影响 | ⚠️ 依赖 3.12+ 行为 | ✅ `WindowsSelectorEventLoopPolicy` 3.8-3.13 可用 |

### 风险

- `WindowsSelectorEventLoopPolicy` 已在 PEP 644 中标记为 deprecated，可能在 Python 3.14+
  或更晚版本中被正式移除。届时 headroom 的 SelectorEventLoop 硬编码本身也会出问题，
  需要更彻底的方案（如方案 E：纯 Go 实现 proxy 管理，去掉 Python 依赖）。
- 如果 headroom 未来完全移除 `run_server` 函数或改签名，补丁会静默失效（不报错，
  但不再拦截 loop 参数）。可以通过在补丁中加版本日志来监控。

## 触发条件（需要重新审视此问题）

- 升级 Python 到 3.14+（`WindowsSelectorEventLoopPolicy` 可能被移除）
- `pip install --upgrade headroom-ai` 后 proxy 再次启动失败
- `pip install --upgrade uvicorn` 后 proxy 再次启动失败
- 将 headroom 整合方案从 Python launcher 迁移到 Go 原生实现（届时此问题自然消失）

## 关联代码

- `desktop/headroom_sidecar.go` — `writeWindowsLauncher()` 函数（生成 Python launcher 脚本）
- `desktop/headroom_sidecar.go` — `start()` 方法中 `if runtime.GOOS == "windows"` 分支
