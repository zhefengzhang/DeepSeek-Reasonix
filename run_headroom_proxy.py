"""Patch uvicorn for Python 3.12+ Windows and start headroom proxy."""
import sys, os

# Patch uvicorn LOOP_SETUPS for Python 3.12+ Windows (SelectorEventLoop removed)
if sys.platform == "win32":
    import uvicorn.config
    from uvicorn.config import LOOP_SETUPS
    if "asyncio:SelectorEventLoop" not in LOOP_SETUPS:
        LOOP_SETUPS["asyncio:SelectorEventLoop"] = LOOP_SETUPS["asyncio"]

# Start headroom proxy
from headroom.cli.proxy import proxy_cli
sys.exit(proxy_cli())
