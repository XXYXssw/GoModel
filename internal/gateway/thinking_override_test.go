package gateway

import (
	"encoding/json"
	"testing"

	"github.com/enterpilot/gomodel/internal/core"
	"github.com/enterpilot/gomodel/internal/thinkingoverrides"
)

type stubThinkingResolver struct {
	override thinkingoverrides.Override
	ok       bool
}

func (s stubThinkingResolver) Resolve(selector string) (thinkingoverrides.Override, bool) {
	return s.override, s.ok
}

func workflowForModel(model, provider string) *core.Workflow {
	return &core.Workflow{
		Resolution: &core.RequestModelResolution{
			Requested:        core.RequestedModelSelector{Model: model, ProviderHint: provider},
			ResolvedSelector: core.ModelSelector{Model: model, Provider: provider},
			ProviderName:     provider,
		},
	}
}

func testOrchestrator(override thinkingoverrides.Override, ok bool) *InferenceOrchestrator {
	return NewInferenceOrchestrator(InferenceConfig{
		ThinkingOverrides: stubThinkingResolver{override: override, ok: ok},
	})
}

func TestChatThinkingOverrideModes(t *testing.T) {
	workflow := workflowForModel("qwen3-8-27b", "RunInfra")

	tests := []struct {
		name       string
		override   thinkingoverrides.Override
		clientSet  bool
		wantEffort string
		wantNil    bool
		missing    bool
	}{
		{
			name:       "native leaves client reasoning",
			override:   thinkingoverrides.Override{Mode: thinkingoverrides.ModeNative, Thinking: true, Effort: thinkingoverrides.EffortHigh},
			clientSet:  true,
			wantEffort: thinkingoverrides.EffortLow,
		},
		{
			name:       "supplement fills unset",
			override:   thinkingoverrides.Override{Mode: thinkingoverrides.ModeSupplement, Thinking: true, Effort: thinkingoverrides.EffortHigh},
			clientSet:  false,
			wantEffort: thinkingoverrides.EffortHigh,
		},
		{
			name:       "supplement keeps client value",
			override:   thinkingoverrides.Override{Mode: thinkingoverrides.ModeSupplement, Thinking: true, Effort: thinkingoverrides.EffortHigh},
			clientSet:  true,
			wantEffort: thinkingoverrides.EffortLow,
		},
		{
			name:       "override forces effort",
			override:   thinkingoverrides.Override{Mode: thinkingoverrides.ModeOverride, Thinking: true, Effort: thinkingoverrides.EffortXHigh},
			clientSet:  true,
			wantEffort: thinkingoverrides.EffortXHigh,
		},
		{
			name:      "override thinking off strips reasoning",
			override:  thinkingoverrides.Override{Mode: thinkingoverrides.ModeOverride, Thinking: false},
			clientSet: true,
			wantNil:   true,
		},
		{
			name:       "no override leaves request alone",
			missing:    true,
			clientSet:  true,
			wantEffort: thinkingoverrides.EffortLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := testOrchestrator(tt.override, !tt.missing)
			o.thinkingOverrides = stubThinkingResolver{override: tt.override, ok: !tt.missing}
			req := &core.ChatRequest{
				Model:    "qwen3-8-27b",
				Messages: []core.Message{{Role: "user", Content: "hi"}},
			}
			if tt.clientSet {
				req.Reasoning = &core.Reasoning{Effort: thinkingoverrides.EffortLow}
			}
			got := applyChatThinkingOverride(o, req, workflow)
			if tt.wantNil {
				if got.Reasoning != nil {
					t.Fatalf("Reasoning = %#v, want nil", got.Reasoning)
				}
				return
			}
			if got.Reasoning == nil {
				t.Fatalf("Reasoning = nil, want effort %q", tt.wantEffort)
			}
			if got.Reasoning.Effort != tt.wantEffort {
				t.Fatalf("Reasoning.Effort = %q, want %q", got.Reasoning.Effort, tt.wantEffort)
			}
		})
	}
}

func TestResponsesThinkingOverrideModes(t *testing.T) {
	workflow := workflowForModel("qwen3-8-27b", "RunInfra")
	o := testOrchestrator(thinkingoverrides.Override{Mode: thinkingoverrides.ModeOverride, Thinking: true, Effort: thinkingoverrides.EffortMedium}, true)

	req := &core.ResponsesRequest{
		Model:     "qwen3-8-27b",
		Reasoning: &core.Reasoning{Effort: thinkingoverrides.EffortHigh},
		Input:     "hello",
		Stream:    true,
	}
	got := applyResponsesThinkingOverride(o, req, workflow)
	if got.Reasoning == nil || got.Reasoning.Effort != thinkingoverrides.EffortMedium {
		t.Fatalf("Reasoning = %#v, want effort medium", got.Reasoning)
	}
	if got.Input != "hello" || !got.Stream {
		t.Fatalf("responses request fields changed: %#v", got)
	}
}

