package clickhouse

import (
	"testing"
)

func TestTranslateFinal(t *testing.T) {
	tr := NewCHTranslator()

	tests := []struct {
		input    string
		expected string
	}{
		{"SELECT * FROM traces FINAL", "SELECT * FROM traces "},
		{"SELECT id, name FROM scores FINAL WHERE project_id = 'abc'", "SELECT id, name FROM scores  WHERE project_id = 'abc'"},
	}

	for _, tc := range tests {
		result, err := tr.Translate(tc.input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != tc.expected {
			t.Errorf("Translate(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestTranslateMapAccess(t *testing.T) {
	tr := NewCHTranslator()

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "SELECT metadata['key'] FROM traces",
			expected: "SELECT JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.key')) FROM traces",
		},
	}

	for _, tc := range tests {
		result, err := tr.Translate(tc.input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != tc.expected {
			t.Errorf("Translate(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestTranslateHasFunctions(t *testing.T) {
	tr := NewCHTranslator()

	input := "SELECT * FROM traces WHERE hasAny(tags, ['tag1', 'tag2'])"
	result, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsStr(result, "JSON_OVERLAPS") {
		t.Errorf("expected JSON_OVERLAPS in result, got %q", result)
	}
}

func TestTranslateDateFunctions(t *testing.T) {
	tr := NewCHTranslator()

	tests := []struct {
		input    string
		contains string
	}{
		{
			input:    "SELECT toDate(timestamp)",
			contains: "DATE(timestamp)",
		},
		{
			input:    "SELECT toStartOfHour(timestamp)",
			contains: "DATE_FORMAT(timestamp, '%Y-%m-%d %H:00:00')",
		},
		{
			input:    "SELECT dateDiff('hour', start_time, end_time)",
			contains: "TIMESTAMPDIFF(HOUR, start_time, end_time)",
		},
		{
			input:    "SELECT toUnixTimestamp64Milli(timestamp)",
			contains: "UNIX_TIMESTAMP(timestamp) * 1000",
		},
	}

	for _, tc := range tests {
		result, err := tr.Translate(tc.input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !containsStr(result, tc.contains) {
			t.Errorf("Translate(%q) should contain %q, got %q", tc.input, tc.contains, result)
		}
	}
}

func TestTranslateCHAggregateFunctions(t *testing.T) {
	tr := NewCHTranslator()

	tests := []struct {
		input    string
		contains string
	}{
		{
			input:    "SELECT countIf(value > 0)",
			contains: "COUNT(CASE WHEN value > 0 THEN 1 END)",
		},
		{
			input:    "SELECT sumIf(cost, type = 'input')",
			contains: "SUM(CASE WHEN type = 'input' THEN cost END)",
		},
		{
			input:    "SELECT uniq(user_id)",
			contains: "COUNT(DISTINCT user_id)",
		},
		{
			input:    "SELECT groupArray(name)",
			contains: "JSON_ARRAYAGG(name)",
		},
	}

	for _, tc := range tests {
		result, err := tr.Translate(tc.input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !containsStr(result, tc.contains) {
			t.Errorf("Translate(%q) should contain %q, got %q", tc.input, tc.contains, result)
		}
	}
}

func TestTranslateCHParameters(t *testing.T) {
	tr := NewCHTranslator()

	input := "SELECT * FROM traces WHERE project_id = {projectId: String}"
	result, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsStr(result, "?") {
		t.Errorf("expected ? placeholder, got %q", result)
	}
}

func TestTranslateCast(t *testing.T) {
	tr := NewCHTranslator()

	tests := []struct {
		input    string
		contains string
	}{
		{
			input:    "SELECT timestamp::DateTime64(3)",
			contains: "CAST(timestamp AS DATETIME(3))",
		},
		{
			input:    "SELECT value::String",
			contains: "value",
		},
	}

	for _, tc := range tests {
		result, err := tr.Translate(tc.input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !containsStr(result, tc.contains) {
			t.Errorf("Translate(%q) should contain %q, got %q", tc.input, tc.contains, result)
		}
	}
}

func TestTranslateComplexQuery(t *testing.T) {
	tr := NewCHTranslator()

	// A typical Langfuse ClickHouse query
	input := `
		SELECT
			t.id,
			t.timestamp,
			t.name,
			t.user_id,
			t.metadata['session_id'] as session_id
		FROM traces t FINAL
		WHERE t.project_id = {projectId: String}
		AND t.timestamp >= {timestamp: DateTime64(3)}
		AND t.timestamp <= {timestamp: DateTime64(3)} + INTERVAL 2 DAY
		ORDER BY t.timestamp DESC
		LIMIT 1 BY id, project_id
	`

	result, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify key translations
	if containsStr(result, "FINAL") {
		t.Error("FINAL should be removed")
	}
	if !containsStr(result, "JSON_UNQUOTE(JSON_EXTRACT") {
		t.Error("metadata access should be translated")
	}
	if !containsStr(result, "?") {
		t.Error("parameters should be replaced with ?")
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
