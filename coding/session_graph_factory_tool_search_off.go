//go:build pig_strip_tool_search

// pig-specific: a build without tool search serves no tool-search factory.

package coding

import "github.com/MichaelKinsy/PiG/coding/extension"

func (f graphExtensionFactory) toolSearch() (extension.InlineExtension, error) {
	return nil, f.strippedError()
}
