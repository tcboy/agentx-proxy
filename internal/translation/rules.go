package translation

import (
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

// Config holds the translation rules loaded from YAML
type Config struct {
	PGToMySQL  *PGToMySQLConfig  `yaml:"pg_to_mysql"`
	CHToMySQL  *CHToMySQLConfig  `yaml:"ch_to_mysql"`
}

type PGToMySQLConfig struct {
	Types        map[string]string `yaml:"types"`
	Functions    map[string]string `yaml:"functions"`
	Operators    map[string]string `yaml:"operators"`
	ArrayColumns *ArrayColumnConfig `yaml:"array_columns"`
}

type ArrayColumnConfig struct {
	Mode string `yaml:"mode"`
}

type CHToMySQLConfig struct {
	Functions map[string]string `yaml:"functions"`
	Aggregates map[string]string `yaml:"aggregates"`
}

var (
	loaded     *Config
	loadedOnce sync.Once
)

// Load reads translation rules from a YAML file.
// On subsequent calls it returns the cached config.
func Load(path string) (*Config, error) {
	var err error
	loadedOnce.Do(func() {
		var data []byte
		data, err = os.ReadFile(path)
		if err != nil {
			return
		}
		var cfg Config
		err = yaml.Unmarshal(data, &cfg)
		if err != nil {
			return
		}
		loaded = &cfg
	})
	return loaded, err
}

// Default returns the cached config or nil if Load hasn't been called.
func Default() *Config {
	return loaded
}

// Reset clears the cached config (useful for tests).
func Reset() {
	loaded = nil
	loadedOnce = sync.Once{}
}
