# cache

A generic Go cache library with periodic synchronization, automatic retry logic, and exponential backoff.

## Features

- **Generic Type Support**: Built with Go generics to support any data type in a type-safe manner
- **Periodic Synchronization**: Automatically refreshes cached data from a configurable source at regular intervals
- **Automatic Retry**: Exponential backoff retry mechanism for transient failures (network issues, timeouts)
- **Thread-Safe**: Concurrent reads are safe during sync operations using `sync.RWMutex`
- **Context-Aware**: Respects context timeout and cancellation for each sync operation
- **Graceful Shutdown**: Safe shutdown with `Stop()` that can be called multiple times
- **Initial Sync**: Blocks on `Start()` until initial data is successfully loaded
- **Error Classification**: Automatically distinguishes retryable vs non-retryable errors

## Installation

```bash
go get github.com/dailyyoga/go-kit/cache
```

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/dailyyoga/go-kit/cache"
    "github.com/dailyyoga/go-kit/logger"
)

type User struct {
    ID   int64
    Name string
}

func main() {
    // Create logger
    log, err := logger.New(nil)
    if err != nil {
        panic(err)
    }
    defer log.Sync()

    // Define sync function to fetch data
    syncFunc := func(ctx context.Context) ([]User, error) {
        // Fetch from database, API, etc.
        return fetchUsersFromDB(ctx)
    }

    // Configure cache
    cfg := &cache.SyncableCacheConfig{
        Name:         "user-cache",
        SyncInterval: 5 * time.Minute,   // Sync every 5 minutes
        SyncTimeout:  30 * time.Second,  // 30s timeout per sync
        MaxRetries:   3,                  // Retry up to 3 times
    }

    // Create cache
    c, err := cache.NewSyncableCache(log, cfg, syncFunc)
    if err != nil {
        log.Fatal(err)
    }

    // Start periodic sync (blocks until initial sync succeeds)
    if err := c.Start(); err != nil {
        log.Fatal("initial sync failed:", err)
    }
    defer c.Stop()

    // Get cached data (thread-safe, non-blocking)
    users := c.Get()
    log.Printf("Loaded %d users", len(users))

    // Manually trigger sync if needed
    if err := c.Sync(context.Background()); err != nil {
        log.Printf("manual sync failed: %v", err)
    }
}

func fetchUsersFromDB(ctx context.Context) ([]User, error) {
    // Your implementation here
    return []User{
        {ID: 1, Name: "Alice"},
        {ID: 2, Name: "Bob"},
    }, nil
}
```

### Advanced Usage with Custom Type

```go
type ConfigData struct {
    Settings   map[string]string
    FeatureFlags map[string]bool
    UpdatedAt  time.Time
}

// Sync function fetches config from remote API
syncFunc := func(ctx context.Context) (*ConfigData, error) {
    resp, err := http.Get("https://api.example.com/config")
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var config ConfigData
    if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
        return nil, err
    }

    config.UpdatedAt = time.Now()
    return &config, nil
}

// Create cache with pointer type
cfg := &cache.SyncableCacheConfig{
    Name:         "app-config",
    SyncInterval: 1 * time.Minute,
    SyncTimeout:  10 * time.Second,
    MaxRetries:   5,
}

configCache, _ := cache.NewSyncableCache(logger, cfg, syncFunc)
configCache.Start()
defer configCache.Stop()

// Get config (returns pointer)
config := configCache.Get()
if config.FeatureFlags["new_feature"] {
    // Use feature
}
```

## Configuration

### Config Structure

```go
type SyncableCacheConfig struct {
    // Name is used for logging purposes to identify the cache
    // Required: must be explicitly set by the user
    Name string

    // SyncInterval is the interval between periodic sync operations
    // Default: 5 * time.Minute
    SyncInterval time.Duration

    // SyncTimeout is the timeout for each sync operation
    // Default: 30 * time.Second
    SyncTimeout time.Duration

    // MaxRetries is the maximum number of retry attempts for failed sync operations
    // Default: 3
    MaxRetries int
}
```

### Default Configuration

```go
// Name field is required, other fields will use defaults
cfg := &cache.SyncableCacheConfig{
    Name: "my-cache",  // Required field
    // SyncInterval, SyncTimeout, MaxRetries will use defaults
}
cache, err := cache.NewSyncableCache(log, cfg, syncFunc)

