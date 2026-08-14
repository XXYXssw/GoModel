package providers

import (
	"log/slog"
	"slices"
	"strings"

	"github.com/goccy/go-json"

	"github.com/google/uuid"

	"github.com/enterpilot/gomodel/internal/streaming"
)

// ResponsesOutputToolCallState tracks one function_call item in a Responses stream.
type ResponsesOutputToolCallState struct {
	ItemID            string
	CallID            string
	Name              string
	OutputIndex       int
	Arguments         strings.Builder
	Started           bool
	Completed         bool
	PlaceholderObject bool
}

// ResponsesOutputEventState manages assistant/tool output items for Responses streams.
type ResponsesOutputEventState struct {
	responseID         string
	assistantReserved  bool
	assistantStarted   bool
	assistantDone      bool
	assistantMessageID string
	assistantText      strings.Builder

	reasoningReserved bool
	reasoningStarted  bool
	reasoningDone     bool
	reasoningItemID   string
	reasoningText     strings.Builder
}

// NewResponsesOutputEventState creates a new Responses output-item state manager.
func NewResponsesOutputEventState(responseID string) *ResponsesOutputEventState {
	return &ResponsesOutputEventState{responseID: responseID}
}

// WriteEvent renders one SSE event in Responses API format.
func (s *ResponsesOutputEventState) WriteEvent(eventName string, payload map[string]any) string {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		slog.Error("failed to marshal responses stream event", "error", err, "event", eventName, "response_id", s.responseID)
		return ""
	}
	var b strings.Builder
	b.Grow(len("event: \ndata: \n\n") + len(eventName) + len(jsonData))
	b.WriteString("event: ")
	b.WriteString(eventName)
	b.WriteString("\ndata: ")
	b.Write(jsonData)
	b.WriteString("\n\n")
	return b.String()
}

// ReserveAssistant marks that the assistant message output item occupies index 0.
func (s *ResponsesOutputEventState) ReserveAssistant() {
	s.assistantReserved = true
}

// AssistantReserved reports whether the assistant output item has been reserved.
func (s *ResponsesOutputEventState) AssistantReserved() bool {
	return s.assistantReserved
}

// AssistantStarted reports whether the assistant output item has been emitted.
func (s *ResponsesOutputEventState) AssistantStarted() bool {
	return s.assistantStarted
}

// AssistantDone reports whether the assistant output item has been completed.
func (s *ResponsesOutputEventState) AssistantDone() bool {
	return s.assistantDone
}

// AppendAssistantText appends assistant text content to the output item buffer.
func (s *ResponsesOutputEventState) AppendAssistantText(text string) {
	_, _ = s.assistantText.WriteString(text)
}

// AssistantMessageItem renders the assistant message output item payload.
func (s *ResponsesOutputEventState) AssistantMessageItem(status string, includeContent bool) map[string]any {
	item := map[string]any{
		"id":      s.assistantMessageID,
		"type":    "message",
		"status":  status,
		"role":    "assistant",
		"content": []map[string]any{},
	}
	if includeContent {
		item["content"] = []map[string]any{
			{
				"type":        "output_text",
				"text":        s.assistantText.String(),
				"annotations": []json.RawMessage{},
			},
		}
	}
	return item
}

// StartAssistantOutput emits the assistant message output_item.added event once.
func (s *ResponsesOutputEventState) StartAssistantOutput(outputIndex int) string {
	if s.assistantStarted {
		return ""
	}
	s.assistantStarted = true
	if s.assistantMessageID == "" {
		s.assistantMessageID = "msg_" + uuid.New().String()
	}
	return s.WriteEvent("response.output_item.added", map[string]any{
		"type":         "response.output_item.added",
		"item":         s.AssistantMessageItem("in_progress", false),
		"output_index": outputIndex,
	})
}

// CompleteAssistantOutput emits the assistant message output_item.done event once.
func (s *ResponsesOutputEventState) CompleteAssistantOutput(outputIndex int) string {
	if !s.assistantReserved || s.assistantDone {
		return ""
	}
	s.assistantDone = true
	return s.StartAssistantOutput(outputIndex) + s.WriteEvent("response.output_item.done", map[string]any{
		"type":         "response.output_item.done",
		"item":         s.AssistantMessageItem("completed", true),
		"output_index": outputIndex,
	})
}

