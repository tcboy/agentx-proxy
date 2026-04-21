package postgresql

import (
	"fmt"
	"testing"
)

func TestTranslateILIKE(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	tests := []struct {
		input    string
		expected string
	}{
		{"SELECT * FROM users WHERE name ILIKE '%john%'", "SELECT * FROM users WHERE name LIKE COLLATE utf8mb4_general_ci '%john%'"},
		{"SELECT * FROM users WHERE name NOT ILIKE '%john%'", "SELECT * FROM users WHERE name NOT LIKE COLLATE utf8mb4_general_ci '%john%'"},
		{"SELECT ILIKE_test", "SELECT ILIKE_test"}, // Shouldn't match inside identifier
	}

	for _, tc := range tests {
		result := tr.translateILIKE(tc.input)
		if result != tc.expected {
			t.Errorf("translateILIKE(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestTranslateTypeCasts(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	tests := []struct {
		input    string
		expected string
	}{
		{"SELECT id::text FROM users", "SELECT id FROM users"},
		{"SELECT id::uuid FROM users", "SELECT id FROM users"},
		{`SELECT status::"CommentObjectType" FROM comments`, "SELECT status FROM comments"},
		{"SELECT id::boolean FROM users", "SELECT id FROM users"},
		{"SELECT id::integer FROM users", "SELECT id FROM users"},
		{"SELECT CAST(id AS text) FROM users", "SELECT id FROM users"},
	}

	for _, tc := range tests {
		result := tr.translateTypeCasts(tc.input)
		if result != tc.expected {
			t.Errorf("translateTypeCasts(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestTranslateOnConflict(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "DO NOTHING",
			input:    "INSERT INTO trace_sessions (id, project_id) VALUES ('123', '456') ON CONFLICT (id, project_id) DO NOTHING",
			contains: "INSERT IGNORE",
		},
		{
			name:     "DO UPDATE",
			input:    "INSERT INTO trace_sessions (id, project_id, created_at) VALUES ('123', '456', NOW()) ON CONFLICT (id, project_id) DO UPDATE SET created_at = EXCLUDED.created_at",
			contains: "ON DUPLICATE KEY UPDATE",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tr.translateOnConflict(tc.input)
			if !containsStr(result, tc.contains) {
				t.Errorf("translateOnConflict(%q) should contain %q, got %q", tc.input, tc.contains, result)
			}
		})
	}
}

func TestTranslateDateTrunc(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	tests := []struct {
		input    string
		expected string
	}{
		{"SELECT date_trunc('hour', created_at) FROM users", "SELECT DATE_FORMAT(created_at, '%Y-%m-%d %H:00:00') FROM users"},
		{"SELECT date_trunc('day', created_at) FROM users", "SELECT DATE(created_at) FROM users"},
		{"SELECT date_trunc('month', created_at) FROM users", "SELECT DATE_FORMAT(created_at, '%Y-%m-01') FROM users"},
		{"SELECT date_trunc('year', created_at) FROM users", "SELECT MAKEDATE(YEAR(created_at), 1) FROM users"},
	}

	for _, tc := range tests {
		result := tr.translateDateTrunc(tc.input)
		if result != tc.expected {
			t.Errorf("translateDateTrunc(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestTranslateExtractEpoch(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	tests := []struct {
		input    string
		expected string
	}{
		{"SELECT EXTRACT(EPOCH FROM created_at) FROM users", "SELECT UNIX_TIMESTAMP(created_at) FROM users"},
		{"SELECT EXTRACT(EPOCH FROM timestamp) FROM logs", "SELECT UNIX_TIMESTAMP(timestamp) FROM logs"},
	}

	for _, tc := range tests {
		result := tr.translateExtractEpoch(tc.input)
		if result != tc.expected {
			t.Errorf("translateExtractEpoch(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestTranslateJSONBFunctions(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	tests := []struct {
		input    string
		contains string
	}{
		{
			input:    "SELECT jsonb_set(data, '{key}', 'value')",
			contains: "JSON_SET(",
		},
		{
			input:    "SELECT jsonb_agg(name) FROM users",
			contains: "JSON_ARRAYAGG(",
		},
		{
			input:    "SELECT data->>'key' FROM users",
			contains: "JSON_UNQUOTE(JSON_EXTRACT(data, '$.key'))",
		},
		{
			input:    "SELECT data->'key' FROM users",
			contains: "JSON_EXTRACT(data, '$.key')",
		},
	}

	for _, tc := range tests {
		result := tr.translateJSONBFunctions(tc.input)
		if !containsStr(result, tc.contains) {
			t.Errorf("translateJSONBFunctions(%q) should contain %q, got %q", tc.input, tc.contains, result)
		}
	}
}

func TestTranslateArrayFunctions(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	tests := []struct {
		input    string
		contains string
	}{
		{
			input:    "'tag1' = ANY(tags)",
			contains: "JSON_CONTAINS(tags, '\"tag1\"')",
		},
		{
			input:    "SELECT cardinality(tags) FROM users",
			contains: "JSON_LENGTH(tags)",
		},
		{
			input:    "SELECT unnest(tags) FROM users",
			contains: "JSON_TABLE",
		},
	}

	for _, tc := range tests {
		result := tr.translateArrayFunctions(tc.input)
		if !containsStr(result, tc.contains) {
			t.Errorf("translateArrayFunctions(%q) should contain %q, got %q", tc.input, tc.contains, result)
		}
	}
}

func TestTranslateFinal(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	tests := []struct {
		input    string
		expected string
	}{
		{"SELECT * FROM traces FINAL", "SELECT * FROM traces "},
		{"SELECT id FROM observations FINAL WHERE project_id = '123'", "SELECT id FROM observations  WHERE project_id = '123'"},
	}

	for _, tc := range tests {
		result := tr.translateFinalKeyword(tc.input)
		if result != tc.expected {
			t.Errorf("translateFinalKeyword(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestTranslateMapAccess(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "SELECT metadata['key'] FROM traces",
			expected: "SELECT JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.key')) FROM traces",
		},
		{
			input:    "SELECT metadata[\"key\"] FROM traces",
			expected: "SELECT JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.key')) FROM traces",
		},
	}

	for _, tc := range tests {
		result := tr.translateMapAccess(tc.input)
		if result != tc.expected {
			t.Errorf("translateMapAccess(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestTranslateCHFunctions(t *testing.T) {
	// These functions are handled by the CH translator, not the PG translator
	// The PG translator handles PG->MySQL, CH translator handles CH->MySQL
}

func TestTranslateReturning(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	// Test that RETURNING clause is detected and transformed
	input := "INSERT INTO users (name) VALUES ('John') RETURNING id"
	result := tr.translateReturning(input)

	if result == input {
		t.Errorf("translateReturning should have modified the query")
	}
}

func TestTranslateDDL(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "CREATE TABLE users (id SERIAL PRIMARY KEY, tags TEXT[])",
			expected: "CREATE TABLE users (id BIGINT AUTO_INCREMENT PRIMARY KEY, tags JSON)",
		},
		{
			input:    "CREATE TABLE users (data JSONB, created_at TIMESTAMPTZ)",
			expected: "CREATE TABLE users (data JSON, created_at DATETIME(3))",
		},
		{
			input:    "CREATE TABLE users (is_active BOOLEAN, id UUID)",
			expected: "CREATE TABLE users (is_active TINYINT(1), id VARCHAR(36))",
		},
	}

	for _, tc := range tests {
		result := tr.TranslateDDL(tc.input)
		if result != tc.expected {
			t.Errorf("TranslateDDL(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStrAt(s, substr))
}

func containsStrAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestTranslateDollarParams(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	tests := []struct {
		input    string
		expected string
	}{
		{"SELECT * FROM users WHERE id = $1", "SELECT * FROM users WHERE id = ?"},
		{"SELECT * FROM users WHERE id = $1 AND name = $2", "SELECT * FROM users WHERE id = ? AND name = ?"},
		{"SELECT $1, $2, $3", "SELECT ?, ?, ?"},
	}

	for _, tc := range tests {
		result, err := tr.Translate(tc.input)
		if err != nil {
			t.Fatalf("Translate(%q) error: %v", tc.input, err)
		}
		if !containsStr(result, "?") {
			t.Errorf("Translate(%q) should contain ?, got %q", tc.input, result)
		}
		if containsStr(result, "$") {
			t.Errorf("Translate(%q) should not contain $, got %q", tc.input, result)
		}
	}
}

func TestTranslateStringAgg(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	input := "SELECT string_agg(name, ',') FROM users"
	result := tr.translateStringAgg(input)

	if !containsStr(result, "GROUP_CONCAT") {
		t.Errorf("translateStringAgg should contain GROUP_CONCAT, got %q", result)
	}
}

func TestTranslateBoolOperators(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	tests := []struct {
		input    string
		contains string
	}{
		{
			input:    "SELECT * FROM users WHERE active IS true",
			contains: "1",
		},
		{
			input:    "SELECT * FROM users WHERE deleted IS false",
			contains: "0",
		},
	}

	for _, tc := range tests {
		result := tr.translateBoolOperators(tc.input)
		if !containsStr(result, tc.contains) {
			t.Errorf("translateBoolOperators(%q) should contain %q, got %q", tc.input, tc.contains, result)
		}
	}
}

func TestTranslateFullPipeline(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	// A typical complex Langfuse PG query
	input := `SELECT id, name, created_at FROM traces WHERE project_id = $1 AND name ILIKE '%test%' ORDER BY created_at DESC LIMIT 10`
	result, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("Translate error: %v", err)
	}

	// Check key translations
	if containsStr(result, "ILIKE") {
		t.Error("ILIKE should be translated")
	}
	if containsStr(result, "$") {
		t.Error("$ params should be translated to ?")
	}
	if !containsStr(result, "LIKE") {
		t.Error("Should contain LIKE")
	}
}

func TestTranslateEdgeCases(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	tests := []struct {
		name  string
		input string
		check func(string) bool
	}{
		{
			name:  "empty query",
			input: "",
			check: func(s string) bool { return s == "" },
		},
		{
			name:  "query without special chars",
			input: "SELECT 1",
			check: func(s string) bool { return s == "SELECT 1" },
		},
		{
			name:  "query with dollar sign in string",
			input: "SELECT '$1' as literal",
			check: func(s string) bool { return containsStr(s, "?") },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tr.Translate(tc.input)
			if err != nil {
				t.Fatalf("Translate error: %v", err)
			}
			if !tc.check(result) {
				t.Errorf("Translate(%q) = %q failed check", tc.input, result)
			}
		})
	}
}

func TestTranslateOnConflictWithMultipleColumns(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	input := `INSERT INTO table1 (a, b, c) VALUES (1, 2, 3) ON CONFLICT (a, b) DO UPDATE SET c = EXCLUDED.c, a = EXCLUDED.a`
	result := tr.translateOnConflict(input)

	if !containsStr(result, "ON DUPLICATE KEY UPDATE") {
		t.Errorf("should contain ON DUPLICATE KEY UPDATE, got %q", result)
	}
	if containsStr(result, "EXCLUDED") {
		t.Error("should not contain EXCLUDED")
	}
	if !containsStr(result, "VALUES(") {
		t.Error("should contain VALUES()")
	}
}

func TestTranslateDateTruncAllUnits(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	tests := []struct {
		unit     string
		expected string
	}{
		{"second", "DATE_FORMAT(ts, '%Y-%m-%d %H:%i:%s')"},
		{"minute", "DATE_FORMAT(ts, '%Y-%m-%d %H:%i:00')"},
		{"hour", "DATE_FORMAT(ts, '%Y-%m-%d %H:00:00')"},
		{"day", "DATE(ts)"},
		{"week", "DATE_SUB(ts, INTERVAL WEEKDAY(ts) DAY)"},
		{"month", "DATE_FORMAT(ts, '%Y-%m-01')"},
		{"quarter", "MAKEDATE(YEAR(ts), 1) + INTERVAL"},
		{"year", "MAKEDATE(YEAR(ts), 1)"},
	}

	for _, tc := range tests {
		input := fmt.Sprintf("SELECT date_trunc('%s', ts)", tc.unit)
		result := tr.translateDateTrunc(input)
		if !containsStr(result, tc.expected) {
			t.Errorf("date_trunc('%s') should contain %q, got %q", tc.unit, tc.expected, result)
		}
	}
}

func TestTranslateJSONBOperatorsNested(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	input := "SELECT data->'key'->'nested' FROM users"
	result := tr.translateJSONBFunctions(input)

	if !containsStr(result, "JSON_EXTRACT") {
		t.Errorf("should contain JSON_EXTRACT, got %q", result)
	}
}

func TestTranslateIntervalArith(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	input := "SELECT * FROM logs WHERE created_at > NOW() - INTERVAL '7' DAY"
	result := tr.translateIntervalArith(input)

	if !containsStr(result, "INTERVAL 7 DAY") {
		t.Errorf("should contain INTERVAL 7 DAY, got %q", result)
	}
}

func TestTranslateGenerateSeries(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	input := "SELECT * FROM GENERATE_SERIES(1, 10)"
	result := tr.translateGenerateSeries(input)

	if !containsStr(result, "WITH RECURSIVE") {
		t.Errorf("should contain WITH RECURSIVE, got %q", result)
	}
}

func TestTranslateLimit1ByWithWrap(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	input := "SELECT id, name FROM traces ORDER BY timestamp DESC LIMIT 1 BY id, project_id"
	result := tr.translateLimit1By(input)

	if !containsStr(result, "ROW_NUMBER()") {
		t.Errorf("should use ROW_NUMBER(), got %q", result)
	}
	if !containsStr(result, "PARTITION BY") {
		t.Errorf("should contain PARTITION BY, got %q", result)
	}
}

func TestTranslateToTsVectorMatchAgainst(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	input := "SELECT * FROM comments WHERE to_tsvector('english', content) @@ plainto_tsquery('english', 'search term')"
	result := tr.translateToTsVector(input)

	if !containsStr(result, "MATCH(") {
		t.Errorf("should contain MATCH(), got %q", result)
	}
	if !containsStr(result, "AGAINST(") {
		t.Errorf("should contain AGAINST(), got %q", result)
	}
}

func TestTranslateToTsVectorLikeMode(t *testing.T) {
	tr := NewTranslator("json", "like")

	input := "SELECT * FROM comments WHERE to_tsvector('english', content) @@ plainto_tsquery('english', 'search')"
	result := tr.translateToTsVector(input)

	if !containsStr(result, "LIKE") {
		t.Errorf("in like mode, should contain LIKE, got %q", result)
	}
}