// Passing nil config will fail validation because Name is required
// cache, err := cache.NewSyncableCache(log, nil, syncFunc)  // Error: Name is required
```

## Architecture

### Core Components

#### SyncableCache Interface

```go
type SyncableCache[T any] interface {
    // Start begins the periodic sync process
    // Performs an initial sync before starting the background goroutine
    // Returns error if initial sync fails
    Start() error

    // Stop gracefully stops the periodic sync process
    // Can be called multiple times safely
    Stop()

    // Get returns the current cached value
    // Safe to call concurrently with sync operations
    Get() T

    // Sync manually triggers a sync operation
    // Returns error if sync fails after all retry attempts
    Sync(ctx context.Context) error
}
```

#### SyncFunc

```go
// SyncFunc is a function that performs the actual sync operation
// Should return the new cache data or an error
// Must respect the context for cancellation and timeout
type SyncFunc[T any] func(ctx context.Context) (T, error)
```

### Retry Logic

The cache implements exponential backoff for retry:

1. **First attempt**: Immediate sync
2. **Second attempt**: 1 second backoff
3. **Third attempt**: 2 seconds backoff
4. **Fourth attempt**: 4 seconds backoff
5. And so on...

**Retryable errors** (automatically detected):
- Context deadline exceeded
- Connection refused/reset
- Broken pipe
- Timeout
- Too many connections
- Temporary failure
- Network unreachable

**Non-retryable errors** will fail immediately without retry.

### Data Flow

1. User calls `NewSyncableCache(log, config, syncFunc)`
2. `Start()` is called:
   - Performs initial sync synchronously (blocks until success)
   - Starts background goroutine for periodic sync
3. Background goroutine:
   - Waits for `SyncInterval` ticker
   - Calls `syncFunc(ctx)` with `SyncTimeout`
   - On failure: retries with exponential backoff up to `MaxRetries`
   - On success: updates cache atomically under write lock
4. `Get()` returns current cached value (uses read lock, non-blocking)
5. `Sync(ctx)` can manually trigger sync at any time
6. `Stop()` cancels context to gracefully shutdown background goroutine

## Error Handling

### Predefined Errors

```go
var (
    // ErrCacheClosed is returned when operations are attempted on a closed cache
    ErrCacheClosed = fmt.Errorf("cache: cache is closed")

    // ErrInvalidConfig is returned when the configuration is invalid
    ErrInvalidConfig = fmt.Errorf("cache: invalid config")
)
```

### Error Constructors

```go
// ErrSync wraps a sync operation error
func ErrSync(err error) error

// ErrInvalidName returns an error for invalid name
func ErrInvalidName(name string) error

// ErrInvalidSyncInterval returns an error for invalid sync interval
func ErrInvalidSyncInterval(interval time.Duration) error

// ErrInvalidSyncTimeout returns an error for invalid sync timeout
func ErrInvalidSyncTimeout(timeout time.Duration) error

// ErrInvalidMaxRetries returns an error for invalid max retries
func ErrInvalidMaxRetries(retries int) error
```

### Error Checking

```go
import "errors"

if err := cache.Start(); err != nil {
    if errors.Is(err, cache.ErrInvalidConfig) {
        // Handle configuration error
    }
    // Check wrapped error
    var syncErr error
    if errors.As(err, &syncErr) {
        // Handle specific sync error
    }
}
```

## Best Practices

### 1. Choose Appropriate Sync Interval

```go
// For frequently changing data
cfg := &cache.SyncableCacheConfig{
    SyncInterval: 30 * time.Second,  // Sync every 30 seconds
}

// For relatively stable data
cfg := &cache.SyncableCacheConfig{
    SyncInterval: 10 * time.Minute,  // Sync every 10 minutes
}
```

### 2. Set Reasonable Timeout

```go
// For fast local operations
cfg := &cache.SyncableCacheConfig{
    SyncTimeout: 5 * time.Second,
}

// For remote API calls
cfg := &cache.SyncableCacheConfig{
    SyncTimeout: 30 * time.Second,
}
```

### 3. Handle Initial Sync Failure

```go
// Start blocks until initial sync succeeds
if err := cache.Start(); err != nil {
    // Use fallback data or fail fast
    log.Fatal("cannot start without initial data:", err)
}
```

### 4. Use Pointer Types for Large Data

```go
// For large data structures, use pointer to avoid copying
type LargeData struct {
    Items []Item  // Could be large
}

syncFunc := func(ctx context.Context) (*LargeData, error) {
    return &LargeData{Items: items}, nil
}

cache, _ := cache.NewSyncableCache[*LargeData](log, cfg, syncFunc)
```

### 5. Graceful Shutdown

```go
// Use defer to ensure Stop is called
cache, err := cache.NewSyncableCache(log, cfg, syncFunc)
if err != nil {
    return err
}
defer cache.Stop()  // Always cleanup

