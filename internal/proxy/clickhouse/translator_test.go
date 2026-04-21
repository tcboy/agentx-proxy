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

func TestTranslateHasAll(t *testing.T) {
	tr := NewCHTranslator()

	input := "SELECT * FROM traces WHERE hasAll(tags, ['tag1', 'tag2'])"
	result, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsStr(result, "JSON_CONTAINS") {
		t.Errorf("expected JSON_CONTAINS in result, got %q", result)
	}
	if !containsStr(result, "AND") {
		t.Errorf("expected AND between conditions, got %q", result)
	}
}

func TestTranslateHas(t *testing.T) {
	tr := NewCHTranslator()

	input := "SELECT * FROM traces WHERE has(tags, 'tag1')"
	result, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsStr(result, "JSON_CONTAINS") {
		t.Errorf("expected JSON_CONTAINS in result, got %q", result)
	}
}

func TestTranslateArrayJoin(t *testing.T) {
	tr := NewCHTranslator()

	input := "SELECT * FROM arrayJoin(tags)"
	result, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsStr(result, "JSON_TABLE") {
		t.Errorf("expected JSON_TABLE in result, got %q", result)
	}
}

func TestTranslateToStartOfFunctions(t *testing.T) {
	tr := NewCHTranslator()

	tests := []struct {
		input    string
		contains string
	}{
		{
			input:    "SELECT toStartOfMinute(ts)",
			contains: "DATE_FORMAT(ts, '%Y-%m-%d %H:%i:00')",
		},
		{
			input:    "SELECT toStartOfMonth(ts)",
			contains: "DATE_FORMAT(ts, '%Y-%m-01')",
		},
		{
			input:    "SELECT toStartOfYear(ts)",
			contains: "MAKEDATE(YEAR(ts), 1)",
		},
		{
			input:    "SELECT toStartOfDay(ts)",
			contains: "DATE(ts)",
		},
	}

	for _, tc := range tests {
		result, err := tr.Translate(tc.input)
		if err != nil {
			t.Fatalf("Translate(%q) error: %v", tc.input, err)
		}
		if !containsStr(result, tc.contains) {
			t.Errorf("Translate(%q) should contain %q, got %q", tc.input, tc.contains, result)
		}
	}
}

func TestTranslateDateDiff(t *testing.T) {
	tr := NewCHTranslator()

	tests := []struct {
		input    string
		contains string
	}{
		{
			input:    "SELECT dateDiff('hour', start, end)",
			contains: "TIMESTAMPDIFF(HOUR, start, end)",
		},
		{
			input:    "SELECT dateDiff('day', start, end)",
			contains: "TIMESTAMPDIFF(DAY, start, end)",
		},
		{
			input:    "SELECT dateDiff('millisecond', start, end)",
			contains: "TIMESTAMPDIFF(MICROSECOND",
		},
	}

	for _, tc := range tests {
		result, err := tr.Translate(tc.input)
		if err != nil {
			t.Fatalf("Translate(%q) error: %v", tc.input, err)
		}
		if !containsStr(result, tc.contains) {
			t.Errorf("Translate(%q) should contain %q, got %q", tc.input, tc.contains, result)
		}
	}
}

func TestTranslateEmptyNotEmpty(t *testing.T) {
	tr := NewCHTranslator()

	tests := []struct {
		input    string
		contains string
	}{
		{
			input:    "SELECT * FROM t WHERE empty(arr)",
			contains: "JSON_LENGTH(arr) = 0",
		},
		{
			input:    "SELECT * FROM t WHERE notEmpty(arr)",
			contains: "JSON_LENGTH(arr) > 0",
		},
	}

	for _, tc := range tests {
		result, err := tr.Translate(tc.input)
		if err != nil {
			t.Fatalf("Translate(%q) error: %v", tc.input, err)
		}
		if !containsStr(result, tc.contains) {
			t.Errorf("Translate(%q) should contain %q, got %q", tc.input, tc.contains, result)
		}
	}
}

func TestTranslateGroupArray(t *testing.T) {
	tr := NewCHTranslator()

	tests := []struct {
		input    string
		contains string
	}{
		{
			input:    "SELECT groupArray(name)",
			contains: "JSON_ARRAYAGG(name)",
		},
		{
			input:    "SELECT groupUniqArray(name)",
			contains: "JSON_ARRAYAGG(DISTINCT name)",
		},
	}

	for _, tc := range tests {
		result, err := tr.Translate(tc.input)
		if err != nil {
			t.Fatalf("Translate(%q) error: %v", tc.input, err)
		}
		if !containsStr(result, tc.contains) {
			t.Errorf("Translate(%q) should contain %q, got %q", tc.input, tc.contains, result)
		}
	}
}

