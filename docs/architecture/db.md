# db (MySQL Database Client)

Read this when working on the `db` MySQL/GORM client — connection-pool tuning, slow-query logging, DSN construction, or health checks. Referenced from the Reading Index in `CLAUDE.md`.

**Core Pattern**: GORM-based MySQL client with connection pool management, structured logging, and slow query detection.

**Key Components**:
- `Database` interface - Provides `DB()`, `Ping(ctx)`, and `Close()`
- `Config` struct - Configuration for connection, pool settings, and logging
- `gormLogger` - Custom GORM logger bridging GORM's logging interface with zap-based structured logging
- `NewMySQL(log, cfg)` factory - Creates and configures database connection with pool settings

**Architecture Details**:
- Built on `gorm.io/gorm` v2 for robust ORM capabilities
- Custom logger integrates GORM's logging with project's `logger.Logger` interface
- Connection pool managed by underlying `database/sql` with configurable parameters
- Slow query detection logs queries exceeding `SlowThreshold` at WARN level
- Prepared statements enabled by default for better performance and security
- Log levels: silent, error, warn, info
- DSN (Data Source Name) automatically constructed from config fields
- Initial `Ping()` test ensures connectivity during initialization

**Important Files**:
- `db.go` - Core `Database` interface
- `mysql.go` - MySQL implementation (`defaultMySQLDatabase`) and factory function
- `logger.go` - Custom GORM logger implementation with zap backend
- `config.go` - Configuration with validation and defaults
- `errors.go` - Error constructors and predefined errors

**Data Flow**:
1. User calls `NewMySQL(log, cfg)` with configuration
2. Configuration validated and merged with defaults
3. GORM DB instance created with custom logger
4. Connection pool settings applied (`MaxOpenConns`, `MaxIdleConns`, etc.)
5. Initial `Ping()` test verifies connectivity
6. User retrieves `*gorm.DB` via `DB()` for GORM operations
7. All SQL operations logged through custom logger with structured fields
8. User calls `Close()` to release connections gracefully

**Configuration**:
```go
cfg := &db.Config{
    Host:            "localhost",
    Port:            3306,
    User:            "root",
    Password:        "password",
    Database:        "myapp",
    MaxOpenConns:    25,              // Max open connections
    MaxIdleConns:    10,              // Max idle connections
    ConnMaxLifetime: 1800 * time.Second,  // 30 minutes
    ConnMaxIdleTime: 600 * time.Second,   // 10 minutes
    LogLevel:        "warn",          // "silent", "error", "warn", "info"
    SlowThreshold:   1 * time.Second, // Slow query threshold
    Charset:         "utf8mb4",
    Loc:             "Local",         // Timezone location
}
database, err := db.NewMySQL(log, cfg)
```

**Usage**:
```go
import (
    "github.com/dailyyoga/nexgo/db"
    "github.com/dailyyoga/nexgo/logger"
)

// Create logger
log, _ := logger.New(nil)
defer log.Sync()

// Create database client
database, err := db.NewMySQL(log, cfg)
if err != nil {
    log.Fatal("database connection failed", zap.Error(err))
}
defer database.Close()

// Health check with context
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()
if err := database.Ping(ctx); err != nil {
    log.Error("database unhealthy", zap.Error(err))
}

// Get GORM instance for operations
gormDB, err := database.DB()
if err != nil {
    log.Error("failed to get db instance", zap.Error(err))
}

// Use GORM for CRUD operations
type User struct {
    ID   int64  `gorm:"primaryKey"`
    Name string `gorm:"size:100"`
    Age  int
}

gormDB.AutoMigrate(&User{})
gormDB.Create(&User{Name: "Alice", Age: 25})

var user User
gormDB.WithContext(ctx).First(&user, "name = ?", "Alice")
```

**Logging Examples**:
The custom logger provides structured logging for all SQL operations:
- **Normal Query (Info)**: `INFO sql trace component=gorm elapsed=2.3ms rows=5 sql="SELECT * FROM users"`
- **Slow Query (Warn)**: `WARN slow sql component=gorm elapsed=1.2s rows=1000 threshold=1s sql="SELECT * FROM users"`
- **SQL Error (Error)**: `ERROR sql error component=gorm elapsed=1.1ms error="Table 'mydb.invalid_table' doesn't exist"`
