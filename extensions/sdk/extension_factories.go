package sdk

// Pi's createCodemodeExtension, createToolSearchExtension and createMcpExtension (extensions/codemode/index.ts, tool-search/index.ts, mcp/index.ts) return an ExtensionFactory, a function Pi's DefaultResourceLoader runs in its own process. A function
// cannot cross the process boundary, so these return a reference to the compiled-in extension of that name: pass it in [DefaultResourceLoaderOptions].ExtensionFactories and the host runs its own factory of that name, with the same options, for
// the extensions the loader loads.

// ExtensionFactory is a reference to a compiled-in extension of the host, which [NewDefaultResourceLoader] takes in its ExtensionFactories option.
type ExtensionFactory struct {
	builtin string
	options map[string]any
}

// CodemodeExtensionOptions is Pi's CodemodeExtensionOptions. A zero member is Pi's undefined.
type CodemodeExtensionOptions struct {
	// Mode overrides the `codemode.mode` setting: "on" or "only".
	Mode string
	// InlineBudget overrides the `codemode.inlineBudget` setting.
	InlineBudget *float64
	// Models exposes the model catalog and classifiers to scripts as `models`. Nil is Pi's default, true.
	Models *bool
}

// CreateCodemodeExtension is `createCodemodeExtension(options)`: the `codemode` tool, registered inactive. Activate it with the tools option, the `defaultTools` setting or `setActiveTools`.
func CreateCodemodeExtension(options ...CodemodeExtensionOptions) ExtensionFactory {
	wire := map[string]any{}
	if len(options) > 0 {
		given := options[0]
		if given.Mode != "" {
			wire["mode"] = given.Mode
		}
		if given.InlineBudget != nil {
			wire["inlineBudget"] = *given.InlineBudget
		}
		if given.Models != nil {
			wire["models"] = *given.Models
		}
	}
	return ExtensionFactory{builtin: "codemode", options: wire}
}

// CreateToolSearchExtension is `createToolSearchExtension()`: the `tool_search` tool, registered inactive.
func CreateToolSearchExtension() ExtensionFactory {
	return ExtensionFactory{builtin: "tool-search", options: map[string]any{}}
}

// McpExtensionOptions is the data part of Pi's McpExtensionOptions. A zero member is Pi's undefined. Pi's function options (`loadConfig`, `createTransport`, `credentials`, `openUrl`, `updateConfig`) run caller code in the host and are not served yet.
type McpExtensionOptions struct {
	// LogPath is the file server log messages are appended to. Empty is `mcp.log` in the agent directory.
	LogPath string
	// StartupWaitMs is how long the first prompt waits for servers with `direct` tools that are still connecting at startup. Nil is Pi's default, 10000.
	StartupWaitMs *float64
}

// CreateMcpExtension is `createMcpExtension(options)`: the MCP extension.
func CreateMcpExtension(options ...McpExtensionOptions) ExtensionFactory {
	wire := map[string]any{}
	if len(options) > 0 {
		given := options[0]
		if given.LogPath != "" {
			wire["logPath"] = given.LogPath
		}
		if given.StartupWaitMs != nil {
			wire["startupWaitMs"] = *given.StartupWaitMs
		}
	}
	return ExtensionFactory{builtin: "mcp", options: wire}
}

// extensionFactoriesWire is the loader's extensionFactories option on the wire: each reference as its name and options. Any other value is a function of this process, which the host cannot run.
func extensionFactoriesWire(factories []any) ([]map[string]any, error) {
	wire := make([]map[string]any, 0, len(factories))
	for _, factory := range factories {
		reference, ok := factory.(ExtensionFactory)
		if !ok {
			return nil, errExtensionFactoryNotServed
		}
		wire = append(wire, map[string]any{"builtin": reference.builtin, "options": reference.options})
	}
	return wire, nil
}
