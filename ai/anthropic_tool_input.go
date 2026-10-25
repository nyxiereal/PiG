package ai

import "encoding/json"

// anthStartInput is the input member of a content_block_start record, read as JSON.parse reads it. encoding/json refuses a number out of float64 range
// where an interface is wanted and ends the turn; JSON.parse reads it as an infinity, which the stream keeps in the call's arguments until the streamed
// text replaces them at content_block_stop (anthropic-messages.ts:659-674, :743-747).
type anthStartInput struct{ value any }

func (input *anthStartInput) UnmarshalJSON(data []byte) error {
	var value any
	err := json.Unmarshal(data, &value)
	if err != nil {
		parsed, ok := jsonParseValue(string(data))
		if !ok {
			return err
		}
		value = parsed
	}
	input.value = value
	return nil
}
