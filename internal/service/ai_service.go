package service

import (
	"context"
	"fmt"
)

// AIService defines the interface for AI-powered commit message generation.
// Implementations include Gemini (Google GenAI SDK) and OpenAI-compatible APIs.
type AIService interface {
	// GenerateCommitMessage analyzes diff data and generates a commit message.
	GenerateCommitMessage(ctx context.Context, data *PreCommitData, opts *CommitOptions) (string, error)

	// SelectFilesAndGenerateCommit lets AI select relevant files and generate
	// a commit message in a single request (used in auto mode).
	SelectFilesAndGenerateCommit(ctx context.Context, diff string, opts *SelectFilesAndGenerateCommitOptions) ([]string, string, error)
}

const (
	ProviderGemini = "gemini"
	ProviderOpenAI = "openai"
)

// NewAIService creates an AIService based on the provider name.
func NewAIService(ctx context.Context, provider, apiKey string, baseURL *string) (AIService, error) {
	switch provider {
	case ProviderGemini:
		return NewGeminiAIService(ctx, apiKey, baseURL)
	case ProviderOpenAI:
		return NewOpenAIService(apiKey, baseURL), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s (supported: gemini, openai)", provider)
	}
}
