// pig-specific: the compiled-in extension factories an extension passes to a DefaultResourceLoader it constructs (resource-loader.ts extensionFactories). Pi's createCodemodeExtension, createToolSearchExtension and createMcpExtension return functions, which cannot cross the process boundary; the SDKs return a named reference instead and the host instantiates its own factory of that name with the options.

package coding

import (
	"fmt"

	"github.com/MichaelKinsy/PiG/coding/extension"
	"github.com/MichaelKinsy/PiG/coding/extension/builtin/codemode"
	"github.com/MichaelKinsy/PiG/coding/extension/builtin/toolsearch"
	"github.com/MichaelKinsy/PiG/coding/mcpext"
	"github.com/MichaelKinsy/PiG/extensions/sdk/json"
	icodingagent "github.com/MichaelKinsy/PiG/internal/codingagent"
)

// graphExtensionFactory is one entry of the loader's extensionFactories option as it crosses the wire: the built-in's name and its JSON options.
type graphExtensionFactory struct {
	Builtin string          `json:"builtin"`
	Options json.RawMessage `json:"options"`
}

// factory builds the host's factory of the named extension. agentDir is the loader's agent directory, where the MCP extension keeps its configuration, credentials and log.
func (f graphExtensionFactory) factory(agentDir string) (extension.InlineExtension, error) {
	switch f.Builtin {
	case "codemode":
		// index.ts CodemodeExtensionOptions: mode, inlineBudget and models (default true).
		options, err := decodeFactoryOptions[struct {
			Mode         string   `json:"mode"`
			InlineBudget *float64 `json:"inlineBudget"`
			Models       *bool    `json:"models"`
		}](f)
		if err != nil {
			return nil, err
		}
		if options.Mode != "" && options.Mode != codemode.ModeOn && options.Mode != codemode.ModeOnly {
			return nil, fmt.Errorf(`createCodemodeExtension option mode must be "on" or "only", got %q`, options.Mode)
		}
		built := codemode.Options{Mode: options.Mode, DisableModels: options.Models != nil && !*options.Models}
		if options.InlineBudget != nil {
			budget := int(*options.InlineBudget)
			built.InlineBudget = &budget
		}
		return codemode.CreateCodemodeExtension(built), nil
	case "tool-search":
		return toolsearch.CreateToolSearchExtension(), nil
	case "mcp":
		// index.ts McpExtensionOptions: the data options logPath and startupWaitMs. The function options (loadConfig, createTransport, credentials, openUrl, updateConfig) run caller code and are not served.
		options, err := decodeFactoryOptions[struct {
			LogPath       string   `json:"logPath"`
			StartupWaitMs *float64 `json:"startupWaitMs"`
		}](f)
		if err != nil {
			return nil, err
		}
		built := mcpext.Options{LogPath: options.LogPath, AgentDir: agentDir, ConfigDirName: icodingagent.ConfigDirName()}
		if options.StartupWaitMs != nil {
			wait := int(*options.StartupWaitMs)
			built.StartupWaitMs = &wait
		}
		return mcpext.CreateMcpExtension(built), nil
	}
	return nil, fmt.Errorf("the host serves no compiled-in extension named %q", f.Builtin)
}

func decodeFactoryOptions[T any](f graphExtensionFactory) (T, error) {
	var options T
	if len(f.Options) == 0 || string(f.Options) == "null" {
		return options, nil
	}
	if err := json.Unmarshal(f.Options, &options); err != nil {
		return options, fmt.Errorf("options of the %s extension: %w", f.Builtin, err)
	}
	return options, nil
}
