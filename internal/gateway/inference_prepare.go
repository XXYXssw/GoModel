package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/enterpilot/gomodel/internal/core"
	"github.com/enterpilot/gomodel/internal/thinkingoverrides"
)

// PrepareChatRequest resolves workflow/model policy and applies translated request patching.
func (o *InferenceOrchestrator) PrepareChatRequest(ctx context.Context, req *core.ChatRequest, meta RequestMeta) (*PreparedChatRequest, error) {
	return prepareTranslated(o, ctx, req, meta, chatPrepareSpec)
}

// PrepareResponsesRequest resolves workflow/model policy and applies translated request patching.
func (o *InferenceOrchestrator) PrepareResponsesRequest(ctx context.Context, req *core.ResponsesRequest, meta RequestMeta) (*PreparedResponsesRequest, error) {
	return prepareTranslated(o, ctx, req, meta, responsesPrepareSpec)
}

// PrepareEmbeddingRequest resolves workflow/model policy for an embeddings request.
func (o *InferenceOrchestrator) PrepareEmbeddingRequest(ctx context.Context, req *core.EmbeddingRequest, meta RequestMeta) (*PreparedEmbeddingRequest, error) {
	if req == nil {
		return nil, core.NewInvalidRequestError("embeddings request is required", nil)
	}
	ctx = contextWithRequestID(ctx, meta.RequestID)
	workflow, err := o.ensureTranslatedRequestWorkflow(ctx, meta.Workflow, meta.RequestID, meta.Endpoint, &req.Model, &req.Provider)
	if err != nil {
		return nil, err
	}
	ctx = o.WithCacheRequestContext(ctx, workflow)
	return &PreparedEmbeddingRequest{Context: ctx, Request: req, Workflow: workflow}, nil
}

type translatedPrepareSpec[Req any, Prepared any] struct {
	requiredMessage string
	patchNilMessage string
	selector        func(Req) (*string, *string)
	patch           func(*InferenceOrchestrator) func(context.Context, Req) (Req, error)
	thinking        func(*InferenceOrchestrator) func(context.Context, Req, *core.Workflow) (Req, error)
	valid           func(Req) bool
	build           func(context.Context, Req, *core.Workflow) Prepared
}

var chatPrepareSpec = translatedPrepareSpec[*core.ChatRequest, *PreparedChatRequest]{
	requiredMessage: "chat request is required",
	patchNilMessage: "patched chat request is required",
	selector:        chatRequestSelector,
	patch:           chatRequestPatch,
	thinking:        chatThinkingOverride,
	valid:           validChatRequest,
	build: func(ctx context.Context, req *core.ChatRequest, workflow *core.Workflow) *PreparedChatRequest {
		return &PreparedChatRequest{Context: ctx, Request: req, Workflow: workflow}
	},
}

var responsesPrepareSpec = translatedPrepareSpec[*core.ResponsesRequest, *PreparedResponsesRequest]{
	requiredMessage: "responses request is required",
	patchNilMessage: "patched responses request is required",
	selector:        responsesRequestSelector,
	patch:           responsesRequestPatch,
	thinking:        responsesThinkingOverride,
	valid:           validResponsesRequest,
	build: func(ctx context.Context, req *core.ResponsesRequest, workflow *core.Workflow) *PreparedResponsesRequest {
		return &PreparedResponsesRequest{Context: ctx, Request: req, Workflow: workflow}
	},
}

func prepareTranslated[Req any, Prepared any](
	o *InferenceOrchestrator,
	ctx context.Context,
	req Req,
	meta RequestMeta,
	spec translatedPrepareSpec[Req, Prepared],
) (Prepared, error) {
	var zero Prepared
	if !spec.valid(req) {
		return zero, core.NewInvalidRequestError(spec.requiredMessage, nil)
	}
	ctx = WithAttemptRecorder(ctx)
	model, provider := spec.selector(req)
	ctx, req, workflow, err := prepareTranslatedRequest(o, ctx, req, meta, model, provider, spec.patch(o), spec.valid, spec.patchNilMessage)
	if err != nil {
		return zero, err
	}
	if spec.thinking != nil {
		req, err = spec.thinking(o)(ctx, req, workflow)
		if err != nil {
			return zero, err
		}
	}
	return spec.build(ctx, req, workflow), nil
}

