// Ports packages/durable/src/harness/partial-changes.ts: what a streamed answer changed in its partial message between two partial commits, so a commit costs the new text.

package harness

import (
	"errors"
	"fmt"
	"sync"

	"github.com/MichaelKinsy/PiG/ai"
	"github.com/MichaelKinsy/PiG/chord/delta"
	"github.com/MichaelKinsy/PiG/durable"
	"github.com/MichaelKinsy/PiG/internal/jsstring"
)

// toolCallChanges is the argument changes of one tracked tool call since the last commit, and its snapshot.
type toolCallChanges struct {
	changes  []ai.JSONChange
	snapshot durable.JsonValue
}

// PartialCommit is what one partial commit writes besides the partial message itself; from PartialChanges.Take.
type PartialCommit struct {
	// plan is the plan AssignJson writes the message with: text appends, and the arguments it leaves to the changes.
	plan *AssignPlan
	// toolCalls is each tracked tool call by content index.
	toolCalls map[int]toolCallChanges
}

type trackedText struct {
	field string
	text  []byte
}

// trackedToolCall is what a tracked tool call changed since the last commit, and whether any commit took its changes yet.
type trackedToolCall struct {
	parser    ai.StreamingJSONParser
	changes   []ai.JSONChange
	committed bool
}

// PartialChanges collects, from a response's events, what changed in its partial message since the last commit, so the commit appends new
// text instead of reassigning growing strings. Reassigning makes Chord compare the old and new string, which costs the length of the string
// per commit.
//
//   - Text and thinking: the text_delta and thinking_delta events. The commit appends them where the committed string plus the deltas has the
//     new string's length, and compares otherwise. A block's end event drops them, because providers may replace the text when a block ends.
//   - Tool-call arguments: a StreamingJSONParser per call, fed with the raw argument text of each toolcall_delta, reports what each piece
//     changed. The commit applies those changes to the committed arguments in order and leaves the arguments out of the message assignment,
//     so the committed arguments are exactly the parsed text so far. toolcall_end ends the tracking; the call's last commit assigns its final
//     arguments.
//
// The event consumer and the commit run on different goroutines, so the state is under a mutex.
type PartialChanges struct {
	mu        sync.Mutex
	text      map[int]*trackedText
	toolCalls map[int]*trackedToolCall
}

// NewPartialChanges returns a collector with nothing tracked.
func NewPartialChanges() *PartialChanges {
	return &PartialChanges{text: map[int]*trackedText{}, toolCalls: map[int]*trackedToolCall{}}
}

// Observe records what event changed.
func (changes *PartialChanges) Observe(event ai.AssistantMessageEvent) {
	changes.mu.Lock()
	defer changes.mu.Unlock()
	switch event := event.(type) {
	case ai.TextDeltaEvent:
		changes.appendText(event.ContentIndex, "text", event.Delta)
	case ai.ThinkingDeltaEvent:
		changes.appendText(event.ContentIndex, "thinking", event.Delta)
	case ai.ToolCallStartEvent:
		changes.toolCalls[event.ContentIndex] = &trackedToolCall{}
	case ai.ToolCallDeltaEvent:
		call := changes.toolCalls[event.ContentIndex]
		if call == nil {
			return
		}
		for _, change := range call.parser.Append(event.Delta) {
			// Arguments are an object: any other root shows as no arguments, so the snapshot is committed instead.
			if len(change.Path) == 0 {
				if _, isRecord := change.Value.(map[string]any); change.Kind != "set" || !isRecord {
					delete(changes.toolCalls, event.ContentIndex)
					return
				}
			}
			call.changes = append(call.changes, change)
		}
	case ai.TextEndEvent:
		delete(changes.text, event.ContentIndex)
	case ai.ThinkingEndEvent:
		delete(changes.text, event.ContentIndex)
	case ai.ToolCallEndEvent:
		delete(changes.toolCalls, event.ContentIndex)
	}
}

func (changes *PartialChanges) appendText(index int, field, delta string) {
	if pending := changes.text[index]; pending != nil && pending.field == field {
		pending.text = append(pending.text, delta...)
		return
	}
	changes.text[index] = &trackedText{field: field, text: []byte(delta)}
}

