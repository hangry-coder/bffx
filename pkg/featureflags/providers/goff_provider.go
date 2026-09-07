package providers

import (
	"github.com/hangry-coder/bffx/pkg/featureflags"
	"context"
	"fmt"
	"path/filepath"
	"time"

	ffclient "github.com/thomaspoignant/go-feature-flag"
	"github.com/thomaspoignant/go-feature-flag/ffcontext"
	"github.com/thomaspoignant/go-feature-flag/retriever"
	"github.com/thomaspoignant/go-feature-flag/retriever/fileretriever"
	"github.com/thomaspoignant/go-feature-flag/retriever/httpretriever"
)

// GoffProvider implements the FlagProvider interface using GO Feature Flag SDK.
// This provider supports local file evaluation and HTTP retrieval without requiring a relay proxy.
type GoffProvider struct {
	api *ffclient.GoFeatureFlag
}

func NewGoffProvider() *GoffProvider {
	return &GoffProvider{}
}

func (p *GoffProvider) Init(config featureflags.ProviderConfig) error {
	var r retriever.Retriever

	// 1. Determine retriever based on config
	if config.URL != "" {
		r = &httpretriever.Retriever{
			URL:     config.URL,
			Timeout: 5 * time.Second,
		}
	} else {
		// Default to local file if no URL
		path := "config/flags.yaml"
		if customPath, ok := config.CustomConfig["flagsFile"].(string); ok {
			path = customPath
		}
		if !filepath.IsAbs(path) && config.Root != "" {
			path = filepath.Join(config.Root, path)
		}
		r = &fileretriever.Retriever{
			Path: path,
		}
	}

	// 2. Initialize GO Feature Flag
	pollingInterval := 10 * time.Second
	if intervalStr, ok := config.CustomConfig["pollingInterval"].(string); ok {
		if d, err := time.ParseDuration(intervalStr); err == nil {
			pollingInterval = d
		}
	}

	options := ffclient.Config{
		PollingInterval: pollingInterval,
		Retriever:       r,
	}

	ff, err := ffclient.New(options)
	if err != nil {
		return fmt.Errorf("init go-feature-flag: %w", err)
	}
	p.api = ff

	return nil
}

func (p *GoffProvider) Close() error {
	if p.api != nil {
		p.api.Close()
	}
	return nil
}

func (p *GoffProvider) Name() string {
	return "goff"
}

func (p *GoffProvider) Evaluate(key string, ctx featureflags.EvalContext) (interface{}, error) {
	if p.api == nil {
		return nil, featureflags.ErrNotFound
	}

	user := p.mapToContext(ctx)
	
	// We use RawVariation to get the value regardless of type.
	// Since the interface expects interface{}, we provide nil as default value
	// because we handle the error ourselves.
	res, err := p.api.RawVariation(key, user, nil)
	if err != nil {
		return nil, err
	}
	
	return res.Value, nil
}

func (p *GoffProvider) EvaluateAll(ctx featureflags.EvalContext) (map[string]featureflags.ResolvedFlag, error) {
	if p.api == nil {
		return nil, featureflags.ErrNotFound
	}

	results := make(map[string]featureflags.ResolvedFlag)
	user := p.mapToContext(ctx)
	
	allFlags := p.api.AllFlagsState(user)
	
	for key, state := range allFlags.GetFlags() {
		results[key] = featureflags.ResolvedFlag{
			Key:   key,
			Value: state.Value,
		}
	}
	
	return results, nil
}

func (p *GoffProvider) Subscribe(ctx context.Context, onChange func(key string, value interface{})) error {
	// go-feature-flag SDK doesn't support real-time subscription in the same way as LD,
	// it uses polling. We can implement a background poller if needed, but for v1 
	// we stick to pull-based evaluation.
	return nil
}

// Admin Operations (Not supported for goff via SDK as it's file-based)

func (p *GoffProvider) CreateFlag(flag featureflags.FlagDefinition) (*featureflags.FlagDefinition, error) {
	return nil, featureflags.ErrNotSupported
}

func (p *GoffProvider) UpdateFlag(key string, flag featureflags.FlagDefinition) (*featureflags.FlagDefinition, error) {
	return nil, featureflags.ErrNotSupported
}

func (p *GoffProvider) DeleteFlag(key string) error {
	return featureflags.ErrNotSupported
}

func (p *GoffProvider) GetFlag(key string) (*featureflags.FlagDefinition, error) {
	return nil, featureflags.ErrNotSupported
}

func (p *GoffProvider) ListFlags() ([]featureflags.FlagDefinition, error) {
	return nil, featureflags.ErrNotSupported
}

// Internal mappers

func (p *GoffProvider) mapToContext(ctx featureflags.EvalContext) ffcontext.Context {
	ffCtx := ffcontext.NewEvaluationContext(ctx.UserID)
	
	if ctx.UserRole != "" {
		ffCtx.AddCustomAttribute("role", ctx.UserRole)
	}
	if ctx.DeviceID != "" {
		ffCtx.AddCustomAttribute("deviceId", ctx.DeviceID)
	}
	if ctx.Email != "" {
		ffCtx.AddCustomAttribute("email", ctx.Email)
	}
	if ctx.Country != "" {
		ffCtx.AddCustomAttribute("country", ctx.Country)
	}
	if ctx.Platform != "" {
		ffCtx.AddCustomAttribute("platform", ctx.Platform)
	}
	if ctx.AppVersion != "" {
		ffCtx.AddCustomAttribute("appVersion", ctx.AppVersion)
	}
	
	for k, v := range ctx.Attributes {
		ffCtx.AddCustomAttribute(k, v)
	}
	for k, v := range ctx.Claims {
		ffCtx.AddCustomAttribute(k, v)
	}
	
	return ffCtx
}
