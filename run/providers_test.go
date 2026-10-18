package run

import (
	"slices"
	"testing"

	"github.com/enterpilot/gomodel/config"
	"github.com/enterpilot/gomodel/internal/providers"
)

// TestDefaultProviderFactoryCredentialForms pins the credential form the
// shipped provider registrations produce — the contract the admin API serves
// and the dashboard renders. A registration that stops declaring a field its
// adapter reads would otherwise fail silently: the field disappears from the
// form, and a stored value for it is no longer offered for editing.
func TestDefaultProviderFactoryCredentialForms(t *testing.T) {
	schemas := map[string]providers.CredentialSchema{}
	for _, schema := range defaultProviderFactory(&config.Config{}).CredentialSchemas() {
		schemas[schema.Type] = schema
	}

	tests := []struct {
		providerType string
		defaultURL   string
		fields       []string // exact, in display order
		required     []string
		absent       []string
		options      map[string][]string
	}{
		{
			// The plain shape every API-key provider derives.
			providerType: "openai",
			defaultURL:   "https://api.openai.com/v1",
			fields:       []string{"api_keys", "base_url", "responses_mode", "proxy", "session_sticky_keys", "models", "headers"},
			required:     []string{"api_keys"},
			absent:       []string{"api_version", "vertex_project"},
		},
		{
			// Newly registered API-key providers use the same schema feed as
			// every other type, so the dashboard can offer them immediately.
			providerType: "openrouter",
			defaultURL:   "https://openrouter.ai/api/v1",
			fields:       []string{"api_keys", "base_url", "responses_mode", "proxy", "session_sticky_keys", "models", "headers"},
			required:     []string{"api_keys"},
		},
		{
			providerType: "kilo",
			defaultURL:   "https://api.kilo.ai/api/gateway",
			fields:       []string{"api_keys", "base_url", "responses_mode", "proxy", "session_sticky_keys", "models", "headers"},
			required:     []string{"api_keys"},
		},
		{
			// One adapter, two backends: an AI Studio key, or Google
			// credentials once it points at Vertex.
			providerType: "gemini",
			fields: []string{
				"api_keys", "backend", "base_url", "proxy", "api_mode", "auth_type",
				"vertex_project", "vertex_location", "service_account_json",
				"service_account_file", "service_account_json_base64", "gcp_scope", "session_sticky_keys", "models", "headers",
			},
			required: nil,
			options: map[string][]string{
				"backend":   {"aistudio", "vertex"},
				"api_mode":  {"native", "openai_compatible"},
				"auth_type": {"api_key", "gcp_adc", "gcp_service_account"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.providerType, func(t *testing.T) {
			schema, ok := schemas[tt.providerType]
			if !ok {
				t.Fatalf("no credential schema for provider type %q", tt.providerType)
			}
			if tt.defaultURL != "" && schema.DefaultBaseURL != tt.defaultURL {
				t.Errorf("DefaultBaseURL = %q, want %q", schema.DefaultBaseURL, tt.defaultURL)
			}

			var names []string
			for _, field := range schema.Fields {
				names = append(names, field.Name)
			}
			if !slices.Equal(names, tt.fields) {
				t.Errorf("fields = %v, want %v", names, tt.fields)
			}
			var requiredNames []string
			for _, field := range schema.Fields {
				if field.Required {
					requiredNames = append(requiredNames, field.Name)
				}
			}
			if !slices.Equal(requiredNames, tt.required) {
				t.Errorf("required = %v, want %v", requiredNames, tt.required)
			}
			for _, name := range tt.absent {
				if schema.Accepts(name) {
					t.Errorf("Accepts(%s) = true, want false for provider type %q", name, tt.providerType)
				}
			}
			for name, want := range tt.options {
				field, _ := schema.Field(name)
				if !slices.Equal(field.Options, want) {
					t.Errorf("%s.Options = %v, want %v", name, field.Options, want)
				}
			}
			// Every field a form renders must be one the API accepts, or the
			// value would be dropped on save.
			for _, name := range names {
				if !slices.Contains(credentialPayloadFields, name) {
					t.Errorf("field %q is not part of the upsert payload", name)
				}
			}
		})
	}
}

// credentialPayloadFields mirrors the upsert request's credential keys.
var credentialPayloadFields = []string{
	"api_keys", "session_sticky_keys", "base_url", "api_version", "backend", "auth_type", "api_mode", "responses_mode", "proxy", "headers",
	"vertex_project", "vertex_location", "service_account_file", "service_account_json",
	"service_account_json_base64", "gcp_scope", "models",
}

func TestDefaultProviderFactoryRegistersAllProviderTypes(t *testing.T) {
	expected := []string{
		"gemini", "kilo", "openai", "openrouter",
	}

	for _, metricsEnabled := range []bool{false, true} {
		cfg := &config.Config{}
		cfg.Metrics.Enabled = metricsEnabled

		factory := defaultProviderFactory(cfg)
		got := factory.RegisteredTypes()
		slices.Sort(got)

		if !slices.Equal(got, expected) {
			t.Errorf("metrics=%v: registered types = %v, want %v", metricsEnabled, got, expected)
		}

		// CredentialSchemas is the source for
		// GET /admin/provider-credentials/types, which drives the dashboard's
		// Add Provider selector. Keep it in exact lockstep with construction.
		dashboardTypes := make([]string, 0, len(expected))
		for _, schema := range factory.CredentialSchemas() {
			dashboardTypes = append(dashboardTypes, schema.Type)
		}
		if !slices.Equal(dashboardTypes, expected) {
			t.Errorf("metrics=%v: dashboard provider types = %v, want %v", metricsEnabled, dashboardTypes, expected)
		}
	}
}
