//go:build !pig_strip_tool_search

// pig-specific: the tool-search extension an extension passes as a loader extensionFactories entry.

package coding

import (
	"github.com/MichaelKinsy/PiG/coding/extension"
	"github.com/MichaelKinsy/PiG/coding/extension/builtin/toolsearch"
)

func (f graphExtensionFactory) toolSearch() (extension.InlineExtension, error) {
	return toolsearch.CreateToolSearchExtension(), nil
}
