package postgresql

import (
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
