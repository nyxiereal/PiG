//go:build pig_strip_codemode

package coding

import (
	"fmt"

	"github.com/MichaelKinsy/PiG/coding/extension"
)

func (f graphExtensionFactory) codemodeFactory() (extension.InlineExtension, error) {
	return nil, fmt.Errorf("the compiled-in extension %q is not in this build", f.Builtin)
}
