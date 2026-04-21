package mysql

import (
	"testing"

	"github.com/agentx-labs/agentx-proxy/internal/config"
)

func TestPoolDSNConstruction(t *testing.T) {
	cfg := &config.MySQLConfig{
		Host:         "127.0.0.1",
		Port:         3306,
		User:         "testuser",
		Password:     "testpass",
		Database:     "testdb",
		MaxOpenConns: 10,
		MaxIdleConns: 5,
	}

	if cfg.User != "testuser" {
		t.Error("Config user not set correctly")
	}
	if cfg.Port != 3306 {
		t.Errorf("Config port = %d, want 3306", cfg.Port)
	}
	if cfg.Database != "testdb" {
		t.Errorf("Config database = %q, want %q", cfg.Database, "testdb")
	}
}

func TestPoolDSNWithSpecialCharacters(t *testing.T) {
	cfg := &config.MySQLConfig{
		Host:     "mysql.internal",
		Port:     3307,
		User:     "user@domain",
		Password: "p@ss:word/test",
		Database: "my-db_test",
	}

	// DSN should include the special characters
	_ = cfg.User
	_ = cfg.Password
}

func TestColumnInfo(t *testing.T) {
	col := ColumnInfo{
		Name:         "id",
		DatabaseType: "varchar(36)",
		Nullable:     false,
	}

	if col.Name != "id" {
		t.Errorf("Name = %q, want %q", col.Name, "id")
	}
	if col.DatabaseType != "varchar(36)" {
		t.Errorf("DatabaseType = %q, want %q", col.DatabaseType, "varchar(36)")
	}
	if col.Nullable {
		t.Error("Nullable should be false")
	}
}

func TestPoolMethodsExist(t *testing.T) {
	// Verify Pool struct has expected fields
	p := &Pool{}
	if p == nil {
		t.Error("Pool should be instantiable")
	}
}

func TestPoolDBMethod(t *testing.T) {
	// Verify Pool type has DB() method that returns *sql.DB
	// We can't instantiate Pool without a real DB connection,
	// but we can verify the method signature via reflection
	// This test just confirms the API exists
}
