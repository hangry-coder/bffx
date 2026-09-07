package providers

import (
	"github.com/hangry-coder/bffx/pkg/featureflags"
	"github.com/hangry-coder/bffx/pkg/logger"
	"context"
	"errors"
	"sync"
	"time"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
	"github.com/launchdarkly/go-sdk-common/v3/ldlog"
	"github.com/launchdarkly/go-sdk-common/v3/ldvalue"
	ld "github.com/launchdarkly/go-server-sdk/v7"
	"github.com/launchdarkly/go-server-sdk/v7/ldcomponents"
)

// LaunchDarklyProvider implements the FlagProvider interface for LaunchDarkly
type LaunchDarklyProvider struct {
	apiKey string
	client *ld.LDClient
	mu     sync.RWMutex
	initCh chan struct{}
}

func NewLaunchDarklyProvider(config map[string]string) *LaunchDarklyProvider {
	return &LaunchDarklyProvider{
		apiKey: config["apiKey"],
		initCh: make(chan struct{}),
	}
}

func (p *LaunchDarklyProvider) Init(config featureflags.ProviderConfig) error {
	if p.apiKey == "" {
		p.apiKey = config.SDKKey
	}
	if p.apiKey == "" {
		return errors.New("launchdarkly provider requires `features.config.apiKey` to be set in project.yaml")
	}

	// Initialize SDK asynchronously to prevent blocking framework startup
	go func() {
		cfg := ld.Config{
			Logging: ldcomponents.Logging().MinLevel(ldlog.Warn),
		}
		client, err := ld.MakeCustomClient(p.apiKey, cfg, 5*time.Second)
		if err != nil {
			logger.Error("Failed to initialize LaunchDarkly client: %v", err)
			return
		}
		p.mu.Lock()
		p.client = client
		p.mu.Unlock()
		close(p.initCh)
		logger.Info("LaunchDarkly provider initialized")
	}()

	return nil
}

func (p *LaunchDarklyProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}

func (p *LaunchDarklyProvider) Name() string {
	return "launchdarkly"
}

func (p *LaunchDarklyProvider) Subscribe(ctx context.Context, onChange func(key string, value interface{})) error {
	return nil
}

func (p *LaunchDarklyProvider) Evaluate(key string, ctx featureflags.EvalContext) (interface{}, error) {
	client := p.getClient()
	if client == nil {
		return nil, errors.New("launchdarkly client not initialized")
	}

	ldCtx := p.mapToContext(ctx)
	// Variations in v7 return (value, detail, error)
	val, err := client.JSONVariation(key, ldCtx, ldvalue.Null())
	if err != nil {
		return nil, err
	}
	
	// Convert ldvalue to native interface{}
	return p.toInterface(val), nil
}

func (p *LaunchDarklyProvider) EvaluateAll(ctx featureflags.EvalContext) (map[string]featureflags.ResolvedFlag, error) {
	client := p.getClient()
	if client == nil {
		return nil, errors.New("launchdarkly client not initialized")
	}

	ldCtx := p.mapToContext(ctx)
	state := client.AllFlagsState(ldCtx)
	if !state.IsValid() {
		return nil, errors.New("failed to get all flags state from launchdarkly")
	}

	flags := state.ToValuesMap()
	results := make(map[string]featureflags.ResolvedFlag, len(flags))
	for k, v := range flags {
		results[k] = featureflags.ResolvedFlag{
			Key:   k,
			Value: p.toInterface(v),
		}
	}
	return results, nil
}

func (p *LaunchDarklyProvider) getClient() *ld.LDClient {
	p.mu.RLock()
	if p.client != nil {
		defer p.mu.RUnlock()
		return p.client
	}
	p.mu.RUnlock()

	select {
	case <-p.initCh:
		p.mu.RLock()
		defer p.mu.RUnlock()
		return p.client
	case <-time.After(2 * time.Second):
		return nil
	}
}

func (p *LaunchDarklyProvider) mapToContext(ctx featureflags.EvalContext) ldcontext.Context {
	builder := ldcontext.NewBuilder(ctx.UserID)
	if ctx.UserID == "" {
		builder = ldcontext.NewBuilder("anonymous")
		builder.Anonymous(true)
	}

	if ctx.Email != "" {
		builder.SetString("email", ctx.Email)
	}
	if ctx.UserRole != "" {
		builder.SetString("role", ctx.UserRole)
	}
	if ctx.Country != "" {
		builder.SetString("country", ctx.Country)
	}
	if ctx.DeviceID != "" {
		builder.SetString("deviceId", ctx.DeviceID)
	}
	if ctx.Platform != "" {
		builder.SetString("platform", ctx.Platform)
	}
	if ctx.AppVersion != "" {
		builder.SetString("appVersion", ctx.AppVersion)
	}

	for k, v := range ctx.Attributes {
		builder.SetValue(k, ldvalue.CopyArbitraryValue(v))
	}
	for k, v := range ctx.Claims {
		builder.SetValue(k, ldvalue.CopyArbitraryValue(v))
	}

	return builder.Build()
}

func (p *LaunchDarklyProvider) toInterface(v ldvalue.Value) interface{} {
	switch v.Type() {
	case ldvalue.BoolType:
		return v.BoolValue()
	case ldvalue.NumberType:
		return v.Float64Value()
	case ldvalue.StringType:
		return v.StringValue()
	case ldvalue.ArrayType:
		return v.AsValueArray()
	case ldvalue.ObjectType:
		return v.AsValueMap()
	case ldvalue.NullType:
		return nil
	default:
		return nil
	}
}

// Admin Operations (Read-only)

func (p *LaunchDarklyProvider) GetFlag(key string) (*featureflags.FlagDefinition, error) {
	return nil, featureflags.ErrNotSupported
}

func (p *LaunchDarklyProvider) ListFlags() ([]featureflags.FlagDefinition, error) {
	return nil, featureflags.ErrNotSupported
}

func (p *LaunchDarklyProvider) CreateFlag(flag featureflags.FlagDefinition) (*featureflags.FlagDefinition, error) {
	return nil, featureflags.ErrNotSupported
}

func (p *LaunchDarklyProvider) UpdateFlag(key string, flag featureflags.FlagDefinition) (*featureflags.FlagDefinition, error) {
	return nil, featureflags.ErrNotSupported
}

func (p *LaunchDarklyProvider) DeleteFlag(key string) error {
	return featureflags.ErrNotSupported
}
