package testing

import (
	"context"

	"github.com/hangry-coder/bffx/pkg/api/handlers"
	"github.com/hangry-coder/bffx/pkg/comm"
	"github.com/hangry-coder/bffx/pkg/batteries/analytics"
	"github.com/hangry-coder/bffx/pkg/comm/email"
	"github.com/hangry-coder/bffx/pkg/comm/notifications"
	"github.com/hangry-coder/bffx/pkg/manifest"
	"github.com/hangry-coder/bffx/pkg/storage"
)

type TestEnvironment struct {
	Store    storage.Store
	Registry *manifest.Registry
	Hub      *comm.Hub
	Factory  *Factory
}

func SetupEnvironment(root string) *TestEnvironment {
	// 1. Load Registry
	reg, _ := manifest.LoadAll(root)

	// 2. Setup In-Memory Store
	store := storage.NewMemoryStore()

	// 3. Setup Comm Hub (with mock providers)
	emailMgr := email.NewManager()
	notifyMgr := notifications.NewManager(store)
	tmplMgr := comm.NewTemplateManager(reg)
	hub := comm.NewHub(store, notifyMgr, emailMgr, tmplMgr)

	// 4. Setup Factory
	factory := NewFactory(store, reg)

	return &TestEnvironment{
		Store:    store,
		Registry: reg,
		Hub:      hub,
		Factory:  factory,
	}
}

func (env *TestEnvironment) NewContext(user map[string]any) *handlers.ActionContext {
	return &handlers.ActionContext{
		Store:     env.Store,
		Comm:      env.Hub,
		User:      user,
		Context:   context.Background(),
		Analytics: analytics.NewNoopProvider(),
	}
}
