package ai

import (
	"context"
	"fmt"
)

// Provider is the interface all AI backends must implement.
type Provider interface {
	TailorResume(ctx context.Context, req *ResumeTailorRequest) (*ResumeTailorResponse, error)
	GenerateCoverLetter(ctx context.Context, req *CoverLetterRequest) (*CoverLetterResponse, error)
	AnswerFormQuestion(ctx context.Context, req *FormQARequest) (*FormQAResponse, error)
}

// ProviderConfig holds the configuration needed to select and initialise an AI provider.
type ProviderConfig struct {
	// Provider selects which backend to use: "python", "anthropic", "gemini", "openai".
	// Defaults to "python" (the Python FastAPI AI service).
	Provider string

	// Python AI service settings (provider == "python")
	ServiceURL string

	// Anthropic settings (provider == "anthropic")
	AnthropicKey   string
	AnthropicModel string // default: claude-3-haiku-20240307

	// Google Gemini settings (provider == "gemini")
	GeminiAPIKey string
	GeminiModel  string // default: gemini-1.5-flash

	// OpenAI settings (provider == "openai")
	OpenAIAPIKey string
	OpenAIModel  string // default: gpt-4o-mini
}

// NewProvider constructs the appropriate Provider implementation from cfg.
func NewProvider(cfg ProviderConfig) (Provider, error) {
	switch cfg.Provider {
	case "", "python":
		return NewClient(cfg.ServiceURL), nil
	case "anthropic":
		if cfg.AnthropicKey == "" {
			return nil, fmt.Errorf("AI provider \"anthropic\" requires ANTHROPIC_API_KEY")
		}
		model := cfg.AnthropicModel
		if model == "" {
			model = "claude-3-haiku-20240307"
		}
		return NewAnthropicProvider(cfg.AnthropicKey, model), nil
	case "gemini":
		if cfg.GeminiAPIKey == "" {
			return nil, fmt.Errorf("AI provider \"gemini\" requires GEMINI_API_KEY")
		}
		model := cfg.GeminiModel
		if model == "" {
			model = "gemini-1.5-flash"
		}
		return NewGeminiProvider(cfg.GeminiAPIKey, model), nil
	case "openai":
		if cfg.OpenAIAPIKey == "" {
			return nil, fmt.Errorf("AI provider \"openai\" requires OPENAI_API_KEY")
		}
		model := cfg.OpenAIModel
		if model == "" {
			model = "gpt-4o-mini"
		}
		return NewOpenAIProvider(cfg.OpenAIAPIKey, model), nil
	default:
		return nil, fmt.Errorf("unknown AI provider %q; valid values: python, anthropic, gemini, openai", cfg.Provider)
	}
}
