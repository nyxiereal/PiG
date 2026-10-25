// pig-specific: the compiled-in extension factories an extension passes to a DefaultResourceLoader it constructs (resource-loader.ts extensionFactories). Pi's createCodemodeExtension, createToolSearchExtension and createMcpExtension return functions, which cannot cross the process boundary; the SDKs return a named reference instead and the host instantiates its own factory of that name with the options.

package coding

import (
	"fmt"

	"github.com/MichaelKinsy/PiG/coding/extension"
	"github.com/MichaelKinsy/PiG/coding/extension/host/subprocess"
	"github.com/MichaelKinsy/PiG/extensions/sdk/json"
)

// graphExtensionFactory is one entry of the loader's extensionFactories option as it crosses the wire: the built-in's name and its JSON options.
type graphExtensionFactory struct {
	Builtin string          `json:"builtin"`
	Options json.RawMessage `json:"options"`
}

// factory builds the host's factory of the named extension. agentDir is the loader's agent directory, where the MCP extension keeps its configuration, credentials and log. scope is the extension's connection, where the functions an option names run.
// Each built-in's factory lives in a file of its own strip tag (session_graph_extension_{codemode,tool_search,mcp}*.go), so a build that strips a built-in links none of its packages.
func (f graphExtensionFactory) factory(scope subprocess.SessionGraphScope, agentDir string) (extension.InlineExtension, error) {
	switch f.Builtin {
	case "codemode":
		return f.codemodeFactory()
	case "tool-search":
		return f.toolSearchFactory()
	case "mcp":
		return f.mcpFactory(scope, agentDir)
	}
	return nil, fmt.Errorf("the host serves no compiled-in extension named %q", f.Builtin)
}

func (f graphExtensionFactory) hasOptions() bool {
	return len(f.Options) != 0 && string(f.Options) != "null"
}

func (f graphExtensionFactory) optionsError(err error) error {
	return fmt.Errorf("options of the %s extension: %w", f.Builtin, err)
}
