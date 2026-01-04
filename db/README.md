# db

A MySQL database client wrapper built on GORM with connection pool management, structured logging, and configurable query settings.

## Features

- **GORM Integration**: Built on top of GORM v2 for robust ORM capabilities
- **Connection Pool Management**: Configurable max connections, idle connections, and connection lifetimes
- **Unified Logging**: Integrates with the project's logger package using zap for structured SQL logging
- **Slow Query Detection**: Automatic detection and logging of slow queries with configurable thresholds
- **SQL Tracing**: Detailed SQL execution logging with elapsed time, rows affected, and error details
- **Prepared Statements**: Enabled by default for better performance and security
- **Configuration Validation**: Automatic validation with sensible defaults

## Installation

```bash
go get github.com/dailyyoga/go-kit/db
```

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "log"

    "github.com/dailyyoga/go-kit/db"
    "github.com/dailyyoga/go-kit/logger"
)

type User struct {
    ID   int64  `gorm:"primaryKey"`
    Name string `gorm:"size:100"`
    Age  int
}

func main() {
    // Create logger
    log, err := logger.New(nil)
    if err != nil {
        panic(err)
    }
    defer log.Sync()

    // Configure database
    cfg := &db.Config{
        Host:     "localhost",
        Port:     3306,
        User:     "root",
        Password: "password",
        Database: "myapp",
    }

    // Create database client
    database, err := db.NewMySQL(log, cfg)
    if err != nil {
        log.Fatal("failed to connect database:", err)
    }
    defer database.Close()

    // Get GORM DB instance
    gormDB, err := database.DB()
    if err != nil {
        log.Fatal("failed to get db instance:", err)
    }

    // Use GORM for operations
    gormDB.AutoMigrate(&User{})

    // Create
    gormDB.Create(&User{Name: "Alice", Age: 25})

    // Query
    var user User
    gormDB.First(&user, "name = ?", "Alice")

    // Update
    gormDB.Model(&user).Update("Age", 26)

    // Delete
    gormDB.Delete(&user)
}
```

### With Context

```go
ctx := context.Background()

// Ping database
if err := database.Ping(ctx); err != nil {
    log.Fatal("database not reachable:", err)
}

// Use context in queries
gormDB, _ := database.DB()
var users []User
gormDB.WithContext(ctx).Where("age > ?", 18).Find(&users)
```

## Configuration

### Config Structure

```go
type Config struct {
    // Host is the host of the database (required)
    Host string

    // Port is the port of the database
    // Default: 3306
    Port int

    // User is the database user (required)
    User string

    // Password is the database password (required)
    Password string

    // Database is the name of the database (required)
    Database string

    // MaxOpenConns is the maximum number of open connections to the database
    // Default: 25
    MaxOpenConns int

    // MaxIdleConns is the maximum number of idle connections in the pool
    // Default: 10
    MaxIdleConns int

    // ConnMaxLifetime is the maximum lifetime of a connection
    // Default: 1800 * time.Second (30 minutes)
    ConnMaxLifetime time.Duration

    // ConnMaxIdleTime is the maximum idle time of a connection
    // Default: 600 * time.Second (10 minutes)
    ConnMaxIdleTime time.Duration

    // LogLevel is the GORM log level
    // Options: "silent", "error", "warn", "info"
    // Default: "warn"
    LogLevel string

    // SlowThreshold is the threshold for slow query detection
    // Default: 1 * time.Second
    SlowThreshold time.Duration

    // Charset is the charset of the database connection
    // Default: "utf8mb4"
    Charset string

    // Loc is the timezone location for parseTime
    // Default: "Local"
    Loc string
}
```

### Default Configuration

```go
// Using defaults for optional fields
cfg := &db.Config{
    Host:     "localhost",
    User:     "root",
    Password: "password",
    Database: "myapp",
    // Port, MaxOpenConns, etc. will use defaults
}
database, err := db.NewMySQL(log, cfg)