cache.Start()
// ... use cache ...
```

### 6. Manual Sync When Needed

```go
// Trigger immediate sync (e.g., after external update)
if err := cache.Sync(ctx); err != nil {
    log.Error("manual sync failed", zap.Error(err))
    // Continue with stale data or retry
}
```

### 7. Treat Cached Data as Read-Only (Reference Types)

**Critical for Concurrent Safety**: When using reference types (slice, map, pointer, chan), `Get()` returns a reference to the internal cache data, not a deep copy. You **MUST** treat the returned value as read-only.

#### Safe Usage (Read-Only Access)

```go
// ✅ Safe - read-only access
users := cache.Get()  // T = []User
for _, user := range users {
    fmt.Println(user.Name)  // OK
}

// ✅ Safe - passing to read-only function
users := cache.Get()
displayUsers(users)  // OK if displayUsers doesn't modify
```

#### Unsafe Usage (Modifying Returned Data)

```go
// ❌ DANGEROUS - data race!
users := cache.Get()  // T = []User
users[0].Name = "modified"  // Modifies shared cache data

// ❌ DANGEROUS - appending to slice
users := cache.Get()  // T = []User
users = append(users, newUser)  // May modify shared backing array

// ❌ DANGEROUS - modifying map
config := cache.Get()  // T = map[string]string
config["key"] = "value"  // Modifies shared cache data
```

#### Solution: Create a Deep Copy When Modification is Needed

```go
// ✅ Safe - create a copy before modifying
users := cache.Get()  // T = []User
usersCopy := make([]User, len(users))
copy(usersCopy, users)
usersCopy[0].Name = "modified"  // OK - modifying copy

// ✅ Safe - copy map before modifying
config := cache.Get()  // T = map[string]string
configCopy := make(map[string]string, len(config))
for k, v := range config {
    configCopy[k] = v
}
configCopy["key"] = "value"  // OK - modifying copy

// ✅ Safe - copy pointer data
data := cache.Get()  // T = *ConfigData
dataCopy := *data  // Shallow copy
dataCopy.Setting = "modified"  // OK if Setting is value type
```

#### Value Types Don't Have This Issue

```go
// ✅ Automatically safe - value types are copied
count := cache.Get()  // T = int
count++  // OK - modifying local copy

text := cache.Get()  // T = string
text = text + " modified"  // OK - strings are immutable

// ✅ Safe - struct with value fields only
type Config struct {
    Port int
    Host string
}
cfg := cache.Get()  // T = Config (not *Config)
cfg.Port = 8080  // OK - modifying local copy
```

#### Design Patterns for Safe Usage

**Pattern 1: Read-Only Access (Recommended)**
```go
// Design your cache for read-only access
type UserCache struct {
    cache cache.SyncableCache[[]User]
}

func (uc *UserCache) GetUser(id int64) (User, bool) {
    users := uc.cache.Get()
    for _, user := range users {  // Read-only iteration
        if user.ID == id {
            return user, true  // Returns a copy
        }
    }
    return User{}, false
}
```

**Pattern 2: Copy-on-Read**
```go
// Provide a method that returns a copy
type UserCache struct {
    cache cache.SyncableCache[[]User]
}

func (uc *UserCache) GetUsersCopy() []User {
    users := uc.cache.Get()
    usersCopy := make([]User, len(users))
    copy(usersCopy, users)
    return usersCopy
}

// Callers can safely modify the copy
users := userCache.GetUsersCopy()
users[0].Name = "modified"  // OK
```

**Pattern 3: Use Value Types When Possible**
```go
// Instead of []User, use map[int64]User (value type values)
type UserMap map[int64]User

cache, _ := cache.NewSyncableCache[UserMap](log, cfg, syncFunc)

// Still need to copy map, but values are copied automatically
users := cache.Get()
usersCopy := make(UserMap, len(users))
for k, v := range users {
    usersCopy[k] = v  // v is copied (value type)
}
```

## Use Cases

### 1. Configuration Cache

Cache application configuration from remote config service:

```go
type AppConfig struct {
    DatabaseURL string
    APIKeys     map[string]string
    Features    []string
}

syncFunc := func(ctx context.Context) (*AppConfig, error) {
    return fetchConfigFromConsul(ctx)
}

configCache, _ := cache.NewSyncableCache(log, &cache.SyncableCacheConfig{
    Name:         "app-config",
    SyncInterval: 1 * time.Minute,
}, syncFunc)
```

### 2. User Permission Cache

Cache user permissions from database:

```go
type UserPermissions map[int64][]string  // userID -> permissions

syncFunc := func(ctx context.Context) (UserPermissions, error) {
    return db.FetchAllUserPermissions(ctx)
}

