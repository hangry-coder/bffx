package cache

import (
	"context"
	"encoding/json"
	"time"
)

// IndexedProvider wraps a cache Provider to maintain an explicit index of
// cache keys by tag (namespace). This is useful for cache systems that do not
// natively support tagging or to have a secondary explicit index.
type IndexedProvider struct {
	Provider
}

// NewIndexedProvider wraps the given provider with index tracking.
func NewIndexedProvider(underlying Provider) Provider {
	if underlying == nil {
		return nil
	}
	return &IndexedProvider{Provider: underlying}
}

// SetWithTags stores a value and registers its key under each tag's index.
func (ip *IndexedProvider) SetWithTags(ctx context.Context, key string, value []byte, tags []string, ttl time.Duration) error {
	err := ip.Provider.SetWithTags(ctx, key, value, tags, ttl)
	if err != nil {
		return err
	}

	for _, tag := range tags {
		indexKey := TagIndexKey(tag)
		var keys []string

		data, err := ip.Provider.Get(ctx, indexKey)
		if err == nil && len(data) > 0 {
			_ = json.Unmarshal(data, &keys)
		}

		exists := false
		for _, k := range keys {
			if k == key {
				exists = true
				break
			}
		}

		if !exists {
			keys = append(keys, key)
			updatedData, err := json.Marshal(keys)
			if err == nil {
				// Store the index with TTL slightly longer than the item's TTL
				_ = ip.Provider.Set(ctx, indexKey, updatedData, ttl+60*time.Second)
			}
		}
	}
	return nil
}

// Set delegates to SetWithTags with no tags.
func (ip *IndexedProvider) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return ip.SetWithTags(ctx, key, value, nil, ttl)
}

// InvalidateTags looks up all keys associated with each tag in the index,
// deletes those keys, and then deletes the index keys. It also delegates to
// the underlying provider.
func (ip *IndexedProvider) InvalidateTags(ctx context.Context, tags []string) (int, error) {
	totalDeleted := 0

	for _, tag := range tags {
		indexKey := TagIndexKey(tag)
		var keys []string

		data, err := ip.Provider.Get(ctx, indexKey)
		if err == nil && len(data) > 0 {
			_ = json.Unmarshal(data, &keys)
		}

		for _, key := range keys {
			if err := ip.Provider.Delete(ctx, key); err == nil {
				totalDeleted++
			}
		}

		_ = ip.Provider.Delete(ctx, indexKey)
	}

	underlyingDeleted, err := ip.Provider.InvalidateTags(ctx, tags)
	if err == nil && underlyingDeleted > totalDeleted {
		totalDeleted = underlyingDeleted
	}

	return totalDeleted, err
}

// Tags returns a Tagger that uses the IndexedProvider to track writes.
func (ip *IndexedProvider) Tags(tags ...string) Tagger {
	return &indexedTagger{
		underlying: ip.Provider.Tags(tags...),
		provider:   ip,
		tags:       tags,
	}
}

type indexedTagger struct {
	underlying Tagger
	provider   *IndexedProvider
	tags       []string
}

func (it *indexedTagger) Get(ctx context.Context, key string) ([]byte, error) {
	return it.underlying.Get(ctx, key)
}

func (it *indexedTagger) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return it.provider.SetWithTags(ctx, key, value, it.tags, ttl)
}
