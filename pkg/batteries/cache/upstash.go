package cache

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"github.com/hangry-coder/bffx/pkg/cache"
)

// UpstashProvider implements cache.Provider using Upstash Redis REST API.
type UpstashProvider struct {
	url    string
	token  string
	client *http.Client
}

func NewUpstashProvider(url, token string) *UpstashProvider {
	return &UpstashProvider{
		url:    url,
		token:  token,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (p *UpstashProvider) do(ctx context.Context, args ...interface{}) (interface{}, error) {
	body, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upstash error: %s (status %d)", string(b), resp.StatusCode)
	}

	var result struct {
		Result interface{} `json:"result"`
		Error  string      `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Error != "" {
		return nil, fmt.Errorf("upstash redis error: %s", result.Error)
	}

	return result.Result, nil
}

func (p *UpstashProvider) Get(ctx context.Context, key string) ([]byte, error) {
	res, err := p.do(ctx, "GET", cache.RedisKeyPrefix+key)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	if s, ok := res.(string); ok {
		return []byte(s), nil
	}
	return nil, nil
}

func (p *UpstashProvider) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return p.SetWithTags(ctx, key, value, nil, ttl)
}

func (p *UpstashProvider) SetWithTags(ctx context.Context, key string, value []byte, tags []string, ttl time.Duration) error {
	fullKey := cache.RedisKeyPrefix + key
	
	// Set value
	args := []interface{}{"SET", fullKey, string(value)}
	if ttl > 0 {
		args = append(args, "EX", int(ttl.Seconds()))
	}
	_, err := p.do(ctx, args...)
	if err != nil {
		return err
	}

	// Add tags
	for _, tag := range tags {
		fullTag := cache.RedisTagPrefix + tag
		p.do(ctx, "SADD", fullTag, fullKey)
		p.do(ctx, "EXPIRE", fullTag, int((ttl + time.Hour).Seconds()))
	}

	return nil
}

func (p *UpstashProvider) Delete(ctx context.Context, key string) error {
	_, err := p.do(ctx, "DEL", cache.RedisKeyPrefix+key)
	return err
}

func (p *UpstashProvider) Flush(ctx context.Context) error {
	_, err := p.do(ctx, "FLUSHDB")
	return err
}

func (p *UpstashProvider) Tags(tags ...string) cache.Tagger {
	return &upstashTagger{
		provider: p,
		tags:     tags,
	}
}

func (p *UpstashProvider) InvalidateTags(ctx context.Context, tags []string) (int, error) {
	total := 0
	for _, tag := range tags {
		fullTag := cache.RedisTagPrefix + tag
		res, err := p.do(ctx, "SMEMBERS", fullTag)
		if err != nil {
			continue
		}
		
		if keys, ok := res.([]interface{}); ok && len(keys) > 0 {
			sKeys := make([]interface{}, len(keys)+1)
			sKeys[0] = "UNLINK"
			for i, k := range keys {
				sKeys[i+1] = k
			}
			delRes, err := p.do(ctx, sKeys...)
			if err == nil {
				if n, ok := delRes.(float64); ok {
					total += int(n)
				}
			}
		}
		p.do(ctx, "DEL", fullTag)
	}
	return total, nil
}

func (p *UpstashProvider) InvalidatePattern(ctx context.Context, pattern string) (int, error) {
	// Keys are stored as RedisKeyPrefix + logicalKey (see Get/Set).
	fullPattern := cache.RedisKeyPrefix + pattern
	total := 0
	var cursor interface{} = "0"
	for {
		res, err := p.do(ctx, "SCAN", cursor, "MATCH", fullPattern, "COUNT", 100)
		if err != nil {
			return total, err
		}
		
		parts, ok := res.([]interface{})
		if !ok || len(parts) < 2 {
			break
		}
		
		cursor = parts[0]
		keys, _ := parts[1].([]interface{})
		
		if len(keys) > 0 {
			sKeys := make([]interface{}, len(keys)+1)
			sKeys[0] = "UNLINK"
			for i, k := range keys {
				sKeys[i+1] = k
			}
			delRes, err := p.do(ctx, sKeys...)
			if err == nil {
				if n, ok := delRes.(float64); ok {
					total += int(n)
				}
			}
		}
		
		// Upstash returns cursor as string "0" or float64 0
		if fmt.Sprintf("%v", cursor) == "0" {
			break
		}
	}
	return total, nil
}

func (p *UpstashProvider) Type() string {
	return "upstash"
}

func (p *UpstashProvider) Ping(ctx context.Context) error {
	_, err := p.do(ctx, "PING")
	return err
}

type upstashTagger struct {
	provider *UpstashProvider
	tags     []string
}

func (t *upstashTagger) Get(ctx context.Context, key string) ([]byte, error) {
	return t.provider.Get(ctx, key)
}

func (t *upstashTagger) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return t.provider.SetWithTags(ctx, key, value, t.tags, ttl)
}
