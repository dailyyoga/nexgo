# cache (Syncable Cache & Redis Client)

Read this when working on the `cache` package — the generic `SyncableCache` or the Redis client wrapper. Referenced from the Reading Index in `CLAUDE.md`.

## SyncableCache

**Core Pattern**: Periodically syncing cache with automatic retry logic and exponential backoff.

**Key Components**:
- `SyncableCache[T]` interface - Generic cache with `Start()`, `Stop()`, `Get()`, `Sync(ctx)`
- `SyncFunc[T]` - User-defined function `func(ctx) (T, error)` to fetch data
- `SyncableCacheConfig` - Configuration for cache name, sync interval, timeout, and retry logic
- Generic implementation supports any data type T

**Architecture Details**:
- Built on Go generics for type-safe caching of any data structure
- Uses `sync.RWMutex` for thread-safe concurrent reads during sync operations
- Background goroutine performs periodic sync on configurable interval
- Exponential backoff retry with configurable max attempts (default: 3)
- Context-aware: respects timeout and cancellation for each sync operation
- Graceful shutdown: `Stop()` can be called multiple times safely with `sync.Once`
- Initial sync: `Start()` performs synchronous initial load before background sync begins
- Error classification: automatically distinguishes retryable (network, timeout) vs non-retryable errors
- Integration: uses `logger.Logger` for structured logging and `routine` package for safe goroutine execution
- **Reference Type Safety**: `Get()` returns a reference to cached data for reference types (slice, map, pointer). Callers must treat returned data as read-only to avoid data races. For value types (int, string, struct without pointers), values are automatically copied

**Important Files**:
- `cache.go` - Core interfaces and `SyncFunc[T]` type
- `syncable_cache.go` - Generic syncable cache implementation with retry logic
- `config.go` - Configuration with validation and defaults
- `errors.go` - Error constructors and predefined errors

**Data Flow** (Sync):
1. User creates cache with `NewSyncableCache(log, config, syncFunc)`
2. `Start()` performs initial sync synchronously (blocks until success or error)
3. Background goroutine starts with ticker for periodic sync
4. On each interval: call `syncFunc(ctx)` with timeout from config
5. On failure: retry with exponential backoff (1s, 2s, 4s, ...)
6. On success: update cache atomically under write lock
7. `Get()` returns current cached value under read lock (non-blocking)
8. `Stop()` cancels context to gracefully shutdown background goroutine

**Configuration**:
```go
cfg := &cache.SyncableCacheConfig{
    Name:         "my-cache",        // For logging/identification (required)
    SyncInterval: 5 * time.Minute,   // Periodic sync interval (default: 5m)
    SyncTimeout:  30 * time.Second,  // Timeout per sync attempt (default: 30s)
    MaxRetries:   3,                  // Max retry attempts on failure (default: 3)
}

cache, err := cache.NewSyncableCache(log, cfg, syncFunc)
```

**Usage**:
```go
import "github.com/dailyyoga/nexgo/cache"

// Define sync function to fetch data
syncFunc := func(ctx context.Context) ([]User, error) {
    // Fetch data from database, API, etc.
    return fetchUsersFromDB(ctx)
}

// Create cache
c, err := cache.NewSyncableCache(log, cfg, syncFunc)
if err != nil {
    return err
}

// Start periodic sync (blocks until initial sync succeeds)
if err := c.Start(); err != nil {
    return err
}
defer c.Stop()

// Get cached data (thread-safe, read-only for reference types)
users := c.Get()

// IMPORTANT: For reference types ([]User here), Get() returns a reference.
// ✅ Safe: Read-only access
for _, user := range users {
    fmt.Println(user.Name)  // OK
}

// ❌ Unsafe: Modifying returned data causes data races
// users[0].Name = "modified"  // DANGER!

// ✅ Safe: Create a copy if you need to modify
usersCopy := make([]User, len(users))
copy(usersCopy, users)
usersCopy[0].Name = "modified"  // OK

// Manually trigger sync if needed
if err := c.Sync(ctx); err != nil {
    log.Error("manual sync failed", zap.Error(err))
}
```

## Redis Client

**Core Pattern**: Thin wrapper around go-redis v9 that embeds `redis.Cmdable` to automatically provide 200+ Redis commands.

**Key Components**:
- `Redis` interface - Embeds `redis.Cmdable` plus custom methods: `Subscribe()`, `PSubscribe()`, `Close()`, `Unwrap()`, `PoolStats()`
- `RedisConfig` struct - Configuration with connection pool settings and timeouts
- `NewRedis(log, cfg)` factory - Creates client, validates config, and tests connection

