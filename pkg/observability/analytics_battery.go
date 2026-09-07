package observability

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hangry-coder/bffx/pkg/storage"
)

type LocalBatteryAnalyticsProvider struct {
	store storage.Store
}

func NewLocalBatteryAnalyticsProvider(store storage.Store) *LocalBatteryAnalyticsProvider {
	return &LocalBatteryAnalyticsProvider{store: store}
}

func (p *LocalBatteryAnalyticsProvider) Info(ctx context.Context) ProviderInfo {
	return ProviderInfo{
		Name:        "battery",
		DisplayName: "Built-in (Local)",
		Capabilities: []Capability{
			CapWrite,
			CapReadList,
			CapReadSummary,
		},
		Healthy: true,
	}
}

func (p *LocalBatteryAnalyticsProvider) ListEvents(ctx context.Context) ([]map[string]any, error) {
	if p.store == nil {
		return []map[string]any{}, nil
	}

	records, err := p.store.Query(ctx, "TelemetryEvent").OrderBy("created_at", true).Limit(100).Execute(ctx)
	if err != nil {
		return nil, fmt.Errorf("query telemetry events: %w", err)
	}

	events := make([]map[string]any, 0, len(records))
	for _, r := range records {
		evt := make(map[string]any)
		evt["id"] = r["id"]
		evt["user_id"] = r["user_id"]
		evt["event_type"] = r["event_type"]
		evt["screen_name"] = r["screen_name"]
		evt["action_name"] = r["action_name"]
		evt["timestamp"] = r["created_at"]

		propStr, _ := r["properties"].(string)
		if propStr != "" {
			var props map[string]any
			if err := json.Unmarshal([]byte(propStr), &props); err == nil {
				evt["properties"] = props
			} else {
				evt["properties"] = map[string]any{"raw": propStr}
			}
		} else {
			evt["properties"] = map[string]any{}
		}

		events = append(events, evt)
	}

	return events, nil
}

func (p *LocalBatteryAnalyticsProvider) Record(ctx context.Context, userId, eventType, screenName, actionName string, properties map[string]any) error {
	if p.store == nil {
		return fmt.Errorf("store is nil")
	}

	propBytes, err := json.Marshal(properties)
	if err != nil {
		return fmt.Errorf("marshal properties: %w", err)
	}

	_, err = p.store.Create(ctx, "TelemetryEvent", map[string]any{
		"user_id":     userId,
		"event_type":  eventType,
		"screen_name": screenName,
		"action_name": actionName,
		"properties":  string(propBytes),
	})
	return err
}
