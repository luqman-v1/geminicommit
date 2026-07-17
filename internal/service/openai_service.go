package service

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/charmbracelet/huh/spinner"
	"github.com/fatih/color"
	openai "github.com/sashabaranov/go-openai"
)

// OpenAIService implements AIService using the OpenAI-compatible API.
// Works with any OpenAI-compatible endpoint including 9router.
type OpenAIService struct {
	client       *openai.Client
	systemPrompt string
}

// NewOpenAIService creates an OpenAIService with the given API key and optional base URL.
func NewOpenAIService(apiKey string, baseURL *string) *OpenAIService {
	cfg := openai.DefaultConfig(apiKey)
	if baseURL != nil && *baseURL != "" {
		cfg.BaseURL = *baseURL
	}

	return &OpenAIService{
		client:       openai.NewClientWithConfig(cfg),
		systemPrompt: systemPrompt,
	}
}

// GenerateCommitMessage implements AIService interface.
func (o *OpenAIService) GenerateCommitMessage(
	ctx context.Context,
	data *PreCommitData,
	opts *CommitOptions,
) (string, error) {
	messageChan := make(chan string, 1)

	if !*opts.Quiet {
		if err := spinner.New().
			Title(fmt.Sprintf("AI is analyzing your changes. (Model: %s)", *opts.Model)).
			Action(func() {
				o.analyzeToChannel(ctx, data, opts, messageChan)
			}).
			Run(); err != nil {
			return "", err
		}
	} else {
		o.analyzeToChannel(ctx, data, opts, messageChan)
	}

	message := <-messageChan
	if !*opts.Quiet {
		underline := color.New(color.Underline)
		underline.Println("\nChanges analyzed!")
	}

	message = strings.TrimSpace(message)
	if message == "" {
		return "", fmt.Errorf("no commit messages were generated. try again")
	}

	return message, nil
}

func (o *OpenAIService) analyzeToChannel(
	ctx context.Context,
	data *PreCommitData,
	opts *CommitOptions,
	messageChan chan string,
) {
	message, err := o.analyzeChanges(ctx, data, opts)
	if err != nil {
		messageChan <- ""
	} else {
		messageChan <- message
	}
}

func (o *OpenAIService) analyzeChanges(
	ctx context.Context,
	data *PreCommitData,
	opts *CommitOptions,
) (string, error) {
	relatedFilesArray := formatRelatedFiles(data.RelatedFiles)

	userPrompt, err := buildUserPrompt(opts.UserContext, data.Diff, relatedFilesArray, opts.MaxLength, opts.Language, &data.Issue)
	if err != nil {
		return "", err
	}

	enhancedSystemPrompt := o.systemPrompt
	if *opts.Language != "english" {
		enhancedSystemPrompt += fmt.Sprintf("\n\nIMPORTANT: Generate the commit message in %s language.", *opts.Language)
	}
	enhancedSystemPrompt += fmt.Sprintf("\n\nIMPORTANT: Keep the commit message under %d characters.", *opts.MaxLength)
	if data.Issue != "" {
		enhancedSystemPrompt += fmt.Sprintf("\n\nIMPORTANT: Reference issue %s in the commit message.", data.Issue)
	}

	temp := getTemperature(*opts.Model)
	resp, err := o.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       *opts.Model,
		Temperature: temp,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: enhancedSystemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
	})
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty response from model")
	}

	result := strings.TrimSpace(resp.Choices[0].Message.Content)
	result = strings.ReplaceAll(result, "```", "")
	result = strings.TrimSpace(result)

	if result == "" {
		return "", fmt.Errorf("empty response text from model")
	}

	return result, nil
}

// SelectFilesAndGenerateCommit implements AIService interface.
func (o *OpenAIService) SelectFilesAndGenerateCommit(
	ctx context.Context,
	diff string,
	opts *SelectFilesAndGenerateCommitOptions,
) ([]string, string, error) {
	if opts == nil {
		return nil, "", fmt.Errorf("options cannot be nil")
	}
	if opts.ModelName == nil {
		return nil, "", fmt.Errorf("ModelName cannot be nil")
	}
	if opts.MaxLength == nil {
		return nil, "", fmt.Errorf("MaxLength cannot be nil")
	}
	if opts.Language == nil {
		return nil, "", fmt.Errorf("Language cannot be nil")
	}
	if opts.RelatedFiles == nil {
		return nil, "", fmt.Errorf("RelatedFiles cannot be nil")
	}

	relatedFilesArray := formatRelatedFiles(*opts.RelatedFiles)

	contextStr := ""
	if opts.UserContext != nil && *opts.UserContext != "" {
		contextStr = fmt.Sprintf("Use the following context to understand intent: %s\n\n", *opts.UserContext)
	}

	prompt := fmt.Sprintf(
		`%sHere's the code diff:
%s

Neighboring files:
%s

Requirements:
- Maximum commit message length: %d characters
- Language: %s`,
		contextStr,
		diff,
		strings.Join(relatedFilesArray, ", "),
		*opts.MaxLength,
		*opts.Language,
	)

	if opts.Issue != nil && *opts.Issue != "" {
		prompt += fmt.Sprintf("\n- Reference issue: %s", *opts.Issue)
	}

	enhancedSystemPrompt := combinedPrompt
	if *opts.Language != "english" {
		enhancedSystemPrompt += fmt.Sprintf("\n\nIMPORTANT: Generate the commit message in %s language.", *opts.Language)
	}
	enhancedSystemPrompt += fmt.Sprintf("\n\nIMPORTANT: Keep the commit message under %d characters.", *opts.MaxLength)
	if opts.Issue != nil && *opts.Issue != "" {
		enhancedSystemPrompt += fmt.Sprintf("\n\nIMPORTANT: Reference issue %s in the commit message.", *opts.Issue)
	}

	temp := getTemperature(*opts.ModelName)
	resp, err := o.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       *opts.ModelName,
		Temperature: temp,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: enhancedSystemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
	})
	if err != nil {
		return nil, "", err
	}

	if len(resp.Choices) == 0 {
		return nil, "", fmt.Errorf("empty response from model")
	}

	result := strings.TrimSpace(resp.Choices[0].Message.Content)

	// Parse files and commit message using the same logic as Gemini
	return parseCombinedResponse(result)
}

