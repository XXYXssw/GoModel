package thinkingoverrides

import (
	"strings"
	"time"

	"github.com/enterpilot/gomodel/internal/modelselectors"
	"github.com/enterpilot/gomodel/internal/validation"
)

// Mode controls how a per-model thinking override is applied to outgoing
// requests. It mirrors the three choices offered in the dashboard settings.
type Mode string

const (
	// ModeNative passes the client's thinking parameters through untouched.
	// It is the default; an override row in this mode is effectively inert.
	ModeNative Mode = "native"
	// ModeSupplement fills in thinking parameters only when the client left
	// them unset; values the client already sent are never removed.
	ModeSupplement Mode = "supplement"
	// ModeOverride forces the thinking parameters regardless of what the
	// client sent: thinking on rewrites the effort, thinking off strips all
	// reasoning fields from the outgoing request.
	ModeOverride Mode = "override"
)

// Thinking effort levels accepted by the override UI.
const (
	EffortLow    = "low"
	EffortMedium = "medium"
	EffortHigh   = "high"
	EffortXHigh  = "xhigh"
)

var validEfforts = map[string]struct{}{
	EffortLow: {}, EffortMedium: {}, EffortHigh: {}, EffortXHigh: {},
}

// ValidEffort reports whether effort is one of the accepted levels.
func ValidEffort(effort string) bool {
	_, ok := validEfforts[strings.TrimSpace(effort)]
	return ok
}

// Override stores one persisted thinking override for a model selector.
type Override struct {
	Selector     string    `json:"selector" bson:"_id"`
	ProviderName string    `json:"provider_name,omitempty" bson:"provider_name,omitempty"`
	Model        string    `json:"model,omitempty" bson:"model,omitempty"`
	Mode         Mode      `json:"mode" bson:"mode"`
	Thinking     bool      `json:"thinking" bson:"thinking"`
	Effort       string    `json:"effort,omitempty" bson:"effort,omitempty"`
	CreatedAt    time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" bson:"updated_at"`
}

// Validate checks the override's mode/thinking/effort combination. Native
// mode carries no constraints because it leaves requests untouched.
func (o Override) Validate() error {
	switch o.Mode {
	case ModeNative, ModeSupplement, ModeOverride:
	default:
		return validation.NewError("mode must be native, supplement, or override", nil)
	}
	if o.Mode != ModeNative && o.Thinking && !ValidEffort(o.Effort) {
		return validation.NewError("effort must be one of low, medium, high, or xhigh when thinking is enabled", nil)
	}
	return nil
}

// ScopeKind reports the normalized selector scope for one override.
func (o Override) ScopeKind() modelselectors.ScopeKind {
	return modelselectors.ScopeKindFor(o.Selector, o.ProviderName, o.Model)
}

func cloneOverride(override Override) Override {
	// Override is a value type; returning it copies all fields. The struct is
	// cloned so callers can mutate the copy without affecting the source.
	return override
}
