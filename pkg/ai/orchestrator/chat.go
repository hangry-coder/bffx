package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hangry-coder/bffx/pkg/batteries/vlm"
)

// ChatEngine orchestrates chatbot conversational state and streaming responses.
type ChatEngine struct {
	providers []vlm.Provider
	memory    *MemoryManager
}

// NewChatEngine creates a ChatEngine instance.
func NewChatEngine(providers []vlm.Provider, memory *MemoryManager) *ChatEngine {
	return &ChatEngine{
		providers: providers,
		memory:    memory,
	}
}

// ChatStream executes a conversation turn, persisting history and streaming tokens back via onChunk callback.
func (e *ChatEngine) ChatStream(ctx context.Context, sessionID string, userMsg string, systemPrompt string, onChunk func(token string) error) (string, string, error) {
	if len(e.providers) == 0 {
		return "", "", fmt.Errorf("no VLM/LLM model providers registered")
	}

	// 1. Append user message to history
	_, err := e.memory.Append(ctx, sessionID, Message{
		Role:    "user",
		Content: userMsg,
	})
	if err != nil {
		return "", "", fmt.Errorf("append user memory failed: %w", err)
	}

	// 2. Fetch full conversation window history
	history, err := e.memory.GetHistory(ctx, sessionID)
	if err != nil {
		return "", "", fmt.Errorf("get history failed: %w", err)
	}

	// 3. Compose conversation prompt context
	var promptBuilder strings.Builder
	if systemPrompt != "" {
		promptBuilder.WriteString("System: " + systemPrompt + "\n\n")
	}
	for _, m := range history {
		roleName := "User"
		if m.Role == "model" {
			roleName = "Assistant"
		}
		promptBuilder.WriteString(fmt.Sprintf("%s: %s\n", roleName, m.Content))
	}
	promptBuilder.WriteString("Assistant: ")

	fullPrompt := promptBuilder.String()

	// 4. Model inference with provider fallbacks
	var fullResponse string
	var activeProvider vlm.Provider
	var inferenceErr error

	for _, p := range e.providers {
		activeProvider = p
		// Chatbot is text-only, so pass a nil image byte slice
		fullResponse, inferenceErr = p.Analyze(ctx, nil, fullPrompt)
		if inferenceErr == nil {
			break
		}
	}

	if inferenceErr != nil {
		return "", "", fmt.Errorf("all LLM model providers failed inference: %w", inferenceErr)
	}

	// 5. Append assistant model reply to chat history
	_, err = e.memory.Append(ctx, sessionID, Message{
		Role:    "model",
		Content: fullResponse,
	})
	if err != nil {
		return "", "", fmt.Errorf("append assistant memory failed: %w", err)
	}

	// 6. Execute streaming callback
	// Split by space to simulate streaming tokens cleanly
	tokens := strings.Split(fullResponse, " ")
	for i, t := range tokens {
		select {
		case <-ctx.Done():
			return fullResponse, activeProvider.Type(), ctx.Err()
		default:
			chunk := t
			if i < len(tokens)-1 {
				chunk += " "
			}
			if err := onChunk(chunk); err != nil {
				return fullResponse, activeProvider.Type(), err
			}
			// Tiny sleep to make the stream smooth and readable for UI
			time.Sleep(10 * time.Millisecond)
		}
	}

	return fullResponse, activeProvider.Type(), nil
}
