package storage

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestContextError(t *testing.T) {
	if ContextError(nil) {
		t.Fatal("nil should not be a context error")
	}
	if !ContextError(context.Canceled) {
		t.Fatal("context.Canceled")
	}
	if !ContextError(context.DeadlineExceeded) {
		t.Fatal("context.DeadlineExceeded")
	}
	if !ContextError(fmt.Errorf("wrap: %w", context.Canceled)) {
		t.Fatal("wrapped canceled")
	}
	if ContextError(errors.New("other")) {
		t.Fatal("unrelated error")
	}
	if !ContextError(errors.New("SQLITE INTERRUPTED")) {
		t.Fatal("sqlite interrupted string")
	}
}
