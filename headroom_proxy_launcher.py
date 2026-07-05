"""Patch uvicorn LOOP_SETUPS for Python 3.12+ Windows, then launch headroom proxy."""
import sys

# Patch before anything else tries to use uvicorn
if sys.platform == "win32":
    import uvicorn.config
    from uvicorn.config import LOOP_SETUPS
    if "asyncio:SelectorEventLoop" not in LOOP_SETUPS:
        LOOP_SETUPS["asyncio:SelectorEventLoop"] = LOOP_SETUPS["asyncio"]

# Now launch headroom proxy
import subprocess
import os

env = os.environ.copy()
cmd = ["headroom", "proxy", "--port", "8787", "--log-file", r"C:\Users\Administrator\headroom_test.jsonl"]
sys.exit(subprocess.call(cmd, env=env))
