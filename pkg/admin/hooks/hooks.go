package hooks

import (
	"context"
	"sync"

	"github.com/hangry-coder/bffx/pkg/audit"
	"github.com/hangry-coder/bffx/pkg/storage"
)

type Context struct {
	Context context.Context
	Store   storage.Store
	Auditor *audit.Auditor
	ActorID string
}

type AdminHookFunc func(ctx *Context, resource string, ids []string, payload map[string]any) (map[string]any, error)

var (
	mu    sync.RWMutex
	hooks = make(map[string]AdminHookFunc)
)

func Register(name string, hook AdminHookFunc) {
	mu.Lock()
	defer mu.Unlock()
	hooks[name] = hook
}

func Get(name string) (AdminHookFunc, bool) {
	mu.RLock()
	defer mu.RUnlock()
	h, ok := hooks[name]
	return h, ok
}
