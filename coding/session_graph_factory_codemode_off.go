//go:build pig_strip_codemode

// pig-specific: a build without codemode serves no codemode factory.

package coding

import "github.com/MichaelKinsy/PiG/coding/extension"

func (f graphExtensionFactory) codemode() (extension.InlineExtension, error) {
	return nil, f.strippedError()
}