// Or use nil for full defaults (will fail validation - required fields missing)
// database, err := db.NewMySQL(log, nil)
```

### Custom Configuration

```go
cfg := &db.Config{
    Host:            "db.example.com",
    Port:            3306,
    User:            "app_user",
    Password:        "secure_password",
    Database:        "production_db",
    MaxOpenConns:    50,
    MaxIdleConns:    20,
    ConnMaxLifetime: 3600 * time.Second,  // 1 hour
    ConnMaxIdleTime: 900 * time.Second,   // 15 minutes
    LogLevel:        "info",              // Verbose SQL logging
    SlowThreshold:   500 * time.Millisecond,  // Detect queries > 500ms
    Charset:         "utf8mb4",
    Loc:             "UTC",
}
database, err := db.NewMySQL(log, cfg)
```

## Architecture

### Core Components

#### Database Interface

```go
type Database interface {
    // DB returns the underlying GORM DB instance
    DB() (*gorm.DB, error)

    // Ping checks if the database connection is alive
    Ping(ctx context.Context) error

    // Close closes the database connection
    Close() error
}
```

#### Custom GORM Logger

The package implements a custom GORM logger that bridges GORM's logging interface with the project's zap-based logger:

- **Structured Logging**: All SQL logs include structured fields (elapsed time, rows affected, SQL statement)
- **Slow Query Detection**: Queries exceeding `SlowThreshold` are logged at WARN level
- **Error Logging**: SQL errors are logged at ERROR level with full context
- **Log Levels**: Supports silent, error, warn, and info levels

### Connection Pool

The client configures the underlying `database/sql` connection pool with:

- **MaxOpenConns**: Limits total connections to prevent database overload
- **MaxIdleConns**: Keeps idle connections ready for reuse
- **ConnMaxLifetime**: Closes long-lived connections to handle database restarts
- **ConnMaxIdleTime**: Closes idle connections to free resources

### Data Flow

1. User calls `NewMySQL(log, cfg)`
2. Configuration is validated and merged with defaults
3. GORM DB instance is created with custom logger
4. Connection pool is configured
5. Initial `Ping()` test ensures connectivity
6. User retrieves `*gorm.DB` via `DB()` method
7. All GORM operations use the configured logger and pool settings
8. User calls `Close()` to release connections gracefully

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

database, err := db.NewMySQL(log, cfg)
if err != nil {
    // Check for invalid configuration
    if strings.Contains(err.Error(), "invalid config") {
        log.Error("configuration error:", err)
    }
    // Check for connection failure
    if strings.Contains(err.Error(), "connection failed") {
        log.Error("cannot connect to database:", err)
    }
}

// Check connection status
gormDB, err := database.DB()
if err != nil {
    if errors.Is(err, db.ErrConnectionNotEstablished) {
        log.Error("database not ready")
    }
}
```

## GORM Usage Examples

### Basic CRUD Operations

```go
gormDB, _ := database.DB()

// Create
user := User{Name: "Bob", Age: 30}
result := gormDB.Create(&user)
if result.Error != nil {
    log.Error("create failed:", result.Error)
}

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
gormDB, _ := database.DB()

// Transactions
err := gormDB.Transaction(func(tx *gorm.DB) error {
    if err := tx.Create(&user1).Error; err != nil {
        return err
    }
    if err := tx.Create(&user2).Error; err != nil {
        return err
    }
    return nil
})

// Associations
type Order struct {
    ID     int64
    UserID int64
    User   User
}

// Preload associations
var orders []Order
gormDB.Preload("User").Find(&orders)

// Joins
var results []struct {
    UserName  string
    OrderID   int64
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

### Log Levels

```go
// Silent: No SQL logs
cfg := &db.Config{
    LogLevel: "silent",
}

// Error: Only log SQL errors
cfg := &db.Config{
    LogLevel: "error",
}

// Warn: Log errors and slow queries
cfg := &db.Config{
    LogLevel: "warn",
    SlowThreshold: 500 * time.Millisecond,
}

// Info: Log all SQL statements with details
cfg := &db.Config{
    LogLevel: "info",
}
```

### Log Output Examples

**Normal Query (Info Level)**:
```
INFO  sql trace  component=gorm elapsed=2.3ms rows=5 sql="SELECT * FROM users WHERE age > 18"
```

**Slow Query (Warn Level)**:
```
WARN  slow sql  component=gorm elapsed=1.2s rows=1000 sql="SELECT * FROM users" threshold=1s
```

**SQL Error (Error Level)**:
```
ERROR  sql error  component=gorm elapsed=1.1ms rows=0 sql="SELECT * FROM invalid_table" error="Error 1146: Table 'mydb.invalid_table' doesn't exist"
```

## Best Practices

### 1. Connection Pool Sizing

```go
// For low-traffic applications
cfg := &db.Config{
    MaxOpenConns: 10,
    MaxIdleConns: 5,
}

// For high-traffic applications
cfg := &db.Config{
    MaxOpenConns: 100,
    MaxIdleConns: 25,
}

// General guideline: MaxIdleConns should be 20-50% of MaxOpenConns
```

### 2. Context Usage

```go
// Always use context for cancellation and timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

gormDB, _ := database.DB()
var users []User
result := gormDB.WithContext(ctx).Find(&users)
if result.Error != nil {
    if errors.Is(result.Error, context.DeadlineExceeded) {
        log.Error("query timeout")
    }
}
```

### 3. Graceful Shutdown

```go
// Ensure connections are closed on shutdown
defer database.Close()

// In server shutdown handler
func shutdown() {
    log.Info("shutting down database connection")
    if err := database.Close(); err != nil {
        log.Error("failed to close database:", err)
    }
}
```

### 4. Health Checks

```go
// Implement health check endpoint
func healthCheck(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
    defer cancel()

    if err := database.Ping(ctx); err != nil {
        w.WriteHeader(http.StatusServiceUnavailable)
        w.Write([]byte("database unhealthy"))
        return
    }
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("ok"))
}
```

### 5. Slow Query Tuning

```go
// Set aggressive threshold in development
cfg := &db.Config{
    LogLevel:      "warn",
    SlowThreshold: 100 * time.Millisecond,  // Find queries > 100ms
}

