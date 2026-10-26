// Package compiledin is the registry of the compiled-in extensions an extension names by reference (createCodemodeExtension, createToolSearchExtension): Pi's
// factories return functions, which cannot cross the process boundary, so the SDKs send the name and the options and the host builds its own factory of that name
// (coding/session_graph_extension_factories.go).
//
// A built-in registers its factory from the package that implements it (coding/extension/builtin), not from the session code that looks it up: a program that
// imports the coding library without the built-ins links neither the sandbox's wasm runtime nor the tool-search index, and a reference to a built-in it does not
// link fails as in a build that strips it.
package compiledin

import "github.com/MichaelKinsy/PiG/coding/extension"

// Factory builds one compiled-in extension. decode fills v from the options the extension passed, and does nothing when it passed none; its error names the extension.
type Factory func(decode func(v any) error) (extension.InlineExtension, error)

// factories is written by Register during package initialization and only read after it.
var factories = map[string]Factory{}

// Register records the factory of the compiled-in extension name. It is for package initialization.
func Register(name string, factory Factory) { factories[name] = factory }

// Lookup returns the factory of the compiled-in extension name.
func Lookup(name string) (Factory, bool) {
	factory, ok := factories[name]
	return factory, ok
}
