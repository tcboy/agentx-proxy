package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Listen  ListenConfig  `yaml:"listen"`
	MySQL   MySQLConfig   `yaml:"mysql"`
	Proxy   ProxyConfig   `yaml:"proxy"`
	Log     LogConfig     `yaml:"log"`
}

type ListenConfig struct {
	PostgreSQL     string `yaml:"postgresql"`
	ClickHouseNative string `yaml:"clickhouse_native"`
	ClickHouseHTTP string `yaml:"clickhouse_http"`
}

type MySQLConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	Database        string        `yaml:"database"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

type ProxyConfig struct {
	PGToMySQL   PGToMySQLConfig   `yaml:"pg_to_mysql"`
	CHToMySQL   CHToMySQLConfig   `yaml:"ch_to_mysql"`
}

type PGToMySQLConfig struct {
	Enabled        bool   `yaml:"enabled"`
	ArrayColumnMode string `yaml:"array_column_mode"`
	FulltextMode   string `yaml:"fulltext_mode"`
}

type CHToMySQLConfig struct {
	Enabled           bool          `yaml:"enabled"`
	AggMode           string        `yaml:"agg_mode"`
	WriteBufferSize   int           `yaml:"write_buffer_size"`
	WriteFlushInterval time.Duration `yaml:"write_flush_interval"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

func DefaultConfig() *Config {
	return &Config{
		Listen: ListenConfig{
			PostgreSQL:     "0.0.0.0:5432",
			ClickHouseNative: "0.0.0.0:9000",
			ClickHouseHTTP: "0.0.0.0:8123",
		},
		MySQL: MySQLConfig{
			Host:            "127.0.0.1",
			Port:            3306,
			User:            "langfuse",
			Password:        "",
			Database:        "langfuse",
			MaxOpenConns:    100,
			MaxIdleConns:    20,
			ConnMaxLifetime: 10 * time.Minute,
		},
		Proxy: ProxyConfig{
			PGToMySQL: PGToMySQLConfig{
				Enabled:        true,
				ArrayColumnMode: "json",
				FulltextMode:   "match_against",
			},
			CHToMySQL: CHToMySQLConfig{
				Enabled:           true,
				AggMode:           "realtime",
				WriteBufferSize:   10000,
				WriteFlushInterval: time.Second,
			},
		},
		Log: LogConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Override with env vars
	if v := os.Getenv("MYSQL_HOST"); v != "" {
		cfg.MySQL.Host = v
	}
	if v := os.Getenv("MYSQL_PORT"); v != "" {
		cfg.MySQL.Port = 3306
	}
	if v := os.Getenv("MYSQL_USER"); v != "" {
		cfg.MySQL.User = v
	}
	if v := os.Getenv("MYSQL_PASSWORD"); v != "" {
		cfg.MySQL.Password = v
	}
	if v := os.Getenv("MYSQL_DATABASE"); v != "" {
		cfg.MySQL.Database = v
	}
	if v := os.Getenv("PG_LISTEN"); v != "" {
		cfg.Listen.PostgreSQL = v
	}
	if v := os.Getenv("CH_NATIVE_LISTEN"); v != "" {
		cfg.Listen.ClickHouseNative = v
	}
	if v := os.Getenv("CH_HTTP_LISTEN"); v != "" {
		cfg.Listen.ClickHouseHTTP = v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}

	return cfg, nil
}
