package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hangry-coder/bffx/pkg/batteries/vlm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryManager_AppendAndSlidingWindow(t *testing.T) {
	fakeCache := NewFakeCache()
	// Set sliding window limit to 3 messages
	mgr := NewMemoryManager(fakeCache, 3, 10*time.Minute)

	ctx := context.Background()
	sessionID := "session-123"

	// 1. Initially empty
	history, err := mgr.GetHistory(ctx, sessionID)
	require.NoError(t, err)
	assert.Nil(t, history)

	// 2. Append first message
	history, err = mgr.Append(ctx, sessionID, Message{Role: "user", Content: "Hello"})
	require.NoError(t, err)
	assert.Len(t, history, 1)
	assert.Equal(t, "Hello", history[0].Content)

	// 3. Append second and third messages
	_, _ = mgr.Append(ctx, sessionID, Message{Role: "model", Content: "Hi there"})
	history, err = mgr.Append(ctx, sessionID, Message{Role: "user", Content: "How are you?"})
	require.NoError(t, err)
	assert.Len(t, history, 3)

	// 4. Append fourth message - should trigger sliding window drop of the first message
	history, err = mgr.Append(ctx, sessionID, Message{Role: "model", Content: "I am great!"})
	require.NoError(t, err)
	assert.Len(t, history, 3)
	// First message ("Hello") should be dropped. The remaining should be:
	// 1. "Hi there"
	// 2. "How are you?"
	// 3. "I am great!"
	assert.Equal(t, "Hi there", history[0].Content)
	assert.Equal(t, "How are you?", history[1].Content)
	assert.Equal(t, "I am great!", history[2].Content)
}

func TestChatEngine_ChatStream(t *testing.T) {
	fakeCache := NewFakeCache()
	mgr := NewMemoryManager(fakeCache, 10, 10*time.Minute)

	attempts := 0
	fakeVlm1 := &FakeVLM{
		TypeStr: "failing_vlm",
		MockFunc: func(ctx context.Context, image []byte, prompt string) (string, error) {
			attempts++
			return "", errors.New("inference timeout")
		},
	}

	fakeVlm2 := &FakeVLM{
		TypeStr: "working_vlm",
		MockFunc: func(ctx context.Context, image []byte, prompt string) (string, error) {
			attempts++
			// Verify context structure has prompt and history
			assert.Contains(t, prompt, "System: Be helpful")
			assert.Contains(t, prompt, "User: What is BFFX?")
			return "BFFX is an awesome framework", nil
		},
	}

	engine := NewChatEngine([]vlm.Provider{fakeVlm1, fakeVlm2}, mgr)

	ctx := context.Background()
	sessionID := "session-456"

	var streamedTokens []string
	onChunk := func(token string) error {
		streamedTokens = append(streamedTokens, token)
		return nil
	}

	fullResponse, providerUsed, err := engine.ChatStream(ctx, sessionID, "What is BFFX?", "Be helpful", onChunk)
	require.NoError(t, err)

	assert.Equal(t, "BFFX is an awesome framework", fullResponse)
	assert.Equal(t, "working_vlm", providerUsed)
	assert.Equal(t, 2, attempts) // First failed, second succeeded!

	// Verify token chunks are correctly split and streamed
	assert.NotEmpty(t, streamedTokens)
	rebuildResponse := strings.Join(streamedTokens, "")
	assert.Equal(t, "BFFX is an awesome framework", rebuildResponse)

	// Verify conversation turns in cache memory
	history, err := mgr.GetHistory(ctx, sessionID)
	require.NoError(t, err)
	assert.Len(t, history, 2)
	assert.Equal(t, "user", history[0].Role)
	assert.Equal(t, "What is BFFX?", history[0].Content)
	assert.Equal(t, "model", history[1].Role)
	assert.Equal(t, "BFFX is an awesome framework", history[1].Content)
}