func prepareTranslatedRequest[Req any](
	o *InferenceOrchestrator,
	ctx context.Context,
	req Req,
	meta RequestMeta,
	model,
	provider *string,
	patch func(context.Context, Req) (Req, error),
	valid func(Req) bool,
	patchNilMessage string,
) (context.Context, Req, *core.Workflow, error) {
	ctx = contextWithRequestID(ctx, meta.RequestID)
	workflow, err := o.ensureTranslatedRequestWorkflow(ctx, meta.Workflow, meta.RequestID, meta.Endpoint, model, provider)
	if err != nil {
		var zero Req
		return ctx, zero, nil, err
	}
	ctx = core.WithWorkflow(ctx, workflow)
	if patch != nil {
		req, err = patch(ctx, req)
		if err != nil {
			var zero Req
			return ctx, zero, nil, err
		}
		if valid != nil && !valid(req) {
			var zero Req
			return ctx, zero, nil, core.NewInvalidRequestError(patchNilMessage, nil)
		}
	}
	ctx = o.WithCacheRequestContext(ctx, workflow)
	return ctx, req, workflow, nil
}

func validChatRequest(req *core.ChatRequest) bool {
	return req != nil
}

func validResponsesRequest(req *core.ResponsesRequest) bool {
	return req != nil
}

func chatRequestSelector(req *core.ChatRequest) (*string, *string) {
	return &req.Model, &req.Provider
}

func responsesRequestSelector(req *core.ResponsesRequest) (*string, *string) {
	return &req.Model, &req.Provider
}

func chatRequestPatch(o *InferenceOrchestrator) func(context.Context, *core.ChatRequest) (*core.ChatRequest, error) {
	if o.translatedRequestPatcher == nil {
		return nil
	}
	return o.translatedRequestPatcher.PatchChatRequest
}

func responsesRequestPatch(o *InferenceOrchestrator) func(context.Context, *core.ResponsesRequest) (*core.ResponsesRequest, error) {
	if o.translatedRequestPatcher == nil {
		return nil
	}
	return o.translatedRequestPatcher.PatchResponsesRequest
}

func contextWithRequestID(ctx context.Context, requestID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || strings.TrimSpace(core.GetRequestID(ctx)) == requestID {
		return ctx
	}
	return core.WithRequestID(ctx, requestID)
}

