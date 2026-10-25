//go:build pig_strip_codemode

package coding

import "github.com/MichaelKinsy/PiG/coding/extension"

func (f graphExtensionFactory) codemodeFactory() (extension.InlineExtension, error) {
	return f.strippedFactory()
}
