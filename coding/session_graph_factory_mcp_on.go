//go:build !pig_strip_mcp

// pig-specific: the MCP extension an extension passes as a loader extensionFactories entry.

package coding

import (
	"github.com/MichaelKinsy/PiG/coding/extension"
	"github.com/MichaelKinsy/PiG/coding/extension/host/subprocess"
	"github.com/MichaelKinsy/PiG/coding/mcpext"
	"github.com/MichaelKinsy/PiG/extensions/sdk/json"
	icodingagent "github.com/MichaelKinsy/PiG/internal/codingagent"
)

func (f graphExtensionFactory) mcp(scope subprocess.SessionGraphScope, agentDir string) (extension.InlineExtension, error) {
	// index.ts McpExtensionOptions: the data options logPath and startupWaitMs, and the function options (loadConfig, createTransport, credentials, openUrl, updateConfig), which the extension names by callback id (session_graph_mcp.go).
	var options struct {
		LogPath       string   `json:"logPath"`
		StartupWaitMs *float64 `json:"startupWaitMs"`
		graphMcpFunctions
	}
	if f.hasOptions() {
		if err := json.Unmarshal(f.Options, &options); err != nil {
			return nil, f.optionsError(err)
		}
	}
	built := mcpext.Options{LogPath: options.LogPath, AgentDir: agentDir, ConfigDirName: icodingagent.ConfigDirName()}
	if options.StartupWaitMs != nil {
		wait := int(*options.StartupWaitMs)
		built.StartupWaitMs = &wait
	}
	options.apply(scope, &built)
	return mcpext.CreateMcpExtension(built), nil
}