// AppendAssistantDelta emits a response.output_text.delta event into buf. The
// item_id is required by Responses clients (the AI SDK schema validates every
// chunk); without it the delta is treated as an unknown chunk and the text is
// dropped, leaving clients with an empty reply. This is the per-token hot
// path, so the payload is a typed struct (no map) and the event is written
// straight into the caller's pooled buffer.
func (s *ResponsesOutputEventState) AppendAssistantDelta(buf *streaming.StreamBuffer, outputIndex int, delta string) {
	jsonData, err := json.Marshal(struct {
		Type         string `json:"type"`
		ItemID       string `json:"item_id"`
		OutputIndex  int    `json:"output_index"`
		ContentIndex int    `json:"content_index"`
		Delta        string `json:"delta"`
	}{
		Type:         "response.output_text.delta",
		ItemID:       s.assistantMessageID,
		OutputIndex:  outputIndex,
		ContentIndex: 0,
		Delta:        delta,
	})
	if err != nil {
		slog.Error("failed to marshal content delta event", "error", err, "response_id", s.responseID)
		return
	}
	buf.AppendString("event: response.output_text.delta\ndata: ")
	buf.AppendBytes(jsonData)
	buf.AppendString("\n\n")
}

// ReserveReasoning marks that a reasoning output item occupies index 0,
// ahead of the assistant message and any tool calls.
func (s *ResponsesOutputEventState) ReserveReasoning() {
	s.reasoningReserved = true
}

// ReasoningReserved reports whether a reasoning output item has been reserved.
func (s *ResponsesOutputEventState) ReasoningReserved() bool {
	return s.reasoningReserved
}

// ReasoningDone reports whether the reasoning output item has been completed.
func (s *ResponsesOutputEventState) ReasoningDone() bool {
	return s.reasoningDone
}

// ReasoningItem renders a raw reasoning output item. Provider
// reasoning_content is chain-of-thought text, not an OpenAI-generated summary,
// so it belongs in content while summary remains empty.
func (s *ResponsesOutputEventState) ReasoningItem(status string, includeContent bool) map[string]any {
	item := map[string]any{
		"id":      s.reasoningItemID,
		"type":    "reasoning",
		"status":  status,
		"summary": []map[string]any{},
	}
	if includeContent {
		item["content"] = []map[string]any{
			{"type": "reasoning_text", "text": s.reasoningText.String()},
		}
	}
	return item
}

// StartReasoningOutput emits the reasoning output_item.added event once before
// any reasoning_summary_text delta references the item.
func (s *ResponsesOutputEventState) StartReasoningOutput(outputIndex int) string {
	if s.reasoningStarted {
		return ""
	}
	s.reasoningStarted = true
	if s.reasoningItemID == "" {
		s.reasoningItemID = "rs_" + uuid.New().String()
	}
	var b strings.Builder
	b.WriteString(s.WriteEvent("response.output_item.added", map[string]any{
		"type":         "response.output_item.added",
		"item":         s.ReasoningItem("in_progress", false),
		"output_index": outputIndex,
	}))
	// Clients (Chatbox/AI SDK) only surface reasoning from the
	// response.reasoning_summary_* events; the part lifecycle carries the
	// summary_index that anchors each reasoning-delta to the open item.
	b.WriteString(s.WriteEvent("response.reasoning_summary_part.added", map[string]any{
		"type":          "response.reasoning_summary_part.added",
		"item_id":       s.reasoningItemID,
		"summary_index": 0,
	}))
	return b.String()
}

// AppendReasoningDelta starts the reasoning item if needed and emits a
// reasoning-summary delta. reasoning_summary_text.delta (not the raw
// reasoning_text.delta variant) is the event Responses clients parse for
// streaming reasoning text; the item content type stays reasoning_text.
func (s *ResponsesOutputEventState) AppendReasoningDelta(outputIndex int, delta string) string {
	var b strings.Builder
	b.WriteString(s.StartReasoningOutput(outputIndex))
	s.reasoningText.WriteString(delta)
	b.WriteString(s.WriteEvent("response.reasoning_summary_text.delta", map[string]any{
		"type":          "response.reasoning_summary_text.delta",
		"item_id":       s.reasoningItemID,
		"summary_index": 0,
		"delta":         delta,
	}))
	return b.String()
}

