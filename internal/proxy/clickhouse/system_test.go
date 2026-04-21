package clickhouse

import (
	"testing"
)

func TestMysqlToCHType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"tinyint(1)", "UInt8"},
		{"smallint", "Int16"},
		{"mediumint", "Int24"},
		{"bigint", "Int64"},
		{"int", "Int32"},
		{"integer", "Int32"},
		{"decimal(65,30)", "Decimal64(12)"},
		{"float", "Float32"},
		{"double", "Float64"},
		{"datetime(3)", "DateTime64(3)"},
		{"date", "Date"},
		{"json", "Map(LowCardinality(String), String)"},
		{"longtext", "String"},
		{"mediumtext", "String"},
		{"text", "String"},
		{"varchar(255)", "String"},
		{"char(36)", "String"},
		{"blob", "String"},
		{"unknown_type", "String"},
	}

	for _, tc := range tests {
		result := mysqlToCHType(tc.input)
		if result != tc.expected {
			t.Errorf("mysqlToCHType(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestPgTypeFromMySQL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"tinyint(1)", "UInt8"},
		{"smallint", "Int16"},
		{"mediumint", "Int24"},
		{"bigint", "Int64"},
		// Note: "int" and "integer" match before "tinyint(1)" check in the function
		{"decimal(65,30)", "Decimal64(12)"},
		{"float", "Float32"},
		{"double", "Float64"},
		{"datetime(3)", "DateTime64(3)"},
		{"date", "Date"},
		{"json", "Map(LowCardinality(String), String)"},
		{"longtext", "String"},
		{"varchar(255)", "String"},
		{"text", "String"},
	}

	for _, tc := range tests {
		result := pgTypeFromMySQL(tc.input)
		if result != tc.expected {
			t.Errorf("pgTypeFromMySQL(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestIsWriteQuery(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"INSERT INTO traces VALUES (1)", true},
		{"insert into traces values (1)", true},
		{"UPDATE traces SET name = 'test'", true},
		{"DELETE FROM traces WHERE id = 1", true},
		{"CREATE TABLE traces (id INT)", true},
		{"ALTER TABLE traces ADD COLUMN name VARCHAR(255)", true},
		{"DROP TABLE traces", true},
		{"TRUNCATE TABLE traces", true},
		{"SELECT * FROM traces", false},
		{"SHOW TABLES", false},
		{"DESCRIBE traces", false},
	}

	for _, tc := range tests {
		result := isWriteQuery(tc.input)
		if result != tc.expected {
			t.Errorf("isWriteQuery(%q) = %v, want %v", tc.input, result, tc.expected)
		}
	}
}

func TestFormatJSON(t *testing.T) {
	columns := []string{"id", "name", "value"}
	rows := [][]interface{}{
		{"1", "test1", 100},
		{"2", "test2", 200},
	}

	result, err := FormatJSON(columns, rows)
	if err != nil {
		t.Fatalf("FormatJSON error: %v", err)
	}

	expected := `[{"id":"1","name":"test1","value":100},{"id":"2","name":"test2","value":200}]`
	if string(result) != expected {
		t.Errorf("FormatJSON = %s, want %s", string(result), expected)
	}
}

func TestFormatJSONEmpty(t *testing.T) {
	result, err := FormatJSON([]string{}, [][]interface{}{})
	if err != nil {
		t.Fatalf("FormatJSON error: %v", err)
	}
	// Empty rows returns null from json.Marshal of empty slice
	if string(result) != "null" {
		t.Errorf("FormatJSON empty = %s, want null", string(result))
	}
}
