"""Pi's ``createCodemodeExtension``, ``createToolSearchExtension`` and ``createMcpExtension`` (extensions/codemode/index.ts, tool-search/index.ts, mcp/index.ts).

Pi's functions return an ExtensionFactory, a function its DefaultResourceLoader runs in its own process. A function cannot cross the process boundary, so these return a reference to the compiled-in extension of that name: pass
it in the ``extension_factories`` list of :class:`DefaultResourceLoader` and the host runs its own factory of that name, with the same options, for the extensions the loader loads.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


@dataclass(frozen=True)
class ExtensionFactory:
    """A reference to a compiled-in extension of the host, which :class:`DefaultResourceLoader` takes in ``extension_factories``."""

    builtin: str
    options: dict[str, Any] = field(default_factory=dict)


def create_codemode_extension(*, mode: str | None = None, inline_budget: float | None = None, models: bool | None = None) -> ExtensionFactory:
    """``createCodemodeExtension(options)``: the ``codemode`` tool, registered inactive. ``mode`` ("on" or "only") and ``inline_budget`` override the ``codemode`` settings; ``models=False`` hides the model catalog from scripts. ``None`` is Pi's undefined."""
    options: dict[str, Any] = {}
    for value, key in ((mode, "mode"), (inline_budget, "inlineBudget"), (models, "models")):
        if value is not None:
            options[key] = value
    return ExtensionFactory("codemode", options)


def create_tool_search_extension() -> ExtensionFactory:
    """``createToolSearchExtension()``: the ``tool_search`` tool, registered inactive."""
    return ExtensionFactory("tool-search", {})


def create_mcp_extension(*, log_path: str | None = None, startup_wait_ms: float | None = None) -> ExtensionFactory:
    """``createMcpExtension(options)``: the MCP extension, with the data part of Pi's options. Pi's function options (``loadConfig``, ``createTransport``, ``credentials``, ``openUrl``, ``updateConfig``) run caller code in the host and are not served yet."""
    options: dict[str, Any] = {}
    for value, key in ((log_path, "logPath"), (startup_wait_ms, "startupWaitMs")):
        if value is not None:
            options[key] = value
    return ExtensionFactory("mcp", options)
