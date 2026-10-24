package extensionconformance

import (
	"encoding/json"
	"testing"
)

// TestConformance_TypeSchemaToolParametersAgreeAcrossSDKs: every SDK registers the tool "type-schema-tool" with parameters built by pi-ai's Type (Go sdk.Type, Python Type, Rust Type; the
// Node runtime runs Pi's own), and the host holds the schema Pi's Type returns for the same calls: the vector "conformance tool parameters" of test/parity/sdk-vectors/pi-ai/Type.json.
// An SDK whose Type returned another schema (an Optional property listed as required, a Record without its pattern, a changed key order below the root) fails here.
func TestConformance_TypeSchemaToolParametersAgreeAcrossSDKs(t *testing.T) {
	var want string
	for _, c := range piVectorCases(t, "pi-ai", "Type") {
		if c.Name != "conformance tool parameters" {
			continue
		}
		var result struct {
			JSON string `json:"json"`
		}
		if err := json.Unmarshal(c.Result, &result); err != nil || result.JSON == "" {
			t.Fatalf("vector result %s: %v", c.Result, err)
		}
		want = result.JSON
	}
	if want == "" {
		t.Fatal("the vector \"conformance tool parameters\" is missing")
	}
	for _, tc := range allHarnessCases() {
		if tc.name == "inproc-go" {
			continue // the in-process Go reference registers through the host API, with no SDK (and so no sdk.Type) in the loop
		}
		t.Run(tc.name, func(t *testing.T) {
			h := tc.make(t)
			t.Cleanup(func() {
				if h.cleanup != nil {
					h.cleanup()
				}
				if h.host != nil {
					h.host.Shutdown("test done")
				}
			})
			for _, tool := range h.runner.Tools() {
				if tool.Definition.Name != "type-schema-tool" {
					continue
				}
				// The root of a Go map schema has no order of its own; the vectors check the order of every constructor's output.
				if got, expected := piAiCanonicalJSON(t, tool.Definition.Parameters, false), piAiCanonicalJSON(t, []byte(want), false); got != expected {
					t.Fatalf("parameters %s, Pi's Type returns %s", got, expected)
				}
				return
			}
			t.Fatal("type-schema-tool is not registered")
		})
	}
}