// parseCombinedResponse parses FILES: and COMMIT_MESSAGE: sections from AI response.
// Shared between Gemini and OpenAI implementations.
func parseCombinedResponse(result string) ([]string, string, error) {
	lines := strings.Split(result, "\n")

	// Parse files
	var filesStr string
	foundFilesSection := false
	var filesLines []string

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, "FILES:") {
			foundFilesSection = true
			afterPrefix := strings.TrimSpace(strings.TrimPrefix(trimmedLine, "FILES:"))
			if afterPrefix != "" {
				filesLines = append(filesLines, afterPrefix)
			}
			continue
		}

		if foundFilesSection {
			if strings.HasPrefix(trimmedLine, "COMMIT_MESSAGE:") {
				break
			}
			filesLines = append(filesLines, line)
		}
	}

	if len(filesLines) > 0 {
		filesStr = strings.TrimSpace(strings.Join(filesLines, " "))
	} else {
		if after, ok := strings.CutPrefix(result, "FILES:"); ok {
			if idx := strings.Index(after, "COMMIT_MESSAGE:"); idx != -1 {
				filesStr = strings.TrimSpace(after[:idx])
			} else {
				filesStr = strings.TrimSpace(after)
			}
		}
	}

	if filesStr == "" {
		return nil, "", fmt.Errorf("AI response did not include file list in expected format. Response was: %s", result)
	}

	files := strings.Split(filesStr, ",")
	for i, f := range files {
		f = strings.Trim(f, "` \t\n\r")
		files[i] = strings.TrimSpace(f)
	}

	var validFiles []string
	for _, f := range files {
		if f != "" {
			validFiles = append(validFiles, f)
		}
	}

	// Parse commit message
	var commitMessage string
	foundCommitSection := false
	var commitLines []string

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, "COMMIT_MESSAGE:") {
			foundCommitSection = true
			afterPrefix := strings.TrimSpace(strings.TrimPrefix(trimmedLine, "COMMIT_MESSAGE:"))
			if afterPrefix != "" {
				commitLines = append(commitLines, afterPrefix)
			}
			continue
		}

		if foundCommitSection {
			if strings.HasPrefix(trimmedLine, "FILES:") {
				break
			}
			commitLines = append(commitLines, line)
		}
	}

	if len(commitLines) > 0 {
		commitMessage = strings.Join(commitLines, "\n")
		commitMessage = strings.ReplaceAll(commitMessage, "```", "")
		commitMessage = strings.TrimSpace(commitMessage)
	}

	if commitMessage == "" {
		return nil, "", fmt.Errorf("AI response did not include commit message in expected format. Response was: %s", result)
	}

	return validFiles, commitMessage, nil
}

// buildUserPrompt constructs the user prompt for commit message generation.
// Shared helper used by both Gemini and OpenAI implementations.
func buildUserPrompt(
	userContext *string,
	diff string,
	files []string,
	maxLength *int,
	language *string,
	issue *string,
) (string, error) {
	context := ""
	if userContext != nil && *userContext != "" {
		context = fmt.Sprintf("Use the following context to understand intent: %s", *userContext)
	}

	prompt := fmt.Sprintf(
		`%s

Code diff:
%s

Neighboring files:
%s

Requirements:
- Maximum commit message length: %d characters
- Language: %s`,
		context,
		diff,
		strings.Join(files, ", "),
		*maxLength,
		*language,
	)

	if issue != nil && *issue != "" {
		prompt += fmt.Sprintf("\n- Reference issue: %s", *issue)
	}

	return prompt, nil
}

// getTemperature returns appropriate temperature for the model.
func getTemperature(modelName string) float32 {
	if modelName == Gemini3ProPreview || modelName == Gemini3FlashPreview {
		return 1.0
	}
	return 0.2
}