permCache, _ := cache.NewSyncableCache(log, &cache.SyncableCacheConfig{
    Name:         "permissions",
    SyncInterval: 5 * time.Minute,
}, syncFunc)

// Check permission
perms := permCache.Get()
if hasPermission(perms[userID], "admin") {
    // Allow action
}
```

### 3. Feature Flag Cache

Cache feature flags from feature management service:

```go
type FeatureFlags map[string]bool

syncFunc := func(ctx context.Context) (FeatureFlags, error) {
    return featureService.GetAllFlags(ctx)
}

flagCache, _ := cache.NewSyncableCache(log, &cache.SyncableCacheConfig{
    Name:         "feature-flags",
    SyncInterval: 30 * time.Second,
}, syncFunc)

// Check flag
if flagCache.Get()["new_checkout_flow"] {
    // Use new flow
}
```

### 4. Reference Data Cache

Cache reference data (countries, currencies, etc.):

```go
type ReferenceData struct {
    Countries  []Country
    Currencies []Currency
    Timezones  []Timezone
}

syncFunc := func(ctx context.Context) (*ReferenceData, error) {
    return loadReferenceData(ctx)
}

refCache, _ := cache.NewSyncableCache(log, &cache.SyncableCacheConfig{
    Name:         "reference-data",
    SyncInterval: 1 * time.Hour,  // Rarely changes
    MaxRetries:   5,
}, syncFunc)
```

## Testing

### Mock SyncFunc for Testing

```go
func TestCache(t *testing.T) {
    log, _ := logger.New(nil)

    // Mock sync function
    callCount := 0
    syncFunc := func(ctx context.Context) ([]string, error) {
        callCount++
        return []string{"item1", "item2"}, nil
    }

    cfg := &cache.SyncableCacheConfig{
        SyncInterval: 100 * time.Millisecond,
        SyncTimeout:  1 * time.Second,
    }

    c, err := cache.NewSyncableCache(log, cfg, syncFunc)
    require.NoError(t, err)

    err = c.Start()
    require.NoError(t, err)
    defer c.Stop()

    // Initial sync should have been called
    assert.Equal(t, 1, callCount)

    // Get cached data
    items := c.Get()
    assert.Equal(t, []string{"item1", "item2"}, items)

    // Wait for periodic sync
    time.Sleep(150 * time.Millisecond)
    assert.GreaterOrEqual(t, callCount, 2)
}
```

### Test Retry Logic

```go
func TestCacheRetry(t *testing.T) {
    log, _ := logger.New(nil)

    attempts := 0
    syncFunc := func(ctx context.Context) (string, error) {
        attempts++
        if attempts < 3 {
            return "", fmt.Errorf("temporary failure")
        }
        return "success", nil
    }

    cfg := &cache.SyncableCacheConfig{
        MaxRetries: 5,
    }

    c, _ := cache.NewSyncableCache(log, cfg, syncFunc)
    err := c.Start()

    // Should succeed after retries
    assert.NoError(t, err)
    assert.GreaterOrEqual(t, attempts, 3)
    assert.Equal(t, "success", c.Get())
}
```

## Thread Safety

The cache is fully thread-safe:

- **Read operations** (`Get()`): Multiple goroutines can read concurrently
- **Write operations** (during sync): Protected by write lock, blocks reads briefly
- **Control operations** (`Start()`, `Stop()`): Safe to call from multiple goroutines

```go
// Safe to use from multiple goroutines
go func() {
    for {
        data := cache.Get()  // Thread-safe read
        processData(data)
        time.Sleep(time.Second)
    }
}()

go func() {
    for {
        cache.Sync(ctx)  // Thread-safe manual sync
        time.Sleep(5 * time.Second)
    }
}()
```

## Performance Considerations

1. **Read Performance**: `Get()` is very fast (only read lock, no allocation)
2. **Write Performance**: Sync blocks reads briefly during data update
3. **Memory**: Cache stores complete copy of data in memory
4. **Sync Frequency**: Balance freshness vs system load

```go
// For read-heavy workloads with large data
// - Use longer sync intervals
// - Use pointer types to avoid copying
// - Consider data size in memory

cfg := &cache.SyncableCacheConfig{
    SyncInterval: 10 * time.Minute,  // Less frequent sync
}

// For write-heavy sources with smaller data
// - Use shorter sync intervals
// - Value types are fine

cfg := &cache.SyncableCacheConfig{
    SyncInterval: 30 * time.Second,  // More frequent sync
}
```

## License

This project is licensed under the MIT License - see the [LICENSE](../LICENSE) file for details.