**Architecture Details**:
- Embeds `*redis.Client` to automatically implement all `redis.Cmdable` methods (200+ commands)
- Thread-safe: `redis.Client` handles all concurrency internally
- Connection test: `Ping()` executed during initialization to verify connectivity
- Custom Subscribe methods wait for subscription confirmation before returning
- Direct access to underlying client via `Unwrap()` for advanced operations (Pipeline, Transaction, etc.)
- Pool statistics available via `PoolStats()`

**Important Files**:
- `cache.go` - `Redis` interface definition (along with `SyncableCache`)
- `config.go` - `RedisConfig` with `Validate()`, `MergeDefaults()`, `Options()`
- `redis.go` - Implementation (`defaultRedis`) with `NewRedis()` factory
- `errors.go` - Redis-specific error constructors

**Configuration**:
```go
cfg := &cache.RedisConfig{
    Addr:            "localhost:6379",  // Redis address (required)
    Username:        "",                 // Username for ACL auth (Redis 6.0+, default: "")
    Password:        "",                 // Auth password (default: "")
    DB:              0,                  // Database number (default: 0)
    PoolSize:        10,                 // Max connections (default: 10)
    MinIdleConns:    5,                  // Min idle connections (default: 5)
    MaxRetries:      3,                  // Max retries (default: 3)
    DialTimeout:     5 * time.Second,   // Dial timeout (default: 5s)
    ReadTimeout:     3 * time.Second,   // Read timeout (default: 3s)
    WriteTimeout:    3 * time.Second,   // Write timeout (default: 3s)
    ConnMaxIdleTime: 5 * time.Minute,   // Max idle time (default: 5m)
    ConnMaxLifetime: 0,                  // Max lifetime (default: 0, no limit)
}
rdb, err := cache.NewRedis(log, cfg)
```

**Usage**:
```go
import (
    "github.com/dailyyoga/nexgo/cache"
    "github.com/redis/go-redis/v9"
)

// Create Redis client
rdb, err := cache.NewRedis(log, cfg)
if err != nil {
    return err
}
defer rdb.Close()

ctx := context.Background()

// String operations
rdb.Set(ctx, "key", "value", time.Hour)
val, err := rdb.Get(ctx, "key").Result()
if err == cache.Nil {
    // Key does not exist
}

// Distributed lock with SetNX
ok, _ := rdb.SetNX(ctx, "lock:resource", "owner", 30*time.Second).Result()

// Hash operations
rdb.HSet(ctx, "user:1", "name", "Alice", "age", "25")
name, _ := rdb.HGet(ctx, "user:1", "name").Result()

// List operations
rdb.LPush(ctx, "queue", "task1", "task2")
task, _ := rdb.RPop(ctx, "queue").Result()

// Sorted set operations
rdb.ZAdd(ctx, "leaderboard", redis.Z{Score: 100, Member: "player1"})
rank, _ := rdb.ZRank(ctx, "leaderboard", "player1").Result()

// Pub/Sub
pubsub, err := rdb.Subscribe(ctx, "channel")
if err != nil {
    return err
}
defer pubsub.Close()

// Pattern subscription
pubsub, err := rdb.PSubscribe(ctx, "events:*")

// Lua script
result, _ := rdb.Eval(ctx, `return ARGV[1]`, nil, "hello").Result()

// Pipeline (via Unwrap)
pipe := rdb.Unwrap().Pipeline()
incr := pipe.Incr(ctx, "counter")
pipe.Expire(ctx, "counter", time.Hour)
pipe.Exec(ctx)
count, _ := incr.Result()

// Transaction (via Unwrap)
rdb.Unwrap().Watch(ctx, func(tx *redis.Tx) error {
    // Transaction logic
    return nil
}, "key")

// Pool statistics
stats := rdb.PoolStats()
log.Info("pool stats", zap.Uint32("total", stats.TotalConns))
```

**Available Commands** (via `redis.Cmdable`):
- **String**: Get, Set, SetNX, SetEX, MGet, MSet, Incr, Decr, Append, etc.
- **Key**: Del, Exists, Expire, TTL, Keys, Scan, Rename, Type, etc.
- **Hash**: HGet, HSet, HGetAll, HDel, HExists, HIncrBy, HScan, etc.
- **List**: LPush, RPush, LPop, RPop, LRange, LLen, LIndex, etc.
- **Set**: SAdd, SRem, SMembers, SIsMember, SCard, SInter, SUnion, etc.
- **Sorted Set**: ZAdd, ZRem, ZRange, ZRank, ZScore, ZCard, ZIncrBy, etc.
- **Script**: Eval, EvalSha, ScriptLoad, ScriptExists, ScriptFlush
- **Pub/Sub**: Publish (Subscribe/PSubscribe via custom methods)
- **Server**: Ping, Info, DBSize, FlushDB, etc.
