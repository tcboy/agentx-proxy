package clickhouse

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/agentx-labs/agentx-proxy/internal/mysql"
)

// SystemHandler handles ClickHouse system table queries
type SystemHandler struct {
	pool *mysql.Pool
}

func NewSystemHandler(pool *mysql.Pool) *SystemHandler {
	return &SystemHandler{pool: pool}
}

// QuerySystemTables handles SELECT * FROM system.tables
func (h *SystemHandler) QuerySystemTables(ctx context.Context) ([]string, [][]interface{}, error) {
	cols := []string{
		"database", "name", "uuid", "engine", "engine_full", "sorting_key",
		"primary_key", "partition_key", "sampling_key", "storage_policy",
		"total_rows", "total_bytes", "lifetime_rows", "lifetime_bytes",
		"parts", "active_parts", "marks",
	}

	rows, err := h.pool.Query(ctx, `
		SELECT table_name, table_name, '', 'MergeTree', 'MergeTree()', '', '', '', '', '',
			0, 0, 0, 0, 0, 0, 0
		FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'
	`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var data [][]interface{}
	for rows.Next() {
		row := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range row {
			ptrs[i] = &row[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			continue
		}

		// Prefix with database name
		result := make([]interface{}, len(cols))
		result[0] = "default"
		for i := 1; i < len(cols); i++ {
			result[i] = row[i]
		}
		data = append(data, result)
	}

	return cols, data, nil
}

// QuerySystemColumns handles SELECT * FROM system.columns
func (h *SystemHandler) QuerySystemColumns(ctx context.Context) ([]string, [][]interface{}, error) {
	cols := []string{
		"database", "table", "name", "type", "position", "default_expression",
		"comment", "codec_expression", "default_type",
	}

	rows, err := h.pool.Query(ctx, `
		SELECT table_name, column_name, column_type, ordinal_position
		FROM information_schema.columns
		WHERE table_schema = DATABASE()
		ORDER BY table_name, ordinal_position
	`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var data [][]interface{}
	for rows.Next() {
		var table, name, mysqlType string
		var pos int
		if err := rows.Scan(&table, &name, &mysqlType, &pos); err != nil {
			continue
		}

		chType := mysqlToCHType(mysqlType)
		data = append(data, []interface{}{
			"default", table, name, chType, pos, "", "", "", "",
		})
	}

	return cols, data, nil
}

// QuerySystemParts handles SELECT * FROM system.parts
func (h *SystemHandler) QuerySystemParts(ctx context.Context) ([]string, [][]interface{}, error) {
	cols := []string{
		"database", "table", "partition_id", "name", "active", "marks",
		"rows", "bytes_on_disk", "modification_time", "remove_time",
	}

	// Return a single part per table
	rows, err := h.pool.Query(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'
	`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var data [][]interface{}
	for rows.Next() {
		var table string
		rows.Scan(&table)
		data = append(data, []interface{}{
			"default", table, "all", "all_0", true, 1, 0, 0, "", "",
		})
	}

	return cols, data, nil
}

// QuerySystemProcesses handles SELECT * FROM system.processes
func (h *SystemHandler) QuerySystemProcesses(ctx context.Context) ([]string, [][]interface{}, error) {
	cols := []string{
		"query_id", "address", "user", "initial_user", "query",
		"elapsed", "is_cancelled", "read_rows", "written_rows",
	}

	return cols, nil, nil
}

// QuerySystemMetrics handles SELECT * FROM system.metrics
func (h *SystemHandler) QuerySystemMetrics(ctx context.Context) ([]string, [][]interface{}, error) {
	cols := []string{"metric", "value", "description"}

	metrics := [][]interface{}{
		{"Query", 0, "Number of executing queries"},
		{"Merge", 0, "Number of executing background merges"},
		{"PartMutation", 0, "Number of mutations"},
		{"ReplicatedFetch", 0, "Number of data parts being fetched from replica"},
		{"ReplicatedSend", 0, "Number of data parts being sent to replicas"},
	}

	return cols, metrics, nil
}

func mysqlToCHType(mysqlType string) string {
	switch {
	case strings.Contains(mysqlType, "tinyint(1)"):
		return "UInt8"
	case strings.Contains(mysqlType, "smallint"):
		return "Int16"
	case strings.Contains(mysqlType, "mediumint"):
		return "Int24"
	case strings.Contains(mysqlType, "bigint"):
		return "Int64"
	case strings.Contains(mysqlType, "int"), strings.Contains(mysqlType, "integer"):
		return "Int32"
	case strings.Contains(mysqlType, "decimal"):
		return "Decimal64(12)"
	case strings.Contains(mysqlType, "float"):
		return "Float32"
	case strings.Contains(mysqlType, "double"):
		return "Float64"
	case strings.Contains(mysqlType, "datetime"):
		return "DateTime64(3)"
	case strings.Contains(mysqlType, "date"):
		return "Date"
	case strings.Contains(mysqlType, "json"):
		return "Map(LowCardinality(String), String)"
	case strings.Contains(mysqlType, "longtext"):
		return "String"
	case strings.Contains(mysqlType, "mediumtext"):
		return "String"
	case strings.Contains(mysqlType, "text"):
		return "String"
	case strings.Contains(mysqlType, "varchar"), strings.Contains(mysqlType, "char"):
		return "String"
	case strings.Contains(mysqlType, "blob"):
		return "String"
	default:
		return "String"
	}
}

// CHResponse represents a ClickHouse response format
type CHResponse struct {
	Meta     []ColumnMeta `json:"meta,omitempty"`
	Data     []RowData    `json:"data,omitempty"`
	Rows     int          `json:"rows,omitempty"`
	RowsRead int          `json:"rows_read,omitempty"`
	Bytes    int          `json:"bytes,omitempty"`
}

type ColumnMeta struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type RowData map[string]interface{}

// FormatJSON formats response as JSONEachRow
func FormatJSON(columns []string, rows [][]interface{}) ([]byte, error) {
	var result []json.RawMessage

	for _, row := range rows {
		obj := make(map[string]interface{})
		for i, col := range columns {
			if i < len(row) {
				obj[col] = row[i]
			}
		}
		data, err := json.Marshal(obj)
		if err != nil {
			return nil, err
		}
		result = append(result, data)
	}

	return json.Marshal(result)
}
