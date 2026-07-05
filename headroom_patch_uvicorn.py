"""PYTHONSTARTUP script: patch uvicorn LOOP_SETUPS for Python 3.12+ Windows."""
import sys
if sys.platform == "win32":
    try:
        import uvicorn.config
        from uvicorn.config import LOOP_SETUPS
        if "asyncio:SelectorEventLoop" not in LOOP_SETUPS:
            LOOP_SETUPS["asyncio:SelectorEventLoop"] = LOOP_SETUPS["asyncio"]
    except Exception:
        pass  # uvicorn not yet imported
