//go:build !pig_strip_tool_search

package coding

import (
	"github.com/MichaelKinsy/PiG/coding/extension"
	"github.com/MichaelKinsy/PiG/coding/extension/builtin/toolsearch"
)

// toolSearchFactory is createToolSearchExtension.
func (f graphExtensionFactory) toolSearchFactory() (extension.InlineExtension, error) {
	return toolsearch.CreateToolSearchExtension(), nil
}
