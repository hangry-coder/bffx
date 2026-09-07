---
title: Writing a Pluggable Battery
description: Deep dive cookbook on extending BFFX with custom backend batteries satisfying core framework interfaces.
category: extending
---

# 🛠️ Writing a Pluggable Battery

BFFX uses a clean **Pluggable Batteries** pattern for **infrastructure** provider swaps. Core application orchestrations (manifests, routers, hooks) stay decoupled from external backends. Modules like Auth, Cache, Relational Storage, Object Storage, and Feature Flag **adapters** are wired through `pkg/batteries/registry.go`.

**Before you start:** confirm your extension belongs in batteries and not in addons or core. Use the [extension taxonomy](../core-concepts/extension_taxonomy.md) decision checklist. Domain features (billing, catalog lookup, game services) belong in `pkg/addons/`, not batteries.

---

## 1. Core Architecture Pattern

Every battery satisfies a standard Go interface. For example, the `Cache` battery interface is defined under `pkg/batteries/cache` as:

```go
package cache

import "context"

type Engine interface {
    Get(ctx context.Context, key string) (string, error)
    Set(ctx context.Context, key string, val string, ttlSeconds int) error
    Delete(ctx context.Context, key string) error
    Ping(ctx context.Context) error
}
```

By keeping providers bound strictly to Go interfaces, we can swap execution engines (e.g. from local memory caching to Upstash or Redis) via `project.yaml` changes, without modifying a single line of business logic in hooks.

---

## 2. Writing a Custom Battery (Memcached Example)

Suppose you want to write a custom caching battery powered by **Memcached** instead of standard Redis. 

### Step 1: Create the Battery Package
Create a new file under `pkg/batteries/cache/memcached.go`:

```go
package cache

import (
    "context"
    "fmt"
    "time"

    "github.com/bradfitz/gomemcache/memcache"
)

type MemcachedBattery struct {
    client *memcache.Client
}

// Ensure MemcachedBattery fully implements the Engine interface at compile time
var _ Engine = (*MemcachedBattery)(nil)

func NewMemcached(servers ...string) *MemcachedBattery {
    return &MemcachedBattery{
        client: memcache.New(servers...),
    }
}

func (m *MemcachedBattery) Get(ctx context.Context, key string) (string, error) {
    item, err := m.client.Get(key)
    if err != nil {
        if err == memcache.ErrCacheMiss {
            return "", nil // Return empty without error for cache misses
        }
        return "", err
    }
    return string(item.Value), nil
}

func (m *MemcachedBattery) Set(ctx context.Context, key string, val string, ttlSeconds int) error {
    return m.client.Set(&memcache.Item{
        Key:        key,
        Value:      []byte(val),
        Expiration: int32(ttlSeconds),
    })
}

func (m *MemcachedBattery) Delete(ctx context.Context, key string) error {
    return m.client.Delete(key)
}

func (m *MemcachedBattery) Ping(ctx context.Context) error {
    // Memcached does not have a native direct ping command; execute a test set/get
    err := m.client.Set(&memcache.Item{
        Key:        "__bffx_ping__",
        Value:      []byte("1"),
        Expiration: 1,
    })
    if err != nil {
        return fmt.Errorf("memcached server ping failed: %w", err)
    }
    return nil
}
```

---

## 3. Registering the Custom Battery

To make the orchestrator aware of your new battery provider, wire it into the initialization routines in `pkg/batteries/registry.go`.

### Step 1: Add Configuration Mapping
Update your global `project.yaml` to declare the custom battery option under `spec.batteries`:

```yaml
# bffx/project.yaml
apiVersion: bffx.io/v1alpha1
kind: Project
metadata:
  name: demo-project
spec:
  batteries:
    cache: memcached  # Mounts our new custom memcached engine
```

### Step 2: Wire the Factory Routine
Locate the initialization case switch in `pkg/batteries/registry.go` (normally under `InitializeCache`) and append your provider setup:

```go
// Inside pkg/batteries/registry.go
func ResolveCacheEngine(mode string, config *ProjectSpec) (cache.Engine, error) {
    switch strings.ToLower(mode) {
    case "memory":
        return cache.NewMemory(), nil
    case "redis":
        return cache.NewRedis(os.Getenv("BFFX_REDIS_URL")), nil
    case "memcached":
        // Extract Memcached cluster endpoints from environment variables
        servers := os.Getenv("BFFX_MEMCACHED_SERVERS")
        if servers == "" {
            servers = "127.0.0.1:11211"
        }
        return cache.NewMemcached(strings.Split(servers, ",")...), nil
    default:
        return nil, fmt.Errorf("unknown cache battery provider: %s", mode)
    }
}
```

---

## 4. Diagnostics & Testing

### 4.1 Update Doctor Commands
To ensure `bffx doctor` validates the health status of your Memcached server on boot, add a specific case logic in the diagnostics runner under `pkg/doctor/doctor.go`:

```go
// Inside checkBatteries in pkg/doctor/doctor.go
case "Cache":
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    if err := bat.Cache.Ping(ctx); err != nil {
        status = "fail"
        msg = fmt.Sprintf("Memcached cache ping failed: %v. Confirm servers are running.", err)
    } else {
        msg = "Memcached cache cluster is reachable and healthy"
    }
```

### 4.2 Write Unit Tests
Validate the battery's implementation contract under `pkg/batteries/cache/memcached_test.go`:

```go
package cache

import (
    "context"
    "testing"
)

func TestMemcachedBattery(t *testing.T) {
    // Skip test if no local Memcached server is running to prevent CI failures
    engine := NewMemcached("127.0.0.1:11211")
    ctx := context.Background()
    
    if err := engine.Ping(ctx); err != nil {
        t.Skip("Memcached is not running locally. Skipping integration check.")
    }

    // Validate Set/Get lifecycle contracts
    err := engine.Set(ctx, "test_key", "hello_world", 10)
    if err != nil {
        t.Fatalf("Failed to Set value: %v", err)
    }

    val, err := engine.Get(ctx, "test_key")
    if err != nil || val != "hello_world" {
        t.Fatalf("Get returned unexpected value: %s, err: %v", val, err)
    }
}
```
