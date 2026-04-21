package clickhouse

import (
	"strings"
	"testing"
)

func TestBatchInsertsEmpty(t *testing.T) {
	result := BatchInserts([]string{})
	if result != "" {
		t.Errorf("BatchInserts([]) = %q, want empty", result)
	}
}

func TestBatchInsertsSingle(t *testing.T) {
	inputs := []string{
		"INSERT INTO traces (id, name) VALUES ('1', 'test')",
	}
	result := BatchInserts(inputs)
	if !strings.Contains(result, "INSERT INTO traces") {
		t.Errorf("BatchInserts should contain table name, got %q", result)
	}
	if !strings.Contains(result, "VALUES") {
		t.Errorf("BatchInserts should contain VALUES, got %q", result)
	}
}

func TestBatchInsertsMultiple(t *testing.T) {
	inputs := []string{
		"INSERT INTO traces (id, name) VALUES ('1', 'test1')",
		"INSERT INTO traces (id, name) VALUES ('2', 'test2')",
		"INSERT INTO traces (id, name) VALUES ('3', 'test3')",
	}
	result := BatchInserts(inputs)

	// All values should be combined
	if strings.Count(result, "VALUES") != 1 {
		t.Errorf("BatchInserts should combine into single VALUES, got %q", result)
	}
	if strings.Count(result, ",") < 2 {
		t.Errorf("BatchInserts should have comma-separated values, got %q", result)
	}
}

func TestBatchInsertsMultipleTables(t *testing.T) {
	inputs := []string{
		"INSERT INTO traces (id, name) VALUES ('1', 'trace1')",
		"INSERT INTO observations (id, type) VALUES ('2', 'span')",
		"INSERT INTO traces (id, name) VALUES ('3', 'trace2')",
	}
	result := BatchInserts(inputs)

	// Should contain both table names
	if !strings.Contains(result, "traces") {
		t.Errorf("Result should contain 'traces', got %q", result)
	}
	if !strings.Contains(result, "observations") {
		t.Errorf("Result should contain 'observations', got %q", result)
	}
}

func TestBatchInsertsNonInsertQueries(t *testing.T) {
	inputs := []string{
		"SELECT * FROM traces",
		"UPDATE traces SET name = 'test' WHERE id = '1'",
		"DELETE FROM traces WHERE id = '1'",
	}
	result := BatchInserts(inputs)
	if result != "" {
		t.Errorf("BatchInserts should return empty for non-INSERT queries, got %q", result)
	}
}

func TestExtractTableName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"INSERT INTO traces (id) VALUES ('1')", "traces"},
		{"INSERT INTO `observations` (id) VALUES ('1')", "`observations`"},
		{"INSERT INTO ch_scores (id) VALUES ('1')", "ch_scores"},
		{"SELECT * FROM traces", ""},
	}

	for _, tc := range tests {
		result := extractTableName(tc.input)
		if result != tc.expected {
			t.Errorf("extractTableName(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestBatchInsertsMixedQueries(t *testing.T) {
	inputs := []string{
		"INSERT INTO traces (id) VALUES ('1')",
		"SELECT * FROM users",
		"INSERT INTO traces (id) VALUES ('2')",
	}
	result := BatchInserts(inputs)

	// Should only process INSERT queries
	if strings.Contains(result, "SELECT") {
		t.Errorf("BatchInserts should not contain SELECT, got %q", result)
	}
}