// CompleteReasoningOutput emits the reasoning completion and output_item.done
// events once.
func (s *ResponsesOutputEventState) CompleteReasoningOutput(outputIndex int) string {
	if !s.reasoningReserved || s.reasoningDone {
		return ""
	}
	s.reasoningDone = true
	var b strings.Builder
	b.WriteString(s.StartReasoningOutput(outputIndex))
	b.WriteString(s.WriteEvent("response.reasoning_summary_part.done", map[string]any{
		"type":          "response.reasoning_summary_part.done",
		"item_id":       s.reasoningItemID,
		"summary_index": 0,
	}))
	b.WriteString(s.WriteEvent("response.output_item.done", map[string]any{
		"type":         "response.output_item.done",
		"item":         s.ReasoningItem("completed", true),
		"output_index": outputIndex,
	}))
	return b.String()
}

// CompletedOutputItems assembles the completed output array for the final
// response.completed event, in output order: the reasoning item first (it
// always claims the first output slot), then the assistant message, then any
// completed tool calls. Clients like Chatbox and Codex render the final
// message from response.completed.response.output, so omitting it leaves them
// with an empty reply even though the delta events carried the text.
func (s *ResponsesOutputEventState) CompletedOutputItems(toolCalls map[int]*ResponsesOutputToolCallState) []map[string]any {
	items := make([]map[string]any, 0, 1+len(toolCalls))
	if s.ReasoningDone() {
		items = append(items, s.ReasoningItem("completed", true))
	}
	if s.AssistantDone() {
		items = append(items, s.AssistantMessageItem("completed", true))
	}
	indices := make([]int, 0, len(toolCalls))
	for index := range toolCalls {
		indices = append(indices, index)
	}
	slices.Sort(indices)
	for _, index := range indices {
		state := toolCalls[index]
		if state == nil || !state.Completed {
			continue
		}
		items = append(items, s.RenderToolCallItem(state, "completed", true))
	}
	return items
}

// ToolCallArguments returns the serialized argument payload for a function_call item.
func (s *ResponsesOutputEventState) ToolCallArguments(state *ResponsesOutputToolCallState) string {
	if state == nil {
		return ""
	}
	if state.PlaceholderObject && state.Arguments.Len() == 0 {
		return "{}"
	}
	return state.Arguments.String()
}

// RenderToolCallItem renders a function_call output item payload.
func (s *ResponsesOutputEventState) RenderToolCallItem(state *ResponsesOutputToolCallState, status string, includePlaceholder bool) map[string]any {
	arguments := state.Arguments.String()
	if includePlaceholder {
		arguments = s.ToolCallArguments(state)
	}
	return map[string]any{
		"id":        state.ItemID,
		"type":      "function_call",
		"status":    status,
		"call_id":   state.CallID,
		"name":      state.Name,
		"arguments": arguments,
	}
}

// StartToolCall emits the function_call output_item.added event once the item metadata is available.
func (s *ResponsesOutputEventState) StartToolCall(state *ResponsesOutputToolCallState, includePlaceholder bool) string {
	if state == nil || state.Started {
		return ""
	}
	if strings.TrimSpace(state.CallID) == "" || strings.TrimSpace(state.Name) == "" {
		return ""
	}
	state.CallID = ResponsesFunctionCallCallID(state.CallID)
	state.ItemID = ResponsesFunctionCallItemID(state.CallID)
	state.Started = true
	return s.WriteEvent("response.output_item.added", map[string]any{
		"type":         "response.output_item.added",
		"item":         s.RenderToolCallItem(state, "in_progress", includePlaceholder),
		"output_index": state.OutputIndex,
	})
}

// CompleteToolCall emits the argument completion and output_item.done events once.
func (s *ResponsesOutputEventState) CompleteToolCall(state *ResponsesOutputToolCallState, includePlaceholder bool) string {
	if state == nil || state.Completed {
		return ""
	}
	state.Completed = true
	return s.WriteEvent("response.function_call_arguments.done", map[string]any{
		"type":         "response.function_call_arguments.done",
		"item_id":      state.ItemID,
		"output_index": state.OutputIndex,
		"arguments":    s.ToolCallArguments(state),
	}) + s.WriteEvent("response.output_item.done", map[string]any{
		"type":         "response.output_item.done",
		"item":         s.RenderToolCallItem(state, "completed", includePlaceholder),
		"output_index": state.OutputIndex,
	})
}
