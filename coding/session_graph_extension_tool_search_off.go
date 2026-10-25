//go:build pig_strip_tool_search

package coding

import "github.com/MichaelKinsy/PiG/coding/extension"

func (f graphExtensionFactory) toolSearchFactory() (extension.InlineExtension, error) {
	return f.strippedFactory()
}
