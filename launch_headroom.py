"""Launch headroom proxy with uvicorn patch for Python 3.12+ Windows and DeepSeek upstream."""
import os, sys

# Set upstream target BEFORE importing headroom
os.environ["OPENAI_TARGET_API_URL"] = "https://api.deepseek.com/v1"

if sys.platform == "win32":
    import uvicorn.config
    from uvicorn.config import LOOP_SETUPS
    if "asyncio:SelectorEventLoop" not in LOOP_SETUPS:
        LOOP_SETUPS["asyncio:SelectorEventLoop"] = LOOP_SETUPS["asyncio"]

from headroom.cli import proxy as proxy_cli_module
from click.testing import CliRunner

runner = CliRunner()
sys.argv = ["headroom", "proxy", "--port", "8787", "--log-file", r"C:\Users\Administrator\headroom_test.jsonl"]
# Call the proxy function directly
proxy_cli_module.proxy()
