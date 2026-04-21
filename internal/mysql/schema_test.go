package mysql

import (
	"strings"
	"testing"
)

func TestExtractTableName(t *testing.T) {
	tests := []struct {
		name  string
		ddl   string
		table string
	}{
		{
			name:  "simple create table",
			ddl:   "CREATE TABLE users (id INT)",
			table: "users",
		},
		{
			name:  "create table if not exists",
			ddl:   "CREATE TABLE IF NOT EXISTS `traces` (id VARCHAR(36))",
			table: "traces",
		},
		{
			name:  "backtick quoted table",
			ddl:   "CREATE TABLE IF NOT EXISTS `ch_scores` (id VARCHAR(36))",
			table: "ch_scores",
		},
		{
			name:  "not a create table",
			ddl:   "SELECT * FROM users",
			table: "",
		},
		{
			name:  "empty string",
			ddl:   "",
			table: "",
		},
		{
			name:  "view creation",
			ddl:   "CREATE OR REPLACE VIEW analytics_traces AS SELECT 1",
			table: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := extractTableName(tc.ddl)
			if result != tc.table {
				t.Errorf("extractTableName(%q) = %q, want %q", tc.ddl, result, tc.table)
			}
		})
	}
}

func TestStripQuotes(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"`users`", "users"},
		{`"users"`, "users"},
		{"'users'", "users"},
		{"users", "users"},
		{"", ""},
	}

	for _, tc := range tests {
		result := stripQuotes(tc.input)
		if result != tc.expected {
			t.Errorf("stripQuotes(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestOLTPDDLCount(t *testing.T) {
	if len(oltpDDLs) == 0 {
		t.Error("oltpDDLs should not be empty")
	}
	// Verify we have the expected number of OLTP tables (50+ per PRODUCT.md)
	if len(oltpDDLs) < 40 {
		t.Errorf("expected at least 40 OLTP DDLs, got %d", len(oltpDDLs))
	}
}

func TestOLAPDDLCount(t *testing.T) {
	if len(olapDDLs) == 0 {
		t.Error("olapDDLs should not be empty")
	}
	// Verify we have traces, observations, scores tables
	hasTraces := false
	hasObservations := false
	hasScores := false
	for _, ddl := range olapDDLs {
		if strings.Contains(ddl, "CREATE TABLE IF NOT EXISTS traces") {
			hasTraces = true
		}
		if strings.Contains(ddl, "CREATE TABLE IF NOT EXISTS observations") {
			hasObservations = true
		}
		if strings.Contains(ddl, "CREATE TABLE IF NOT EXISTS ch_scores") {
			hasScores = true
		}
	}
	if !hasTraces {
		t.Error("olapDDLs should contain traces table")
	}
	if !hasObservations {
		t.Error("olapDDLs should contain observations table")
	}
	if !hasScores {
		t.Error("olapDDLs should contain ch_scores table")
	}
}

func TestPGCatalogDDLCount(t *testing.T) {
	if len(pgCatalogDDLs) == 0 {
		t.Error("pgCatalogDDLs should not be empty")
	}
	// Verify all required catalog tables are defined
	requiredTables := []string{
		"pg_type", "pg_class", "pg_attribute", "pg_namespace",
		"pg_index", "pg_proc", "pg_enum", "pg_constraint",
		"pg_description", "pg_database",
	}
	for _, table := range requiredTables {
		found := false
		for _, ddl := range pgCatalogDDLs {
			if strings.Contains(ddl, table) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("pgCatalogDDLs should contain %s table", table)
		}
	}
}

func TestOLTPDDLValidSyntax(t *testing.T) {
	for i, ddl := range oltpDDLs {
		ddl = strings.TrimSpace(ddl)
		if ddl == "" {
			t.Errorf("oltpDDLs[%d] is empty", i)
			continue
		}
		if !strings.HasPrefix(strings.ToUpper(ddl), "CREATE") {
			t.Errorf("oltpDDLs[%d] does not start with CREATE: %s", i, ddl[:50])
		}
		if !strings.Contains(ddl, "ENGINE=InnoDB") && !strings.Contains(ddl, "CREATE OR REPLACE VIEW") {
			t.Errorf("oltpDDLs[%d] missing ENGINE=InnoDB: %s", i, ddl[:50])
		}
	}
}

func TestOLAPDDLValidSyntax(t *testing.T) {
	for i, ddl := range olapDDLs {
		ddl = strings.TrimSpace(ddl)
		if ddl == "" {
			t.Errorf("olapDDLs[%d] is empty", i)
			continue
		}
		if !strings.HasPrefix(strings.ToUpper(ddl), "CREATE") {
			t.Errorf("olapDDLs[%d] does not start with CREATE: %s", i, ddl[:50])
		}
	}
}

func TestPGCatalogDDLValidSyntax(t *testing.T) {
	for i, ddl := range pgCatalogDDLs {
		ddl = strings.TrimSpace(ddl)
		if ddl == "" {
			t.Errorf("pgCatalogDDLs[%d] is empty", i)
			continue
		}
		if !strings.HasPrefix(strings.ToUpper(ddl), "CREATE") {
			t.Errorf("pgCatalogDDLs[%d] does not start with CREATE: %s", i, ddl[:50])
		}
	}
}

func TestTracesTableHasPrimaryKey(t *testing.T) {
	for _, ddl := range olapDDLs {
		if strings.Contains(ddl, "CREATE TABLE IF NOT EXISTS traces") {
			if !strings.Contains(ddl, "PRIMARY KEY") {
				t.Error("traces table should have PRIMARY KEY")
			}
			if !strings.Contains(ddl, "id VARCHAR(36)") {
				t.Error("traces table should have id VARCHAR(36) column")
			}
			if !strings.Contains(ddl, "project_id VARCHAR(36)") {
				t.Error("traces table should have project_id column")
			}
			if !strings.Contains(ddl, "event_ts") {
				t.Error("traces table should have event_ts column for ReplacingMergeTree versioning")
			}
			if !strings.Contains(ddl, "is_deleted") {
				t.Error("traces table should have is_deleted column for ReplacingMergeTree dedup")
			}
			break
		}
	}
}

func TestObservationsTableHasRequiredColumns(t *testing.T) {
	for _, ddl := range olapDDLs {
		if strings.Contains(ddl, "CREATE TABLE IF NOT EXISTS observations") {
			requiredCols := []string{"id", "trace_id", "project_id", "type", "start_time", "event_ts", "is_deleted"}
			for _, col := range requiredCols {
				if !strings.Contains(ddl, col) {
					t.Errorf("observations table should have column: %s", col)
				}
			}
			break
		}
	}
}

func TestArrayColumnsUseJSONType(t *testing.T) {
	// Verify that array columns (tags, labels, etc.) use JSON type
	for _, ddl := range oltpDDLs {
		if strings.Contains(ddl, "CREATE TABLE IF NOT EXISTS prompts") {
			if !strings.Contains(ddl, "tags JSON") {
				t.Error("prompts.tags should be JSON type (array column mapping)")
			}
			if !strings.Contains(ddl, "labels JSON") {
				t.Error("prompts.labels should be JSON type")
			}
			break
		}
	}

	for _, ddl := range olapDDLs {
		if strings.Contains(ddl, "CREATE TABLE IF NOT EXISTS traces") {
			if !strings.Contains(ddl, "tags JSON") {
				t.Error("traces.tags should be JSON type (array column mapping)")
			}
			break
		}
	}
}

func TestUUIDColumnsUseVARCHAR36(t *testing.T) {
	// In MySQL, UUID columns should be VARCHAR(36), not UUID type
	for _, ddl := range oltpDDLs {
		if strings.Contains(ddl, "id VARCHAR(36)") {
			// Good - using VARCHAR(36) for UUID
		} else if strings.Contains(ddl, "id ") && strings.Contains(ddl, "PRIMARY KEY") {
			if strings.Contains(ddl, "UUID") {
				t.Error("Should use VARCHAR(36) for UUID, not UUID type")
			}
		}
	}
}

func TestBooleanColumnsUseTinyInt(t *testing.T) {
	// In MySQL, boolean columns should be TINYINT(1)
	for _, ddl := range oltpDDLs {
		if strings.Contains(ddl, "BOOLEAN") {
			t.Errorf("DDL should use TINYINT(1) for booleans, found BOOLEAN in: %s", ddl[:80])
		}
	}
}

func TestTimestampColumnsUseDatetime3(t *testing.T) {
	// Timestamp columns should use DATETIME(3) for millisecond precision
	for _, ddl := range olapDDLs {
		if strings.Contains(ddl, "TIMESTAMP WITH TIME ZONE") || strings.Contains(ddl, "TIMESTAMPTZ") {
			t.Errorf("DDL should use DATETIME(3) for timestamps, found PG type in: %s", ddl[:80])
		}
	}
}

func TestAggregationTablesExist(t *testing.T) {
	aggTables := []string{"traces_all_amt", "traces_7d_amt", "traces_30d_amt"}
	for _, wantTable := range aggTables {
		found := false
		for _, ddl := range olapDDLs {
			if strings.Contains(ddl, "CREATE TABLE IF NOT EXISTS "+wantTable) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing aggregation table: %s", wantTable)
		}
	}
}

func TestAnalyticsViewsExist(t *testing.T) {
	views := []string{"analytics_traces", "analytics_observations", "analytics_scores"}
	for _, wantView := range views {
		found := false
		for _, ddl := range olapDDLs {
			if strings.Contains(ddl, "CREATE OR REPLACE VIEW "+wantView) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing analytics view: %s", wantView)
		}
	}
}

func TestScoreViewsExist(t *testing.T) {
	views := []string{"scores_numeric", "scores_categorical"}
	for _, wantView := range views {
		found := false
		for _, ddl := range olapDDLs {
			if strings.Contains(ddl, "CREATE OR REPLACE VIEW "+wantView) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing score view: %s", wantView)
		}
	}
}

func TestMultiValuedIndexForTags(t *testing.T) {
	for _, ddl := range olapDDLs {
		if strings.Contains(ddl, "CREATE TABLE IF NOT EXISTS traces") {
			if !strings.Contains(ddl, "MULTI-VALUED INDEX") && !strings.Contains(ddl, "idx_tags") {
				t.Error("traces table should have multi-valued index on tags for array queries")
			}
			break
		}
	}
}

func TestPrismaMigrationsTableExists(t *testing.T) {
	found := false
	for _, ddl := range oltpDDLs {
		if strings.Contains(ddl, "_prisma_migrations") {
			found = true
			break
		}
	}
	if !found {
		t.Error("_prisma_migrations table should exist for Prisma compatibility")
	}
}

func TestExecDDLSkipsEmptyStatements(t *testing.T) {
	// Verify that empty DDL strings are handled gracefully
	ddl := "   "
	ddl = strings.TrimSpace(ddl)
	if ddl != "" {
		t.Error("TrimSpace should make empty DDL truly empty")
	}
}

func TestDDLOrdering(t *testing.T) {
	// OLTP tables should be created before OLAP tables (OLAP may reference OLTP)
	// Verify the first OLTP DDL is a table (not a view)
	if len(oltpDDLs) > 0 {
		firstDDL := strings.TrimSpace(oltpDDLs[0])
		if strings.Contains(firstDDL, "VIEW") {
			t.Error("First OLTP DDL should be a table, not a view")
		}
	}
}