func TestChatThinkingOverrideStripsReasoningAndSetsQwenThinkingOff(t *testing.T) {
	o := testOrchestrator(thinkingoverrides.Override{Mode: thinkingoverrides.ModeOverride, Thinking: false}, true)
	workflow := workflowForModel("qwen3-8-27b", "RunInfra")

	req := &core.ChatRequest{
		Model:       "qwen3-8-27b",
		Reasoning:   &core.Reasoning{Effort: thinkingoverrides.EffortHigh},
		ExtraFields: core.UnknownJSONFieldsFromMap(map[string]json.RawMessage{"reasoning_effort": json.RawMessage(`"high"`)}),
	}
	got := applyChatThinkingOverride(o, req, workflow)
	if got.Reasoning != nil {
		t.Fatalf("Reasoning = %#v, want nil", got.Reasoning)
	}
	if raw := got.ExtraFields.Lookup("reasoning_effort"); len(raw) != 0 {
		t.Fatalf("reasoning_effort still present: %s", raw)
	}
	if raw := got.ExtraFields.Lookup("enable_thinking"); string(raw) != "false" {
		t.Fatalf("enable_thinking = %s, want false", raw)
	}
	if raw := got.ExtraFields.Lookup("chat_template_kwargs"); string(raw) != `{"enable_thinking":false}` {
		t.Fatalf("chat_template_kwargs = %s, want {\"enable_thinking\":false}", raw)
	}
}

func TestChatThinkingOverrideSetsQwenThinkingOn(t *testing.T) {
	o := testOrchestrator(thinkingoverrides.Override{Mode: thinkingoverrides.ModeOverride, Thinking: true, Effort: thinkingoverrides.EffortXHigh}, true)
	workflow := workflowForModel("qwen3-8-27b", "RunInfra")

	req := &core.ChatRequest{Model: "qwen3-8-27b", Reasoning: &core.Reasoning{Effort: thinkingoverrides.EffortLow}}
	got := applyChatThinkingOverride(o, req, workflow)
	if got.Reasoning == nil || got.Reasoning.Effort != thinkingoverrides.EffortXHigh {
		t.Fatalf("Reasoning = %#v, want xhigh", got.Reasoning)
	}
	if raw := got.ExtraFields.Lookup("enable_thinking"); string(raw) != "true" {
		t.Fatalf("enable_thinking = %s, want true", raw)
	}
	if raw := got.ExtraFields.Lookup("chat_template_kwargs"); string(raw) != `{"enable_thinking":true}` {
		t.Fatalf("chat_template_kwargs = %s, want {\"enable_thinking\":true}", raw)
	}
}

func TestChatThinkingOverrideNonQwenNotTagged(t *testing.T) {
	o := testOrchestrator(thinkingoverrides.Override{Mode: thinkingoverrides.ModeOverride, Thinking: false}, true)
	workflow := workflowForModel("gpt-5", "openai")

	req := &core.ChatRequest{Model: "gpt-5", Reasoning: &core.Reasoning{Effort: thinkingoverrides.EffortHigh}}
	got := applyChatThinkingOverride(o, req, workflow)
	if got.Reasoning != nil {
		t.Fatalf("Reasoning = %#v, want nil", got.Reasoning)
	}
	if !got.ExtraFields.IsEmpty() {
		t.Fatalf("ExtraFields = %#v, want empty for non-qwen model", got.ExtraFields)
	}
}

func TestChatThinkingOverrideSupplementPreservesClientQwenField(t *testing.T) {
	o := testOrchestrator(thinkingoverrides.Override{Mode: thinkingoverrides.ModeSupplement, Thinking: true, Effort: thinkingoverrides.EffortHigh}, true)
	workflow := workflowForModel("qwen3-8-27b", "RunInfra")

	// Client explicitly disabled thinking: supplement must not re-enable it.
	req := &core.ChatRequest{
		Model:       "qwen3-8-27b",
		ExtraFields: core.UnknownJSONFieldsFromMap(map[string]json.RawMessage{"enable_thinking": json.RawMessage(`false`)}),
	}
	got := applyChatThinkingOverride(o, req, workflow)
	if got.Reasoning != nil {
		t.Fatalf("Reasoning = %#v, want nil (client set enable_thinking)", got.Reasoning)
	}
	if raw := got.ExtraFields.Lookup("enable_thinking"); string(raw) != "false" {
		t.Fatalf("enable_thinking = %s, want client value false preserved", raw)
	}
}
