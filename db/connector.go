package db

import "gorm.io/gorm"

// Connector is the contract a driver-specific config must satisfy so that
// shared connection-establishment code (openGorm) can build a *gorm.DB
// without knowing which driver is in play.
//
// Each driver's Config type implements this interface; callers continue to
// hold the concrete type so they keep access to driver-specific knobs
// (Charset for MySQL, SearchPath for PostgreSQL, ...).
type Connector interface {
	// Validate runs both shared and driver-specific configuration checks.
	Validate() error
	// DSN returns the driver-formatted connection string consumed by Dialector.
	DSN() string
	// Base exposes the shared connection / pool / logging fields.
	Base() *BaseConfig
	// Dialector returns a gorm.Dialector wrapping DSN with the driver's
	// gorm-side adapter (gorm.io/driver/mysql, gorm.io/driver/postgres, ...).
	Dialector() gorm.Dialector
}
