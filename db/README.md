# db

A SQL database client wrapper built on GORM with multi-driver support, connection pool management, structured logging, and configurable query settings.

## Features

- **Multi-Driver**: First-class support for **MySQL** and **PostgreSQL** with a shared abstraction (`BaseConfig` + `Connector` interface) so adding new drivers is trivial
- **GORM Integration**: Built on top of GORM v2 for robust ORM capabilities
- **Connection Pool Management**: Configurable max connections, idle connections, and connection lifetimes
- **Unified Logging**: Integrates with the project's logger package using zap for structured SQL logging
- **Slow Query Detection**: Automatic detection and logging of slow queries with configurable thresholds
- **SQL Tracing**: Detailed SQL execution logging with elapsed time, rows affected, and error details
- **Prepared Statements**: Enabled by default for better performance and security
- **Configuration Validation**: Automatic validation with sensible defaults

## Installation

```bash
go get github.com/dailyyoga/nexgo/db
```

## Quick Start

### MySQL

```go
package main

import (
    "github.com/dailyyoga/nexgo/db"
    "github.com/dailyyoga/nexgo/logger"
)

type User struct {
    ID   int64  `gorm:"primaryKey"`
    Name string `gorm:"size:100"`
    Age  int
}

func main() {
    log, _ := logger.New(nil)
    defer log.Sync()

    cfg := &db.Config{
        BaseConfig: db.BaseConfig{
            Host:     "localhost",
            Port:     3306,
            User:     "root",
            Password: "password",
            Database: "myapp",
        },
    }

    database, err := db.NewMySQL(log, cfg)
    if err != nil {
        panic(err)
    }
    defer database.Close()

    gormDB, _ := database.DB()
    gormDB.AutoMigrate(&User{})
    gormDB.Create(&User{Name: "Alice", Age: 25})
}
```

### PostgreSQL

```go
package main

import (
    "github.com/dailyyoga/nexgo/db"
    "github.com/dailyyoga/nexgo/logger"
)

func main() {
    log, _ := logger.New(nil)
    defer log.Sync()

    cfg := &db.PostgresConfig{
        BaseConfig: db.BaseConfig{
            Host:     "localhost",
            Port:     5432,
            User:     "postgres",
            Password: "password",
            Database: "myapp",
        },
        SSLMode:         "disable",
        TimeZone:        "UTC",
        SearchPath:      "public",
        ApplicationName: "my-service",
    }

    database, err := db.NewPostgres(log, cfg)
    if err != nil {
        panic(err)
    }
    defer database.Close()

    gormDB, _ := database.DB()
    // Use the same GORM API regardless of driver
}
```

### Working With Context

```go
ctx := context.Background()

if err := database.Ping(ctx); err != nil {
    log.Fatal("database not reachable:", err)
}

gormDB, _ := database.DB()
var users []User
gormDB.WithContext(ctx).Where("age > ?", 18).Find(&users)
```

## Configuration

Configuration is split into a shared `BaseConfig` (connection identity, pool, logging) and driver-specific structs that embed it. Field access via promotion works as if the fields were declared inline (`cfg.Host`, `cfg.MaxOpenConns`, ...), and YAML/mapstructure payloads stay flat thanks to `mapstructure:",squash"`.

### BaseConfig (shared)

```go
type BaseConfig struct {
    Host string             // required
    Port int                // driver default applied via MergeDefaults
    User string             // required
    Password string         // required
    Database string         // required

    MaxOpenConns    int           // default 25
    MaxIdleConns    int           // default 10
    ConnMaxLifetime time.Duration // default 30m
    ConnMaxIdleTime time.Duration // default 10m

    LogLevel      string        // silent | error | warn | info; default warn
    SlowThreshold time.Duration // default 1s
}
```

### MySQL — `Config`

```go
type Config struct {
    BaseConfig `mapstructure:",squash"`
    Charset string // default "utf8mb4"
    Loc     string // default "Local"
}
```

Default `Port`: `3306`.

### PostgreSQL — `PostgresConfig`

```go
type PostgresConfig struct {
    BaseConfig `mapstructure:",squash"`
    SSLMode         string // disable | require | verify-ca | verify-full; default "disable"
    TimeZone        string // IANA tz name; default "UTC"
    SearchPath      string // PG search_path; default "public"
    ApplicationName string // populates pg_stat_activity.application_name (optional)
}
```

Default `Port`: `5432`.

### YAML Loading

Because driver-specific configs embed `BaseConfig` with `,squash`, YAML keys remain flat:

```yaml
# MySQL
mysql:
  host: localhost
  port: 3306
  user: root
  password: secret
  database: myapp
  max_open_conns: 50
  charset: utf8mb4
  loc: Local

# PostgreSQL
postgres:
  host: localhost
  port: 5432
  user: postgres
  password: secret
  database: myapp
  max_open_conns: 50
  ssl_mode: require
  time_zone: UTC
  search_path: public
  application_name: my-service
```

### Defaults & Custom Tuning