func (o *InferenceOrchestrator) ensureTranslatedRequestWorkflow(
	ctx context.Context,
	current *core.Workflow,
	requestID string,
	endpoint core.EndpointDescriptor,
	model,
	providerHint *string,
) (*core.Workflow, error) {
	if model == nil || providerHint == nil {
		return nil, core.NewInvalidRequestError("model selector targets are required", nil)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	workflow := currentTranslatedWorkflow(current, endpoint)
	var err error
	if workflow == nil {
		resolution, err := ResolveRequestModelWithAuthorizer(ctx, o.provider, o.modelResolver, o.modelAuthorizer, core.NewRequestedModelSelector(*model, *providerHint))
		if err != nil {
			return nil, err
		}
		workflow, err = TranslatedWorkflow(ctx, strings.TrimSpace(requestID), endpoint, resolution, o.workflowPolicyResolver)
		if err != nil {
			return nil, err
		}
		ApplyResolvedSelector(model, providerHint, resolution)
		return workflow, nil
	}

	resolution := workflow.Resolution
	if resolution != nil && o.modelAuthorizer != nil {
		if err := o.modelAuthorizer.ValidateModelAccess(ctx, resolution.ResolvedSelector); err != nil {
			return nil, err
		}
	}
	if resolution == nil {
		resolution, err = ResolveRequestModelWithAuthorizer(ctx, o.provider, o.modelResolver, o.modelAuthorizer, core.NewRequestedModelSelector(*model, *providerHint))
		if err != nil {
			return nil, err
		}
		workflow, err = TranslatedWorkflow(ctx, strings.TrimSpace(requestID), endpoint, resolution, o.workflowPolicyResolver)
		if err != nil {
			return nil, err
		}
	}
	ApplyResolvedSelector(model, providerHint, resolution)
	return workflow, nil
}

func currentTranslatedWorkflow(workflow *core.Workflow, endpoint core.EndpointDescriptor) *core.Workflow {
	if workflow == nil {
		return nil
	}
	if workflow.Mode != core.ExecutionModeTranslated || workflow.Endpoint.Operation != endpoint.Operation {
		return nil
	}
	return workflow
}

// ApplyResolvedSelector updates request model/provider fields to the resolved selector.
func ApplyResolvedSelector(model, providerHint *string, resolution *core.RequestModelResolution) {
	if model == nil || providerHint == nil || resolution == nil {
		return
	}
	*model = resolution.ResolvedSelector.Model
	*providerHint = resolution.ResolvedSelector.Provider
}

// WithCacheRequestContext injects workflow and guardrails cache metadata into ctx.
func (o *InferenceOrchestrator) WithCacheRequestContext(ctx context.Context, workflow *core.Workflow) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if workflow != nil {
		ctx = core.WithWorkflow(ctx, workflow)
	}
	if workflow != nil && workflow.Policy != nil {
		return core.WithGuardrailsHash(ctx, workflow.GuardrailsHash())
	}
	if o.guardrailsHash != "" {
		return core.WithGuardrailsHash(ctx, o.guardrailsHash)
	}
	return ctx
}

// chatThinkingOverride returns a prepare hook that applies the resolved model
// thinking override to a chat request (used by /v1/chat/completions and the
// Responses-via-chat translation).
func chatThinkingOverride(o *InferenceOrchestrator) func(context.Context, *core.ChatRequest, *core.Workflow) (*core.ChatRequest, error) {
	return func(ctx context.Context, req *core.ChatRequest, workflow *core.Workflow) (*core.ChatRequest, error) {
		return applyChatThinkingOverride(o, req, workflow), nil
	}
}

// responsesThinkingOverride returns a prepare hook that applies the resolved
// model thinking override to a Responses API request.
func responsesThinkingOverride(o *InferenceOrchestrator) func(context.Context, *core.ResponsesRequest, *core.Workflow) (*core.ResponsesRequest, error) {
	return func(ctx context.Context, req *core.ResponsesRequest, workflow *core.Workflow) (*core.ResponsesRequest, error) {
		return applyResponsesThinkingOverride(o, req, workflow), nil
	}
}

func applyChatThinkingOverride(o *InferenceOrchestrator, req *core.ChatRequest, workflow *core.Workflow) *core.ChatRequest {
	if req == nil || o.thinkingOverrides == nil || workflow == nil || workflow.Resolution == nil {
		return req
	}
	override, ok := o.thinkingOverrides.Resolve(workflow.ResolvedQualifiedModel())
	if !ok {
		return req
	}
	model := resolvedModelID(workflow)
	cloned := *req
	switch override.Mode {
	case thinkingoverrides.ModeSupplement:
		// Fill only when the client left thinking entirely unset.
		if override.Thinking && cloned.Reasoning == nil && !hasQwenThinkingField(cloned.ExtraFields) {
			cloned.Reasoning = &core.Reasoning{Effort: override.Effort}
			cloned.ExtraFields = setQwenEnableThinking(cloned.ExtraFields, model, true)
		}
	case thinkingoverrides.ModeOverride:
		if !override.Thinking {
			cloned.Reasoning = nil
			cloned.ExtraFields = cloned.ExtraFields.Without("reasoning_effort")
			cloned.ExtraFields = setQwenEnableThinking(cloned.ExtraFields, model, false)
		} else {
			cloned.Reasoning = &core.Reasoning{Effort: override.Effort}
			cloned.ExtraFields = setQwenEnableThinking(cloned.ExtraFields, model, true)
		}
	}
	return &cloned
}