func TestTranslateArgMax(t *testing.T) {
	tr := NewCHTranslator()

	input := "SELECT argMax(value, timestamp) FROM traces"
	result, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsStr(result, "GROUP_CONCAT") {
		t.Errorf("expected GROUP_CONCAT in result, got %q", result)
	}
}

func TestTranslateMapFunctions(t *testing.T) {
	tr := NewCHTranslator()

	tests := []struct {
		input    string
		contains string
	}{
		{
			input:    "SELECT mapKeys(metadata)",
			contains: "JSON_KEYS(metadata)",
		},
		{
			input:    "SELECT mapContains(metadata, 'key')",
			contains: "JSON_CONTAINS_PATH",
		},
	}

	for _, tc := range tests {
		result, err := tr.Translate(tc.input)
		if err != nil {
			t.Fatalf("Translate(%q) error: %v", tc.input, err)
		}
		if !containsStr(result, tc.contains) {
			t.Errorf("Translate(%q) should contain %q, got %q", tc.input, tc.contains, result)
		}
	}
}

func TestTranslateIfFunctions(t *testing.T) {
	tr := NewCHTranslator()

	tests := []struct {
		input    string
		contains string
	}{
		{
			input:    "SELECT avgIf(value, condition)",
			contains: "AVG(CASE WHEN condition THEN value END)",
		},
		{
			input:    "SELECT minIf(value, condition)",
			contains: "MIN(CASE WHEN condition THEN value END)",
		},
		{
			input:    "SELECT maxIf(value, condition)",
			contains: "MAX(CASE WHEN condition THEN value END)",
		},
	}

	for _, tc := range tests {
		result, err := tr.Translate(tc.input)
		if err != nil {
			t.Fatalf("Translate(%q) error: %v", tc.input, err)
		}
		if !containsStr(result, tc.contains) {
			t.Errorf("Translate(%q) should contain %q, got %q", tc.input, tc.contains, result)
		}
	}
}

func TestTranslateComplexCHQuery(t *testing.T) {
	tr := NewCHTranslator()

	input := `
		SELECT
			toStartOfHour(timestamp) as hour,
			countIf(status = 'error') as errors,
			sumIf(cost, cost > 0) as total_cost,
			uniq(user_id) as unique_users,
			groupArray(name) as names
		FROM traces FINAL
		WHERE project_id = {projectId: String}
		AND hasAny(tags, ['important'])
		GROUP BY hour
		ORDER BY hour DESC
	`

	result, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify key translations
	if containsStr(result, "FINAL") {
		t.Error("FINAL should be removed")
	}
	if !containsStr(result, "COUNT(CASE WHEN") {
		t.Error("countIf should be translated")
	}
	if !containsStr(result, "SUM(CASE WHEN") {
		t.Error("sumIf should be translated")
	}
	if !containsStr(result, "COUNT(DISTINCT") {
		t.Error("uniq should be translated")
	}
	if !containsStr(result, "JSON_ARRAYAGG") {
		t.Error("groupArray should be translated")
	}
	if !containsStr(result, "?") {
		t.Error("parameters should be replaced")
	}
	if !containsStr(result, "JSON_OVERLAPS") {
		t.Error("hasAny should be translated")
	}
}

func TestTranslateToUnixTimestamp64Nano(t *testing.T) {
	tr := NewCHTranslator()

	input := "SELECT toUnixTimestamp64Nano(timestamp)"
	result, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsStr(result, "UNIX_TIMESTAMP") {
		t.Errorf("expected UNIX_TIMESTAMP in result, got %q", result)
	}
	if !containsStr(result, "1000000000") {
		t.Errorf("expected * 1000000000 in result, got %q", result)
	}
}

func TestTranslateTodayYesterday(t *testing.T) {
	tr := NewCHTranslator()

	tests := []struct {
		input    string
		expected string
	}{
		{"SELECT today()", "SELECT CURDATE()"},
		{"SELECT yesterday()", "SELECT CURDATE() - INTERVAL 1 DAY"},
	}

	for _, tc := range tests {
		result, err := tr.Translate(tc.input)
		if err != nil {
			t.Fatalf("Translate(%q) error: %v", tc.input, err)
		}
		if result != tc.expected {
			t.Errorf("Translate(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestTranslateDecimalCast(t *testing.T) {
	tr := NewCHTranslator()

	input := "SELECT value::Decimal64(12)"
	result, err := tr.Translate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsStr(result, "CAST(") {
		t.Errorf("expected CAST in result, got %q", result)
	}
	if !containsStr(result, "DECIMAL") {
		t.Errorf("expected DECIMAL in result, got %q", result)
	}
}