```go
// Minimum required fields — every other field falls back to defaults via MergeDefaults
cfg := &db.PostgresConfig{
    BaseConfig: db.BaseConfig{
        Host: "localhost", User: "postgres", Password: "pw", Database: "myapp",
    },
}
database, _ := db.NewPostgres(log, cfg)

// Heavily tuned MySQL
cfg := &db.Config{
    BaseConfig: db.BaseConfig{
        Host: "db.example.com", Port: 3306,
        User: "app", Password: "secret", Database: "prod",
        MaxOpenConns:    50,
        MaxIdleConns:    20,
        ConnMaxLifetime: time.Hour,
        ConnMaxIdleTime: 15 * time.Minute,
        LogLevel:        "info",
        SlowThreshold:   500 * time.Millisecond,
    },
    Charset: "utf8mb4",
    Loc:     "UTC",
}
```

## Architecture

### Database Interface

The driver-neutral handle returned by both `NewMySQL` and `NewPostgres`:

```go
type Database interface {
    DB() (*gorm.DB, error)         // underlying GORM DB
    Ping(ctx context.Context) error
    Close() error
}
```

### Connector Interface

Driver-specific configs implement `Connector`. The shared connection helper (`openGorm`) takes any `Connector`, so adding a new driver is mostly a matter of writing a new `*Config` type and a thin `New<Driver>` wrapper.

```go
type Connector interface {
    Validate() error
    DSN() string
    Base() *BaseConfig
    Dialector() gorm.Dialector
}
```

Both `*Config` (MySQL) and `*PostgresConfig` satisfy this interface.

### Custom GORM Logger

Bridges GORM's logging interface with the project's zap-based logger:

- **Structured Logging**: All SQL logs include structured fields (elapsed time, rows affected, SQL statement)
- **Slow Query Detection**: Queries exceeding `SlowThreshold` are logged at WARN level
- **Error Logging**: SQL errors are logged at ERROR level with full context
- **Log Levels**: Supports `silent`, `error`, `warn`, and `info`

### Connection Pool

The client configures the underlying `database/sql` pool with:

- **MaxOpenConns**: Total connection ceiling
- **MaxIdleConns**: Idle connections kept warm for reuse
- **ConnMaxLifetime**: Closes long-lived connections to handle DB restarts
- **ConnMaxIdleTime**: Closes idle connections to free resources

### Data Flow

1. Caller invokes `NewMySQL(log, cfg)` / `NewPostgres(log, cfg)`
2. `MergeDefaults()` fills zero-valued fields, `Validate()` runs shared + driver-specific checks
3. The shared `openGorm(log, cfg)` helper:
   - Builds the GORM-side `Dialector` via `cfg.Dialector()`
   - Constructs `*gorm.DB` with the custom logger and `PrepareStmt: true`
   - Applies pool settings from `cfg.Base()`
   - Issues an initial `Ping()` to confirm connectivity
4. Caller retrieves `*gorm.DB` via `database.DB()` and runs ORM operations
5. `Close()` releases connections gracefully on shutdown

### Adding a New Driver

To add a new SQL driver (e.g., SQLite, ClickHouse via gorm):

1. Define `<Driver>Config` embedding `BaseConfig` with any driver-specific fields
2. Implement `Validate()`, `DSN()`, `Base()`, `Dialector()` (the `Connector` contract)
3. Provide a `New<Driver>(log, *<Driver>Config) (Database, error)` wrapper that calls `cfg.MergeDefaults()`, `cfg.Validate()`, then `openGorm(log, cfg)`

The pool, logger, and connection lifecycle are inherited automatically.

## Error Handling

### Predefined Errors

```go
var (
    // ErrConnectionNotEstablished is returned when DB() is called before connection
    ErrConnectionNotEstablished = fmt.Errorf("db: database connection not established")
)
```

### Error Constructors

```go
// ErrInvalidConfig returns an error for invalid configuration
func ErrInvalidConfig(msg string) error

// ErrConnection wraps connection-related errors
func ErrConnection(err error) error
```

### Error Checking

```go
import "errors"

database, err := db.NewPostgres(log, cfg)
if err != nil {
    if strings.Contains(err.Error(), "invalid config") {
        log.Error("configuration error:", err)
    }
    if strings.Contains(err.Error(), "connection failed") {
        log.Error("cannot connect to database:", err)
    }
}

gormDB, err := database.DB()
if err != nil {
    if errors.Is(err, db.ErrConnectionNotEstablished) {
        log.Error("database not ready")
    }
}
```

## GORM Usage Examples

The GORM API is identical regardless of driver — these examples apply to both MySQL and PostgreSQL.

### Basic CRUD

```go
gormDB, _ := database.DB()

// Create
user := User{Name: "Bob", Age: 30}
gormDB.Create(&user)

// Read
var foundUser User
gormDB.First(&foundUser, user.ID)
gormDB.Where("name = ?", "Bob").First(&foundUser)

// Update
gormDB.Model(&user).Update("Age", 31)
gormDB.Model(&user).Updates(User{Name: "Bobby", Age: 31})

// Delete
gormDB.Delete(&user)
```

### Advanced Queries

