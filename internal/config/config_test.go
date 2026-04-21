package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Listen.PostgreSQL != "0.0.0.0:5432" {
		t.Errorf("default PG listen = %q, want %q", cfg.Listen.PostgreSQL, "0.0.0.0:5432")
	}
	if cfg.Listen.ClickHouseNative != "0.0.0.0:9000" {
		t.Errorf("default CH native listen = %q, want %q", cfg.Listen.ClickHouseNative, "0.0.0.0:9000")
	}
	if cfg.Listen.ClickHouseHTTP != "0.0.0.0:8123" {
		t.Errorf("default CH HTTP listen = %q, want %q", cfg.Listen.ClickHouseHTTP, "0.0.0.0:8123")
	}
	if cfg.MySQL.Host != "127.0.0.1" {
		t.Errorf("default MySQL host = %q, want %q", cfg.MySQL.Host, "127.0.0.1")
	}
	if cfg.MySQL.Port != 3306 {
		t.Errorf("default MySQL port = %d, want %d", cfg.MySQL.Port, 3306)
	}
	if cfg.MySQL.Database != "langfuse" {
		t.Errorf("default MySQL database = %q, want %q", cfg.MySQL.Database, "langfuse")
	}
	if cfg.MySQL.MaxOpenConns != 100 {
		t.Errorf("default MySQL max_open_conns = %d, want %d", cfg.MySQL.MaxOpenConns, 100)
	}
	if cfg.MySQL.MaxIdleConns != 20 {
		t.Errorf("default MySQL max_idle_conns = %d, want %d", cfg.MySQL.MaxIdleConns, 20)
	}
	if cfg.MySQL.ConnMaxLifetime != 10*time.Minute {
		t.Errorf("default MySQL conn_max_lifetime = %v, want %v", cfg.MySQL.ConnMaxLifetime, 10*time.Minute)
	}
	if !cfg.Proxy.PGToMySQL.Enabled {
		t.Error("default PGToMySQL should be enabled")
	}
	if !cfg.Proxy.CHToMySQL.Enabled {
		t.Error("default CHToMySQL should be enabled")
	}
}

func TestLoadConfigFromYAML(t *testing.T) {
	content := `
listen:
  postgresql: "0.0.0.0:15432"
  clickhouse_native: "0.0.0.0:19000"
  clickhouse_http: "0.0.0.0:18123"

mysql:
  host: "mysql.internal"
  port: 3307
  user: "testuser"
  password: "testpass"
  database: "testdb"
  max_open_conns: 50
  max_idle_conns: 10
  conn_max_lifetime: "5m"

proxy:
  pg_to_mysql:
    enabled: true
    array_column_mode: "delimited"
    fulltext_mode: "like"
  ch_to_mysql:
    enabled: true
    agg_mode: "async"
    write_buffer_size: 5000
    write_flush_interval: "2s"

log:
  level: "debug"
  format: "text"
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if cfg.Listen.PostgreSQL != "0.0.0.0:15432" {
		t.Errorf("PG listen = %q, want %q", cfg.Listen.PostgreSQL, "0.0.0.0:15432")
	}
	if cfg.MySQL.Host != "mysql.internal" {
		t.Errorf("MySQL host = %q, want %q", cfg.MySQL.Host, "mysql.internal")
	}
	if cfg.MySQL.User != "testuser" {
		t.Errorf("MySQL user = %q, want %q", cfg.MySQL.User, "testuser")
	}
	if cfg.Proxy.PGToMySQL.ArrayColumnMode != "delimited" {
		t.Errorf("array_column_mode = %q, want %q", cfg.Proxy.PGToMySQL.ArrayColumnMode, "delimited")
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("log level = %q, want %q", cfg.Log.Level, "debug")
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("LoadConfig should return nil error for missing file, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadConfig should return default config for missing file")
	}
	if cfg.MySQL.Host != "127.0.0.1" {
		t.Error("Should return default config")
	}
}

func TestLoadConfigEnvOverride(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(path, []byte("mysql:\n  host: 'yaml-host'"), 0644)

	os.Setenv("MYSQL_HOST", "env-host")
	os.Setenv("MYSQL_USER", "env-user")
	os.Setenv("MYSQL_DATABASE", "env-db")
	os.Setenv("LOG_LEVEL", "warn")
	defer func() {
		os.Unsetenv("MYSQL_HOST")
		os.Unsetenv("MYSQL_USER")
		os.Unsetenv("MYSQL_DATABASE")
		os.Unsetenv("LOG_LEVEL")
	}()

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if cfg.MySQL.Host != "env-host" {
		t.Errorf("MySQL host = %q, want %q (env should override yaml)", cfg.MySQL.Host, "env-host")
	}
	if cfg.MySQL.User != "env-user" {
		t.Errorf("MySQL user = %q, want %q", cfg.MySQL.User, "env-user")
	}
	if cfg.MySQL.Database != "env-db" {
		t.Errorf("MySQL database = %q, want %q", cfg.MySQL.Database, "env-db")
	}
	if cfg.Log.Level != "warn" {
		t.Errorf("log level = %q, want %q", cfg.Log.Level, "warn")
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")
	os.WriteFile(path, []byte("invalid: yaml: ["), 0644)

	_, err := LoadConfig(path)
	if err == nil {
		t.Error("LoadConfig should return error for invalid YAML")
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := DefaultConfig()

	// Check PG proxy defaults
	if cfg.Proxy.PGToMySQL.Enabled != true {
		t.Error("PG proxy should be enabled by default")
	}
	if cfg.Proxy.PGToMySQL.ArrayColumnMode != "json" {
		t.Errorf("array_column_mode = %q, want %q", cfg.Proxy.PGToMySQL.ArrayColumnMode, "json")
	}
	if cfg.Proxy.PGToMySQL.FulltextMode != "match_against" {
		t.Errorf("fulltext_mode = %q, want %q", cfg.Proxy.PGToMySQL.FulltextMode, "match_against")
	}

	// Check CH proxy defaults
	if cfg.Proxy.CHToMySQL.AggMode != "realtime" {
		t.Errorf("agg_mode = %q, want %q", cfg.Proxy.CHToMySQL.AggMode, "realtime")
	}
	if cfg.Proxy.CHToMySQL.WriteBufferSize != 10000 {
		t.Errorf("write_buffer_size = %d, want %d", cfg.Proxy.CHToMySQL.WriteBufferSize, 10000)
	}
	if cfg.Proxy.CHToMySQL.WriteFlushInterval != time.Second {
		t.Errorf("write_flush_interval = %v, want %v", cfg.Proxy.CHToMySQL.WriteFlushInterval, time.Second)
	}

	// Check log defaults
	if cfg.Log.Level != "info" {
		t.Errorf("log level = %q, want %q", cfg.Log.Level, "info")
	}
	if cfg.Log.Format != "json" {
		t.Errorf("log format = %q, want %q", cfg.Log.Format, "json")
	}
}
