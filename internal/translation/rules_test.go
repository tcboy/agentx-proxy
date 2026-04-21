package translation

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	Reset()

	cfg, err := Load("../../migrations/translation_rules.yaml")
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.PGToMySQL == nil {
		t.Fatal("pg_to_mysql config is nil")
	}
	if cfg.CHToMySQL == nil {
		t.Fatal("ch_to_mysql config is nil")
	}

	// Check PG types
	if cfg.PGToMySQL.Types["JSONB"] != "JSON" {
		t.Errorf("expected JSONB->JSON, got %q", cfg.PGToMySQL.Types["JSONB"])
	}
	if cfg.PGToMySQL.Types["UUID"] != "VARCHAR(36)" {
		t.Errorf("expected UUID->VARCHAR(36), got %q", cfg.PGToMySQL.Types["UUID"])
	}

	// Check PG functions
	if cfg.PGToMySQL.Functions["ILIKE"] != "LIKE COLLATE utf8mb4_general_ci" {
		t.Errorf("ILIKE mapping = %q, want LIKE COLLATE", cfg.PGToMySQL.Functions["ILIKE"])
	}

	// Check CH functions
	if cfg.CHToMySQL.Functions["FINAL"] != "removed" {
		t.Errorf("FINAL mapping = %q, want removed", cfg.CHToMySQL.Functions["FINAL"])
	}
	if cfg.CHToMySQL.Functions["hasAny"] != "JSON_OVERLAPS" {
		t.Errorf("hasAny mapping = %q, want JSON_OVERLAPS", cfg.CHToMySQL.Functions["hasAny"])
	}

	// Check aggregates
	if cfg.CHToMySQL.Aggregates["SimpleAggregateFunction(min)"] != "MIN" {
		t.Errorf("SimpleAggregateFunction(min) = %q, want MIN", cfg.CHToMySQL.Aggregates["SimpleAggregateFunction(min)"])
	}
}

func TestDefault(t *testing.T) {
	Reset()
	if Default() != nil {
		t.Error("Default() should return nil before Load()")
	}

	// Create a temp file for this test
	tmp := filepath.Join(t.TempDir(), "test.yaml")
	os.WriteFile(tmp, []byte("pg_to_mysql:\n  types:\n    TEST: VAL\nch_to_mysql:\n  functions:\n    FN: X\n"), 0644)

	Load(tmp)
	if Default() == nil {
		t.Error("Default() should return config after Load()")
	}
	if Default().PGToMySQL.Types["TEST"] != "VAL" {
		t.Errorf("Default() TEST type = %q, want VAL", Default().PGToMySQL.Types["TEST"])
	}
}

func TestLoadMissing(t *testing.T) {
	Reset()
	_, err := Load("/nonexistent/path/translation_rules.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	Reset()
	tmp := filepath.Join(t.TempDir(), "bad.yaml")
	os.WriteFile(tmp, []byte("{{invalid: yaml: ["), 0644)

	_, err := Load(tmp)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}