func applyResponsesThinkingOverride(o *InferenceOrchestrator, req *core.ResponsesRequest, workflow *core.Workflow) *core.ResponsesRequest {
	if req == nil || o.thinkingOverrides == nil || workflow == nil || workflow.Resolution == nil {
		return req
	}
	override, ok := o.thinkingOverrides.Resolve(workflow.ResolvedQualifiedModel())
	if !ok {
		return req
	}
	model := resolvedModelID(workflow)
	cloned := *req
	switch override.Mode {
	case thinkingoverrides.ModeSupplement:
		// Fill only when the client left thinking entirely unset. ExtraFields
		// are cloned into the converted chat request for convert-mode providers.
		if override.Thinking && cloned.Reasoning == nil && !hasQwenThinkingField(cloned.ExtraFields) {
			cloned.Reasoning = &core.Reasoning{Effort: override.Effort}
			cloned.ExtraFields = setQwenEnableThinking(cloned.ExtraFields, model, true)
		}
	case thinkingoverrides.ModeOverride:
		if !override.Thinking {
			cloned.Reasoning = nil
			cloned.ExtraFields = setQwenEnableThinking(cloned.ExtraFields, model, false)
		} else {
			cloned.Reasoning = &core.Reasoning{Effort: override.Effort}
			cloned.ExtraFields = setQwenEnableThinking(cloned.ExtraFields, model, true)
		}
	}
	return &cloned
}

// resolvedModelID returns the bare upstream model id from the workflow
// resolution (e.g. "qwen3-8-27b" from "RunInfra/qwen3-8-27b").
func resolvedModelID(workflow *core.Workflow) string {
	if workflow == nil || workflow.Resolution == nil {
		return ""
	}
	return workflow.Resolution.ResolvedSelector.Model
}

// isQwenModel reports whether a model id belongs to the Qwen family. Qwen3
// models control thinking through the enable_thinking extension rather than
// (only) the standard reasoning fields, so the override must set that field
// to actually turn thinking off on OpenAI-compatible upstreams.
func isQwenModel(model string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(model)), "qwen")
}

// hasQwenThinkingField reports whether the request already carries an
// enable_thinking value (top-level or inside chat_template_kwargs), used by
// supplement mode to avoid overriding a client-set value.
func hasQwenThinkingField(fields core.UnknownJSONFields) bool {
	if len(fields.Lookup("enable_thinking")) > 0 {
		return true
	}
	if raw := fields.Lookup("chat_template_kwargs"); len(raw) > 0 {
		return bytes.Contains(bytes.ToLower(raw), []byte("enable_thinking"))
	}
	return false
}

// setQwenEnableThinking merges Qwen's enable_thinking extension into the
// request's unknown-field payload: both a top-level "enable_thinking" member
// (DashScope / OpenAI-compatible style) and the nested
// "chat_template_kwargs.enable_thinking" member documented by Qwen3 model
// cards and used by vLLM/SGLang-hosted deployments. Existing
// chat_template_kwargs members are preserved. Non-Qwen models are untouched.
func setQwenEnableThinking(fields core.UnknownJSONFields, model string, thinking bool) core.UnknownJSONFields {
	if !isQwenModel(model) {
		return fields
	}
	value := json.RawMessage("false")
	if thinking {
		value = json.RawMessage("true")
	}
	additions := map[string]json.RawMessage{"enable_thinking": value}

	if raw := fields.Lookup("chat_template_kwargs"); len(raw) > 0 && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		var kwargs map[string]any
		if err := json.Unmarshal(raw, &kwargs); err == nil {
			kwargs["enable_thinking"] = thinking
			if merged, err := json.Marshal(kwargs); err == nil {
				additions["chat_template_kwargs"] = merged
			}
		}
	} else {
		additions["chat_template_kwargs"] = json.RawMessage(`{"enable_thinking":` + string(value) + `}`)
	}

	merged, err := core.MergeUnknownJSONFields(fields, additions)
	if err != nil {
		return fields
	}
	return merged
}
