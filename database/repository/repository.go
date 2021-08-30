package repository

import (
	"github.com/yurulab/gocryptotrader/database"
)

// GetSQLDialect returns current SQL Dialect based on enabled driver
func GetSQLDialect() string {
	switch database.DB.Config.Driver {
	case "sqlite", "sqlite3":
		return database.DBSQLite3
	case "psql", "postgres", "postgresql":
		return database.DBPostgreSQL
	}
	return "invalid driver"
}