// Take takes what changed since the last call, for a commit of message, a copy of the partial. It clears the arguments of tracked tool
// calls in message: the commit writes them from the changes. Call Restore if the commit fails.
func (changes *PartialChanges) Take(message durable.JsonObject) *PartialCommit {
	changes.mu.Lock()
	defer changes.mu.Unlock()
	blocks := map[any]*AssignPlan{}
	for index, pending := range changes.text {
		blocks[index] = &AssignPlan{Kind: planFields, Fields: map[any]*AssignPlan{pending.field: {Kind: planAppend, Text: string(pending.text)}}}
	}
	changes.text = map[int]*trackedText{}
	taken := &PartialCommit{toolCalls: map[int]toolCallChanges{}}
	content, _ := message.Value("content").([]any)
	for index, call := range changes.toolCalls {
		if index < 0 || index >= len(content) {
			continue
		}
		block, ok := content[index].(*delta.JsonObject)
		if !ok {
			continue
		}
		taken.toolCalls[index] = toolCallChanges{changes: call.changes, snapshot: block.Value("arguments")}
		block.Set("arguments", delta.JsonObjectOf())
		blocks[index] = &AssignPlan{Kind: planFields, Fields: map[any]*AssignPlan{"arguments": {Kind: planKeep}}}
		call.changes = nil
	}
	taken.plan = &AssignPlan{Kind: planFields, Fields: map[any]*AssignPlan{"content": {Kind: planFields, Fields: blocks}}}
	return taken
}

// Commit writes message with what taken took into generation.message: the message leaf by leaf with the plan, then the argument changes. A
// tracked call whose block is new here gets its arguments from its changes, which then start at its first piece; when the committed message
// lost a call whose changes were already committed (another writer reset it), the call falls back to its snapshot from now on.
func (changes *PartialChanges) Commit(generation *delta.Object, message durable.JsonObject, taken *PartialCommit) error {
	changes.mu.Lock()
	defer changes.mu.Unlock()
	committedBlocks := 0
	if committed := generation.Object("message"); committed != nil {
		if content := committed.Array("content"); content != nil {
			committedBlocks = content.Len()
		}
	}
	messageContent, _ := message.Value("content").([]any)
	type applied struct {
		index   int
		changes []ai.JSONChange
	}
	var apply []applied
	for index, taken := range taken.toolCalls {
		call := changes.toolCalls[index]
		if call == nil {
			continue
		}
		if index >= committedBlocks && call.committed {
			// The earlier changes are not in the committed message: commit the snapshot and stop tracking the call.
			delete(changes.toolCalls, index)
			messageContent[index].(*delta.JsonObject).Set("arguments", taken.snapshot)
			continue
		}
		call.committed = true
		apply = append(apply, applied{index, taken.changes})
	}
	if err := assignSlot(objectSlot{generation, "message"}, message, taken.plan); err != nil {
		return err
	}
	content := generation.Object("message").Array("content")
	for _, entry := range apply {
		block := content.Object(entry.index)
		for _, change := range entry.changes {
			if err := applyArgumentChange(block, change); err != nil {
				return err
			}
		}
	}
	return nil
}

// Restore puts back what a failed commit took, so the next commit applies it first.
func (changes *PartialChanges) Restore(taken *PartialCommit) {
	changes.mu.Lock()
	defer changes.mu.Unlock()
	for index, entry := range taken.toolCalls {
		if call := changes.toolCalls[index]; call != nil {
			call.changes = append(append([]ai.JSONChange(nil), entry.changes...), call.changes...)
		}
	}
}

// applyArgumentChange applies one argument change to the committed arguments of block.
func applyArgumentChange(block *delta.Object, change ai.JSONChange) error {
	if len(change.Path) == 0 {
		// Only an object root is tracked, and only a set starts it.
		if change.Kind == "set" {
			return block.Set("arguments", change.Value)
		}
		return nil
	}
	var parent any = block.Object("arguments")
	for _, segment := range change.Path[:len(change.Path)-1] {
		switch container := parent.(type) {
		case *delta.Object:
			parent = container.Get(segment.(string))
		case *delta.Array:
			parent = container.Get(segment.(int))
		default:
			return errors.New("partial arguments path leaves the committed arguments")
		}
	}
	key := change.Path[len(change.Path)-1]
	switch container := parent.(type) {
	case *delta.Object:
		name, ok := key.(string)
		if !ok {
			return fmt.Errorf("partial arguments key %v is not a member name", key)
		}
		if change.Kind == "set" {
			return container.Set(name, change.Value)
		}
		return delta.AppendText(container, name, change.Text)
	case *delta.Array:
		index, ok := key.(int)
		if !ok {
			return fmt.Errorf("partial arguments key %v is not an element index", key)
		}
		value := change.Value
		if change.Kind != "set" {
			// AppendText takes object members; a string in an array is reassigned, which compares it whole.
			previous, _ := container.Get(index).(string)
			value = jsstring.Concat(previous, change.Text)
		}
		if index == container.Len() {
			_, err := container.Push(value)
			return err
		}
		return container.Set(index, value)
	}
	return errors.New("partial arguments path leaves the committed arguments")
}
