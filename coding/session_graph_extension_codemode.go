//go:build !pig_strip_codemode

package coding

import (
	"fmt"

	"github.com/MichaelKinsy/PiG/coding/extension"
	"github.com/MichaelKinsy/PiG/coding/extension/builtin/codemode"
	"github.com/MichaelKinsy/PiG/extensions/sdk/json"
)

// codemodeFactory is createCodemodeExtension (index.ts CodemodeExtensionOptions: mode, inlineBudget and models, default true).
func (f graphExtensionFactory) codemodeFactory() (extension.InlineExtension, error) {
	var options struct {
		Mode         string   `json:"mode"`
		InlineBudget *float64 `json:"inlineBudget"`
		Models       *bool    `json:"models"`
	}
	if f.hasOptions() {
		if err := json.Unmarshal(f.Options, &options); err != nil {
			return nil, f.optionsError(err)
		}
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
}
