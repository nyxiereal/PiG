//go:build pig_strip_mcp

package coding

import (
	"fmt"

	"github.com/MichaelKinsy/PiG/coding/extension"
	"github.com/MichaelKinsy/PiG/coding/extension/host/subprocess"
)

func (f graphExtensionFactory) mcpFactory(subprocess.SessionGraphScope, string) (extension.InlineExtension, error) {
	return nil, fmt.Errorf("the compiled-in extension %q is not in this build", f.Builtin)
}
