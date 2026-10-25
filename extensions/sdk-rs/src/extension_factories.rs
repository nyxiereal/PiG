//! Pi's `createCodemodeExtension`, `createToolSearchExtension` and `createMcpExtension` (extensions/codemode/index.ts, tool-search/index.ts, mcp/index.ts).
//!
//! Pi's functions return an ExtensionFactory, a function its DefaultResourceLoader runs in its own process. A function cannot cross the process boundary, so these return a reference to the compiled-in extension of that name:
//! pass it in `DefaultResourceLoaderOptions::extension_factories` and the host runs its own factory of that name, with the same options, for the extensions the loader loads.

use serde_json::{json, Map, Value};

/// A factory of an inline extension. Not supported: it would run in this process but the child session's extensions load in the host.
pub type InlineExtensionFactory = Box<dyn Fn(&mut crate::Extension) + Send + Sync>;

/// An entry of `DefaultResourceLoaderOptions::extension_factories`: a reference to a compiled-in extension of the host (what the `create_*_extension` functions return), or an inline function of this process, which the loader refuses.
pub enum ExtensionFactory {
    Builtin { name: &'static str, options: Value },
    Inline(InlineExtensionFactory),
}

/// Pi's `CodemodeExtensionOptions`. `None` is Pi's undefined.
#[derive(Debug, Clone, Default)]
pub struct CodemodeExtensionOptions {
    /// Overrides the `codemode.mode` setting: `"on"` or `"only"`.
    pub mode: Option<String>,
    /// Overrides the `codemode.inlineBudget` setting.
    pub inline_budget: Option<f64>,
    /// Exposes the model catalog and classifiers to scripts as `models`. `None` is Pi's default, true.
    pub models: Option<bool>,
}

/// `createCodemodeExtension(options)`: the `codemode` tool, registered inactive. Activate it with the tools option, the `defaultTools` setting or `setActiveTools`.
pub fn create_codemode_extension(options: CodemodeExtensionOptions) -> ExtensionFactory {
    let mut wire = Map::new();
    if let Some(mode) = options.mode {
        wire.insert("mode".into(), json!(mode));
    }
    if let Some(budget) = options.inline_budget {
        wire.insert("inlineBudget".into(), json!(budget));
    }
    if let Some(models) = options.models {
        wire.insert("models".into(), json!(models));
    }
    ExtensionFactory::Builtin { name: "codemode", options: Value::Object(wire) }
}

/// `createToolSearchExtension()`: the `tool_search` tool, registered inactive.
pub fn create_tool_search_extension() -> ExtensionFactory {
    ExtensionFactory::Builtin { name: "tool-search", options: json!({}) }
}

/// The data part of Pi's `McpExtensionOptions`. `None` is Pi's undefined. Pi's function options (`loadConfig`, `createTransport`, `credentials`, `openUrl`, `updateConfig`) run caller code in the host and are not served yet.
#[derive(Debug, Clone, Default)]
pub struct McpExtensionOptions {
    /// The file server log messages are appended to. `None` is `mcp.log` in the agent directory.
    pub log_path: Option<String>,
    /// How long the first prompt waits for servers with `direct` tools that are still connecting at startup. `None` is Pi's default, 10000.
    pub startup_wait_ms: Option<f64>,
}

/// `createMcpExtension(options)`: the MCP extension.
pub fn create_mcp_extension(options: McpExtensionOptions) -> ExtensionFactory {
    let mut wire = Map::new();
    if let Some(path) = options.log_path {
        wire.insert("logPath".into(), json!(path));
    }
    if let Some(wait) = options.startup_wait_ms {
        wire.insert("startupWaitMs".into(), json!(wait));
    }
    ExtensionFactory::Builtin { name: "mcp", options: Value::Object(wire) }
}