// Use normal threshold in production
cfg := &db.Config{
    LogLevel:      "warn",
    SlowThreshold: 1 * time.Second,
}
```

### 6. Error Handling

```go
gormDB, _ := database.DB()

// Always check errors
result := gormDB.Create(&user)
if result.Error != nil {
    // Check for specific GORM errors
    if errors.Is(result.Error, gorm.ErrRecordNotFound) {
        log.Info("record not found")
    } else if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
        log.Error("duplicate key violation")
    } else {
        log.Error("database error:", result.Error)
    }
}
```

## Use Cases

### 1. Web Application Backend

```go
// Initialize database once at startup
database, err := db.NewMySQL(log, cfg)
if err != nil {
    log.Fatal("database connection failed:", err)
}
defer database.Close()

// Use in HTTP handlers
func getUserHandler(w http.ResponseWriter, r *http.Request) {
    gormDB, _ := database.DB()

    var user User
    result := gormDB.WithContext(r.Context()).First(&user, r.URL.Query().Get("id"))
    if result.Error != nil {
        http.Error(w, "user not found", http.StatusNotFound)
        return
    }
    json.NewEncoder(w).Encode(user)
}
```

### 2. Background Worker

```go
// Worker with periodic database operations
func worker(ctx context.Context, database db.Database) {
    gormDB, _ := database.DB()

    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            var pendingTasks []Task
            gormDB.WithContext(ctx).Where("status = ?", "pending").Find(&pendingTasks)
            processTasks(pendingTasks)
        case <-ctx.Done():
            return
        }
    }
}
```

### 3. Data Migration Tool

```go
func migrate(database db.Database) error {
    gormDB, _ := database.DB()

    // Auto migrate schemas
    if err := gormDB.AutoMigrate(&User{}, &Order{}, &Product{}); err != nil {
        return fmt.Errorf("migration failed: %w", err)
    }

    log.Info("database migration completed")
    return nil
}
```

## Testing

### Mock Database for Testing

```go
import (
    "testing"
    "github.com/DATA-DOG/go-sqlmock"
    "gorm.io/driver/mysql"
    "gorm.io/gorm"
)

func TestUserRepository(t *testing.T) {
    // Create mock database
    sqlDB, mock, err := sqlmock.New()
    if err != nil {
        t.Fatal(err)
    }
    defer sqlDB.Close()

    // Create GORM DB with mock
    gormDB, err := gorm.Open(mysql.New(mysql.Config{
        Conn: sqlDB,
        SkipInitializeWithVersion: true,
    }), &gorm.Config{})
    if err != nil {
        t.Fatal(err)
    }

    // Set up expectations
    mock.ExpectQuery("SELECT \\* FROM `users`").
        WillReturnRows(sqlmock.NewRows([]string{"id", "name", "age"}).
            AddRow(1, "Alice", 25))

    // Run test
    var user User
    result := gormDB.First(&user)
    if result.Error != nil {
        t.Error("query failed:", result.Error)
    }
    if user.Name != "Alice" {
        t.Errorf("expected Alice, got %s", user.Name)
    }

    // Verify expectations
    if err := mock.ExpectationsWereMet(); err != nil {
        t.Error("unfulfilled expectations:", err)
    }
}
```

### Integration Testing

```go
func TestDatabaseIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    log, _ := logger.New(nil)
    cfg := &db.Config{
        Host:     "localhost",
        User:     "test",
        Password: "test",
        Database: "test_db",
    }

    database, err := db.NewMySQL(log, cfg)
    if err != nil {
        t.Fatal("connection failed:", err)
    }
    defer database.Close()

    gormDB, _ := database.DB()
    gormDB.AutoMigrate(&User{})

    // Test CRUD operations
    user := User{Name: "TestUser", Age: 30}
    gormDB.Create(&user)

    var found User
    gormDB.First(&found, user.ID)
    if found.Name != user.Name {
        t.Errorf("expected %s, got %s", user.Name, found.Name)
    }

    gormDB.Delete(&user)
}
```

## Performance Considerations

1. **Connection Pooling**: Properly size `MaxOpenConns` based on expected load
2. **Prepared Statements**: Enabled by default for better performance
3. **Index Usage**: Ensure proper indexes on frequently queried columns
4. **Batch Operations**: Use `CreateInBatches` for bulk inserts
5. **Select Specific Columns**: Use `Select()` to avoid loading unnecessary data

```go
// Efficient batch insert
gormDB.CreateInBatches(users, 100)

// Select only needed columns
var names []string
gormDB.Model(&User{}).Select("name").Find(&names)

// Use indexes effectively
gormDB.Where("email = ?", email).First(&user)  // Requires index on email
```

## License

This project is licensed under the MIT License - see the [LICENSE](../LICENSE) file for details.
