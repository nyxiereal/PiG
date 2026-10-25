//go:build pig_strip_mcp

// pig-specific: a build without MCP serves no MCP factory.

package coding

import (
	"github.com/MichaelKinsy/PiG/coding/extension"
	"github.com/MichaelKinsy/PiG/coding/extension/host/subprocess"
)

func (f graphExtensionFactory) mcp(subprocess.SessionGraphScope, string) (extension.InlineExtension, error) {
	return nil, f.strippedError()
}