```go
// Transactions
err := gormDB.Transaction(func(tx *gorm.DB) error {
    if err := tx.Create(&user1).Error; err != nil {
        return err
    }
    return tx.Create(&user2).Error
})

// Associations
type Order struct {
    ID     int64
    UserID int64
    User   User
}
var orders []Order
gormDB.Preload("User").Find(&orders)

// Joins
var results []struct {
    UserName string
    OrderID  int64
}
gormDB.Table("orders").
    Select("users.name as user_name, orders.id as order_id").
    Joins("left join users on users.id = orders.user_id").
    Scan(&results)

// Raw SQL
var count int64
gormDB.Raw("SELECT COUNT(*) FROM users WHERE age > ?", 18).Scan(&count)
```

## Logging Examples

### Setting Levels

```go
cfg.LogLevel = "silent"  // No SQL logs
cfg.LogLevel = "error"   // Only SQL errors
cfg.LogLevel = "warn"    // Errors + slow queries
cfg.LogLevel = "info"    // All SQL statements
```

### Sample Output

**Normal Query (info level)**:
```
INFO  sql trace  component=gorm elapsed=2.3ms rows=5 sql="SELECT * FROM users WHERE age > 18"
```

**Slow Query (warn level)**:
```
WARN  slow sql  component=gorm elapsed=1.2s rows=1000 sql="SELECT * FROM users" threshold=1s
```

**SQL Error (error level)**:
```
ERROR sql error  component=gorm elapsed=1.1ms rows=0 sql="SELECT * FROM invalid_table" error="..."
```

## Best Practices

### 1. Connection Pool Sizing

```go
// Low-traffic
MaxOpenConns: 10, MaxIdleConns: 5

// High-traffic
MaxOpenConns: 100, MaxIdleConns: 25

// Rule of thumb: MaxIdleConns is 20–50% of MaxOpenConns
```

### 2. Always Use Context

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

gormDB, _ := database.DB()
var users []User
result := gormDB.WithContext(ctx).Find(&users)
if errors.Is(result.Error, context.DeadlineExceeded) {
    log.Error("query timeout")
}
```

### 3. Graceful Shutdown

```go
defer database.Close()
```

### 4. Health Checks

```go
func healthCheck(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
    defer cancel()
    if err := database.Ping(ctx); err != nil {
        w.WriteHeader(http.StatusServiceUnavailable)
        return
    }
    w.WriteHeader(http.StatusOK)
}
```

### 5. Slow Query Tuning

```go
// Aggressive in development
SlowThreshold: 100 * time.Millisecond

// Normal in production
SlowThreshold: 1 * time.Second
```

### 6. PostgreSQL: Pick the Right SSLMode

| SSLMode       | When to use                                       |
|---------------|---------------------------------------------------|
| `disable`     | Local development, trusted private network        |
| `require`     | Encrypted transport without certificate validation |
| `verify-ca`   | Encrypted + verify CA chain                        |
| `verify-full` | Production: encrypted + verify hostname + CA      |

### 7. PostgreSQL: SearchPath & Schemas

`SearchPath` is the equivalent of `USE schema` in MySQL. Set to a non-`public` value if your tables live in a custom schema:

```go
cfg.SearchPath = "myschema,public"
```

## Performance Considerations

1. **Connection Pooling**: Size `MaxOpenConns` to expected concurrency
2. **Prepared Statements**: Enabled by default
3. **Indexing**: Ensure indexes on commonly queried columns
4. **Batch Operations**: Use `CreateInBatches` for bulk inserts
5. **Select Specific Columns**: Use `Select()` to avoid loading unneeded data

```go
gormDB.CreateInBatches(users, 100)

var names []string
gormDB.Model(&User{}).Select("name").Find(&names)

gormDB.Where("email = ?", email).First(&user) // requires index on email
```

## Testing

### Mock Database (driver-neutral)

```go
import (
    "testing"
    "github.com/DATA-DOG/go-sqlmock"
    "gorm.io/driver/mysql"
    "gorm.io/gorm"
)

func TestUserRepository(t *testing.T) {
    sqlDB, mock, _ := sqlmock.New()
    defer sqlDB.Close()

    gormDB, _ := gorm.Open(mysql.New(mysql.Config{
        Conn:                      sqlDB,
        SkipInitializeWithVersion: true,
    }), &gorm.Config{})

    mock.ExpectQuery("SELECT \\* FROM `users`").
        WillReturnRows(sqlmock.NewRows([]string{"id", "name", "age"}).
            AddRow(1, "Alice", 25))

    var user User
    if err := gormDB.First(&user).Error; err != nil {
        t.Error(err)
    }
}
```

### Integration Testing

```go
func TestPostgresIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    log, _ := logger.New(nil)
    cfg := &db.PostgresConfig{
        BaseConfig: db.BaseConfig{
            Host: "localhost", User: "test", Password: "test", Database: "test_db",
        },
    }
    database, err := db.NewPostgres(log, cfg)
    if err != nil {
        t.Fatal(err)
    }
    defer database.Close()
    // ... run real queries
}
```

## License

This project is licensed under the MIT License — see the [LICENSE](../LICENSE) file for details.
