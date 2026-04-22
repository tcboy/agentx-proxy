package postgresql

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/agentx-labs/agentx-proxy/internal/mysql"
)

// Catalog handles pg_catalog system table emulation
type Catalog struct {
	pool         *mysql.Pool
	colTypeCache map[colKey]uint32
}

type colKey struct {
	table string
	col   string
}

func NewCatalog(pool *mysql.Pool) *Catalog {
	return &Catalog{pool: pool, colTypeCache: make(map[colKey]uint32)}
}

// IsCatalogQuery checks if a query is targeting pg_catalog
func (c *Catalog) IsCatalogQuery(sql string) bool {
	upper := strings.ToUpper(sql)
	return strings.Contains(upper, "PG_CATALOG") ||
		strings.Contains(upper, "PG_TYPE") ||
		strings.Contains(upper, "PG_CLASS") ||
		strings.Contains(upper, "PG_ATTRIBUTE") ||
		strings.Contains(upper, "PG_NAMESPACE") ||
		strings.Contains(upper, "PG_INDEX") ||
		strings.Contains(upper, "PG_PROC") ||
		strings.Contains(upper, "PG_ENUM") ||
		strings.Contains(upper, "PG_DESCRIPTION") ||
		strings.Contains(upper, "PG_DATABASE") ||
		strings.Contains(upper, "PG_CONSTRAINT") ||
		strings.Contains(upper, "PG_EXTENSION") ||
		strings.Contains(upper, "PG_AM") ||
		strings.Contains(upper, "PG_SETTINGS") ||
		strings.Contains(upper, "PG_OPCLASS") ||
		strings.Contains(upper, "PG_OPERATOR") ||
		strings.Contains(upper, "PG_SECLABEL") ||
		strings.Contains(upper, "PG_TRIGGER") ||
		strings.Contains(upper, "PG_RANGE")
}

// HandleCatalogQuery intercepts and responds to pg_catalog queries
func (c *Catalog) HandleCatalogQuery(ctx context.Context, sql string) ([]string, [][]interface{}, error) {
	upper := strings.ToUpper(sql)

	// pg_type queries - return type mappings
	if strings.Contains(upper, "PG_TYPE") {
		return c.handlePGType(ctx, sql)
	}

	// pg_class queries - return table metadata
	if strings.Contains(upper, "PG_CLASS") {
		return c.handlePGClass(ctx, sql)
	}

	// pg_attribute queries - return column metadata
	if strings.Contains(upper, "PG_ATTRIBUTE") {
		return c.handlePGAttribute(ctx, sql)
	}

	// pg_namespace queries
	if strings.Contains(upper, "PG_NAMESPACE") {
		return c.handlePGNamespace(ctx, sql)
	}

	// pg_index queries
	if strings.Contains(upper, "PG_INDEX") {
		return c.handlePGIndex(ctx, sql)
	}

	// pg_proc queries
	if strings.Contains(upper, "PG_PROC") {
		return c.handlePGProc(ctx, sql)
	}

	// pg_enum queries
	if strings.Contains(upper, "PG_ENUM") {
		return c.handlePGEnum(ctx, sql)
	}

	// pg_constraint queries
	if strings.Contains(upper, "PG_CONSTRAINT") {
		return c.handlePGConstraint(ctx, sql)
	}

	// pg_description queries
	if strings.Contains(upper, "PG_DESCRIPTION") {
		return c.handlePGDescription(ctx, sql)
	}

	// pg_database queries
	if strings.Contains(upper, "PG_DATABASE") {
		return c.handlePGDatabase(ctx, sql)
	}

	// pg_extension
	if strings.Contains(upper, "PG_EXTENSION") {
		return c.handlePGExtension(ctx, sql)
	}

	// information_schema queries - pass through to MySQL
	if strings.Contains(upper, "INFORMATION_SCHEMA") {
		return nil, nil, nil // Signal to pass through
	}

	return nil, nil, fmt.Errorf("unsupported catalog query")
}

// handlePGType returns PG type OIDs mapped from MySQL types
func (c *Catalog) handlePGType(ctx context.Context, sql string) ([]string, [][]interface{}, error) {
	columns := []string{"oid", "typname", "typnamespace", "typtype", "typcategory",
		"typrelid", "typelem", "typarray", "typinput", "typoutput", "typreceive",
		"typsend", "typmod_in", "typmod_out", "typalign", "typstorage", "typnotnull",
		"typbasetype", "typtypmod", "typndims", "typcollation", "typdefaultbin", "typdefault", "typacl"}

	// Build a mapping of common PG types
	types := [][]interface{}{
		{16, "bool", 11, "b", "B", 0, 0, 1000, "boolin", "boolout", "-", "-", "-", "-", "c", "p", false, 0, -1, 0, 0, nil, nil, nil},
		{17, "bytea", 11, "b", "U", 0, 0, 1001, "byteain", "byteaout", "-", "-", "-", "-", "i", "x", false, 0, -1, 0, 0, nil, nil, nil},
		{18, "char", 11, "b", "C", 0, 0, 1002, "charin", "charout", "-", "-", "-", "-", "c", "p", false, 0, -1, 0, 0, nil, nil, nil},
		{19, "name", 11, "b", "C", 0, 0, 1003, "namein", "nameout", "-", "-", "-", "-", "c", "p", false, 0, -1, 0, 0, nil, nil, nil},
		{20, "int8", 11, "b", "N", 0, 0, 1016, "int8in", "int8out", "-", "-", "-", "-", "d", "p", false, 0, -1, 0, 0, nil, nil, nil},
		{21, "int2", 11, "b", "N", 0, 0, 1005, "int2in", "int2out", "-", "-", "-", "-", "s", "p", false, 0, -1, 0, 0, nil, nil, nil},
		{23, "int4", 11, "b", "N", 0, 0, 1007, "int4in", "int4out", "-", "-", "-", "-", "i", "p", false, 0, -1, 0, 0, nil, nil, nil},
		{25, "text", 11, "b", "S", 0, 0, 1009, "textin", "textout", "-", "-", "-", "-", "i", "x", false, 0, -1, 0, 0, nil, nil, nil},
		{26, "oid", 11, "b", "N", 0, 0, 1028, "oidin", "oidout", "-", "-", "-", "-", "i", "p", false, 0, -1, 0, 0, nil, nil, nil},
		{700, "float4", 11, "b", "N", 0, 0, 1021, "float4in", "float4out", "-", "-", "-", "-", "i", "p", false, 0, -1, 0, 0, nil, nil, nil},
		{701, "float8", 11, "b", "N", 0, 0, 1022, "float8in", "float8out", "-", "-", "-", "-", "d", "p", false, 0, -1, 0, 0, nil, nil, nil},
		{1043, "varchar", 11, "b", "C", 0, 0, 1015, "varcharin", "varcharout", "-", "-", "-", "-", "i", "x", false, 0, -1, 0, 0, nil, nil, nil},
		{1082, "date", 11, "b", "D", 0, 0, 1182, "date_in", "date_out", "-", "-", "-", "-", "d", "p", false, 0, -1, 0, 0, nil, nil, nil},
		{1083, "time", 11, "b", "D", 0, 0, 1183, "time_in", "time_out", "-", "-", "-", "-", "d", "p", false, 0, -1, 0, 0, nil, nil, nil},
		{1114, "timestamp", 11, "b", "D", 0, 0, 1115, "timestamp_in", "timestamp_out", "-", "-", "-", "-", "d", "p", false, 0, -1, 0, 0, nil, nil, nil},
		{1184, "timestamptz", 11, "b", "D", 0, 0, 1185, "timestamptz_in", "timestamptz_out", "-", "-", "-", "-", "d", "p", false, 0, -1, 0, 0, nil, nil, nil},
		{1700, "numeric", 11, "b", "N", 0, 0, 1231, "numeric_in", "numeric_out", "-", "-", "-", "-", "m", "m", false, 0, -1, 0, 0, nil, nil, nil},
		{2950, "uuid", 11, "b", "U", 0, 0, 2951, "uuid_in", "uuid_out", "-", "-", "-", "-", "c", "p", false, 0, -1, 0, 0, nil, nil, nil},
		{3802, "jsonb", 11, "b", "U", 0, 0, 3807, "jsonb_in", "jsonb_out", "-", "-", "-", "-", "i", "m", false, 0, -1, 0, 0, nil, nil, nil},
		{114, "json", 11, "b", "U", 0, 0, 199, "json_in", "json_out", "-", "-", "-", "-", "i", "m", false, 0, -1, 0, 0, nil, nil, nil},
		{2951, "_uuid", 11, "b", "A", 2950, 0, 0, "array_in", "array_out", "-", "-", "-", "-", "i", "m", false, 0, -1, 0, 0, nil, nil, nil},
		{1009, "_text", 11, "b", "A", 25, 0, 0, "array_in", "array_out", "-", "-", "-", "-", "i", "x", false, 0, -1, 0, 0, nil, nil, nil},
		{1007, "_int4", 11, "b", "A", 23, 0, 0, "array_in", "array_out", "-", "-", "-", "-", "i", "m", false, 0, -1, 0, 0, nil, nil, nil},
		{1016, "_int8", 11, "b", "A", 20, 0, 0, "array_in", "array_out", "-", "-", "-", "-", "d", "m", false, 0, -1, 0, 0, nil, nil, nil},
		{1000, "_bool", 11, "b", "A", 16, 0, 0, "array_in", "array_out", "-", "-", "-", "-", "i", "m", false, 0, -1, 0, 0, nil, nil, nil},
		{1005, "_int2", 11, "b", "A", 21, 0, 0, "array_in", "array_out", "-", "-", "-", "-", "s", "m", false, 0, -1, 0, 0, nil, nil, nil},
		{1002, "_char", 11, "b", "A", 18, 0, 0, "array_in", "array_out", "-", "-", "-", "-", "c", "p", false, 0, -1, 0, 0, nil, nil, nil},
	}

	// Filter based on the WHERE clause if present
	if strings.Contains(strings.ToUpper(sql), "WHERE") {
		// Try to extract OID conditions
		oidRe := regexp.MustCompile(`(?i)oid\s*(?:=|IN\s*\()?\s*(\d+)`)
		if m := oidRe.FindStringSubmatch(sql); len(m) > 1 {
			oid := 0
			fmt.Sscanf(m[1], "%d", &oid)
			var filtered [][]interface{}
			for _, t := range types {
				if t[0].(int) == oid {
					filtered = append(filtered, t)
				}
			}
			if len(filtered) > 0 {
				return columns, filtered, nil
			}
		}

		// Try to extract IN list
		inRe := regexp.MustCompile(`(?i)oid\s+IN\s*\(([^)]+)\)`)
		if m := inRe.FindStringSubmatch(sql); len(m) > 1 {
			oidStrs := strings.Split(m[1], ",")
			oids := make(map[int]bool)
			for _, s := range oidStrs {
				var oid int
				fmt.Sscanf(strings.TrimSpace(s), "%d", &oid)
				oids[oid] = true
			}
			var filtered [][]interface{}
			for _, t := range types {
				if oids[t[0].(int)] {
					filtered = append(filtered, t)
				}
			}
			if len(filtered) > 0 {
				return columns, filtered, nil
			}
		}

		// Try typname condition
		nameRe := regexp.MustCompile(`(?i)typname\s*=\s*'([^']+)'`)
		if m := nameRe.FindStringSubmatch(sql); len(m) > 1 {
			name := m[1]
			var filtered [][]interface{}
			for _, t := range types {
				if t[1].(string) == name {
					filtered = append(filtered, t)
				}
			}
			if len(filtered) > 0 {
				return columns, filtered, nil
			}
		}

		// Try typname IN (...) condition
		nameInRe := regexp.MustCompile(`(?i)typname\s+IN\s*\(([^)]+)\)`)
		if m := nameInRe.FindStringSubmatch(sql); len(m) > 1 {
			nameStrs := strings.Split(m[1], ",")
			names := make(map[string]bool)
			for _, s := range nameStrs {
				s = strings.TrimSpace(s)
				s = strings.Trim(s, "'")
				names[s] = true
			}
			var filtered [][]interface{}
			for _, t := range types {
				if names[t[1].(string)] {
					filtered = append(filtered, t)
				}
			}
			if len(filtered) > 0 {
				return columns, filtered, nil
			}
		}

		// WHERE clause present but no filter matched — return empty
		return columns, nil, nil
	}

	return columns, types, nil
}

func (c *Catalog) handlePGClass(ctx context.Context, query string) ([]string, [][]interface{}, error) {
	columns := []string{"oid", "relname", "relnamespace", "reltype", "reloftype",
		"relowner", "relam", "relfilenode", "reltablespace", "relpages", "reltuples",
		"relallvisible", "reltoastrelid", "relhasindex", "relisshared", "relpersistence",
		"relkind", "relnatts", "relchecks", "relhasoids", "relhaspkey", "relhasrules",
		"relhastriggers", "relhassubclass", "relrowsecurity", "relforcerowsecurity",
		"relispopulated", "relreplident", "relispartition"}

	// Get actual tables from MySQL
	rows, err := c.pool.Query(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var data [][]interface{}
	oid := 16384 // Start from a high OID
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		// Get column count
		colRows, err := c.pool.Query(ctx, "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ?", name)
		natts := 0
		if err == nil {
			colRows.Next()
			colRows.Scan(&natts)
			colRows.Close()
		}
		// Check for PK
		pkRows, err := c.pool.Query(ctx, "SELECT COUNT(*) FROM information_schema.table_constraints WHERE table_schema = DATABASE() AND table_name = ? AND constraint_type = 'PRIMARY KEY'", name)
		hasPK := false
		if err == nil {
			pkRows.Next()
			pkRows.Scan(&hasPK)
			pkRows.Close()
		}

		data = append(data, []interface{}{
			oid, name, 2200, 0, 0, 0, 0, 0, 0, 0, 0,
			0, 0, false, false, "p", "r", natts, 0, false,
			hasPK, false, false, false, false, false, true, "d", false,
		})
		oid++
	}

	return columns, data, nil
}

func (c *Catalog) handlePGAttribute(ctx context.Context, query string) ([]string, [][]interface{}, error) {
	columns := []string{"attrelid", "attname", "atttypid", "attstattarget", "attlen",
		"attnum", "attndims", "attcacheoff", "atttypmod", "attbyval", "attstorage",
		"attalign", "attnotnull", "atthasdef", "atthasmissing", "attidentity",
		"attgenerated", "attisdropped", "attislocal", "attinhcount", "attcollation",
		"attacl", "attoptions", "attfdwoptions"}

	// Extract table OID from query
	oidRe := regexp.MustCompile(`(?i)attrelid\s*=\s*(\d+)`)
	matches := oidRe.FindStringSubmatch(query)
	if len(matches) != 2 {
		// Could be a JOIN with pg_class - extract differently
		return columns, nil, nil
	}

	oid := 0
	fmt.Sscanf(matches[1], "%d", &oid)

	// Map OID to table name (reverse of handlePGClass)
	rows, err := c.pool.Query(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var tableName string
	currentOid := 16384
	for rows.Next() {
		var name string
		rows.Scan(&name)
		if currentOid == oid {
			tableName = name
			break
		}
		currentOid++
	}

	if tableName == "" {
		return columns, nil, nil
	}

	// Get columns
	colRows, err := c.pool.Query(ctx, `
		SELECT column_name, ordinal_position, is_nullable, column_type
		FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = ?
		ORDER BY ordinal_position
	`, tableName)
	if err != nil {
		return nil, nil, err
	}
	defer colRows.Close()

	var data [][]interface{}
	for colRows.Next() {
		var name string
		var pos int
		var nullable, colType string
		colRows.Scan(&name, &pos, &nullable, &colType)

		typeOid := 25 // text default
		switch {
		case strings.HasPrefix(colType, "int"):
			typeOid = 23
		case strings.HasPrefix(colType, "bigint"):
			typeOid = 20
		case strings.HasPrefix(colType, "smallint"):
			typeOid = 21
		case strings.HasPrefix(colType, "float"):
			typeOid = 701
		case strings.HasPrefix(colType, "double"):
			typeOid = 701
		case strings.HasPrefix(colType, "decimal"), strings.HasPrefix(colType, "numeric"):
			typeOid = 1700
		case colType == "json":
			typeOid = 114
		case strings.HasPrefix(colType, "datetime"):
			typeOid = 1114
		case strings.HasPrefix(colType, "varchar"):
			typeOid = 1043
		case colType == "tinyint(1)":
			typeOid = 16
		}

		data = append(data, []interface{}{
			oid, name, typeOid, -1, -1, int16(pos), 0, -1, -1,
			false, "x", "i", nullable == "NO", false, false, "",
			"", false, true, 0, 0, nil, nil, nil,
		})
	}

	return columns, data, nil
}

func (c *Catalog) handlePGNamespace(ctx context.Context, query string) ([]string, [][]interface{}, error) {
	columns := []string{"oid", "nspname", "nspowner", "nspacl"}

	namespaces := [][]interface{}{
		{11, "pg_catalog", 10, nil},
		{2200, "public", 10, nil},
		{13179, "pg_toast", 10, nil},
	}

	return columns, namespaces, nil
}

func (c *Catalog) handlePGIndex(ctx context.Context, query string) ([]string, [][]interface{}, error) {
	columns := []string{"indexrelid", "indrelid", "indnatts", "indnkeyatts", "indisunique",
		"indisprimary", "indisexclusion", "indimmediate", "indisclustered", "indisvalid",
		"indcheckxmin", "indisready", "indislive", "indisreplident", "indkey",
		"indcollation", "indclass", "indoption", "indexprs", "indpred"}

	// Get indexes from MySQL
	rows, err := c.pool.Query(ctx, `
		SELECT TABLE_NAME, INDEX_NAME, NON_UNIQUE, COLUMN_NAME, SEQ_IN_INDEX
		FROM information_schema.statistics
		WHERE table_schema = DATABASE()
		ORDER BY TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX
	`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var data [][]interface{}
	indexOid := 30000

	type idxInfo struct {
		oid      int
		tableOid int
		cols     []int
		unique   bool
		primary  bool
	}

	indexes := make(map[string]*idxInfo)
	tableOidMap := make(map[string]int)

	curTableOid := 16384
	tableRows, err := c.pool.Query(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'
	`)
	if err == nil {
		for tableRows.Next() {
			var name string
			tableRows.Scan(&name)
			tableOidMap[name] = curTableOid
			curTableOid++
		}
		tableRows.Close()
	}

	for rows.Next() {
		var tableName, index, col string
		var nonUnique int
		var seq int
		rows.Scan(&tableName, &index, &nonUnique, &col, &seq)

		tOid := tableOidMap[tableName]
		key := tableName + "." + index

		if _, ok := indexes[key]; !ok {
			indexes[key] = &idxInfo{
				oid:      indexOid,
				tableOid: tOid,
				unique:   nonUnique == 0,
				primary:  index == "PRIMARY",
			}
			indexOid++
		}
		indexes[key].cols = append(indexes[key].cols, seq)
	}

	for _, idx := range indexes {
		colStr := ""
		for i, c := range idx.cols {
			if i > 0 {
				colStr += " "
			}
			colStr += fmt.Sprintf("%d", c)
		}

		data = append(data, []interface{}{
			idx.oid, idx.tableOid, len(idx.cols), len(idx.cols),
			idx.unique, idx.primary, false, true, false, true,
			false, true, true, false, colStr, nil, nil, nil, nil, nil,
		})
	}

	return columns, data, nil
}

func (c *Catalog) handlePGProc(ctx context.Context, query string) ([]string, [][]interface{}, error) {
	columns := []string{"oid", "proname", "pronamespace", "proowner", "prolang", "procost",
		"prorows", "provariadic", "prosupport", "prokind", "prosecdef", "proleakproof",
		"proisstrict", "proretset", "provolatile", "proparallel", "pronargs",
		"pronargdefaults", "prorettype", "proargtypes", "proallargtypes", "proargmodes",
		"proargnames", "proargdefaults", "protrftypes", "prosrc", "probin", "proconfig", "proacl"}

	// Return commonly needed functions
	procs := [][]interface{}{
		{2100, "version", 11, 10, 0, 1, 0, 0, nil, "f", false, false, false, false, "s", "s", int16(0), int16(0), 25, nil, nil, nil, nil, nil, nil, "SELECT VERSION()", nil, nil, nil},
		{2101, "current_database", 11, 10, 0, 1, 1, 0, nil, "f", false, false, false, false, "s", "s", int16(0), int16(0), 25, nil, nil, nil, nil, nil, nil, "SELECT DATABASE()", nil, nil, nil},
		{2102, "current_user", 11, 10, 0, 1, 1, 0, nil, "f", false, false, false, false, "s", "s", int16(0), int16(0), 25, nil, nil, nil, nil, nil, nil, "SELECT CURRENT_USER()", nil, nil, nil},
		{2103, "current_schema", 11, 10, 0, 1, 1, 0, nil, "f", false, false, false, false, "s", "s", int16(0), int16(0), 25, nil, nil, nil, nil, nil, nil, "SELECT 'public'", nil, nil, nil},
		{2104, "pg_get_expr", 11, 10, 0, 1, 0, 0, nil, "f", false, false, false, false, "s", "s", int16(2), int16(0), 25, nil, nil, nil, nil, nil, nil, "pg_get_expr", nil, nil, nil},
		{2105, "pg_get_userbyid", 11, 10, 0, 1, 1, 0, nil, "f", false, false, false, false, "s", "s", int16(1), int16(0), 25, nil, nil, nil, nil, nil, nil, "pg_get_userbyid", nil, nil, nil},
		{2106, "format_type", 11, 10, 0, 1, 1, 0, nil, "f", false, false, false, false, "s", "s", int16(2), int16(0), 25, nil, nil, nil, nil, nil, nil, "format_type", nil, nil, nil},
		{2107, "obj_description", 11, 10, 0, 1, 1, 0, nil, "f", false, false, false, false, "s", "s", int16(1), int16(0), 25, nil, nil, nil, nil, nil, nil, "obj_description", nil, nil, nil},
		{2108, "pg_get_constraintdef", 11, 10, 0, 1, 1, 0, nil, "f", false, false, false, false, "s", "s", int16(1), int16(0), 25, nil, nil, nil, nil, nil, nil, "pg_get_constraintdef", nil, nil, nil},
		{2109, "pg_get_indexdef", 11, 10, 0, 1, 1, 0, nil, "f", false, false, false, false, "s", "s", int16(1), int16(0), 25, nil, nil, nil, nil, nil, nil, "pg_get_indexdef", nil, nil, nil},
	}

	return columns, procs, nil
}

func (c *Catalog) handlePGEnum(ctx context.Context, query string) ([]string, [][]interface{}, error) {
	columns := []string{"oid", "enumtypid", "enumsortorder", "enumlabel"}

	// Return Langfuse enum types
	enums := [][]interface{}{
		// Role enum
		{50001, 40001, 1.0, "OWNER"},
		{50002, 40001, 2.0, "ADMIN"},
		{50003, 40001, 3.0, "MEMBER"},
		{50004, 40001, 4.0, "VIEWER"},
		{50005, 40001, 5.0, "NONE"},
		// ApiKeyScope Enum
		{50011, 40002, 1.0, "ORGANIZATION"},
		{50012, 40002, 2.0, "PROJECT"},
		// DatasetStatus Enum
		{50021, 40003, 1.0, "ACTIVE"},
		{50022, 40003, 2.0, "ARCHIVED"},
		// CommentObjectType Enum
		{50031, 40004, 1.0, "TRACE"},
		{50032, 40004, 2.0, "OBSERVATION"},
		{50033, 40004, 3.0, "SESSION"},
		{50034, 40004, 4.0, "PROMPT"},
		// NotificationChannel Enum
		{50041, 40005, 1.0, "EMAIL"},
		// NotificationType Enum
		{50046, 40006, 1.0, "COMMENT_MENTION"},
		// ActionType Enum
		{50051, 40007, 1.0, "WEBHOOK"},
		{50052, 40007, 2.0, "SLACK"},
		{50053, 40007, 3.0, "GITHUB_DISPATCH"},
		// ActionExecutionStatus Enum
		{50061, 40008, 1.0, "COMPLETED"},
		{50062, 40008, 2.0, "ERROR"},
		{50063, 40008, 3.0, "PENDING"},
		{50064, 40008, 4.0, "CANCELLED"},
		{50065, 40008, 5.0, "DELAYED"},
		// ScoreDataType Enum
		{50071, 40009, 1.0, "NUMERIC"},
		{50072, 40009, 2.0, "BOOLEAN"},
		{50073, 40009, 3.0, "CATEGORICAL"},
		// AnnotationQueueStatus Enum
		{50081, 40010, 1.0, "PENDING"},
		{50082, 40010, 2.0, "COMPLETED"},
		{50083, 40010, 3.0, "SKIPPED"},
		// AnnotationQueueObjectType Enum
		{50091, 40011, 1.0, "TRACE"},
		{50092, 40011, 2.0, "OBSERVATION"},
		{50093, 40011, 3.0, "SESSION"},
		// JobType Enum
		{50101, 40012, 1.0, "EVAL"},
		// JobConfigState Enum
		{50111, 40013, 1.0, "ACTIVE"},
		{50112, 40013, 2.0, "INACTIVE"},
		// JobExecutionStatus Enum
		{50121, 40014, 1.0, "COMPLETED"},
		{50122, 40014, 2.0, "ERROR"},
		{50123, 40014, 3.0, "PENDING"},
		{50124, 40014, 4.0, "CANCELLED"},
		{50125, 40014, 5.0, "DELAYED"},
		// SurveyName Enum
		{50131, 40015, 1.0, "FEEDBACK"},
		{50132, 40015, 2.0, "SATISFACTION"},
		// LegacyPrismaObservationType Enum
		{50141, 40016, 1.0, "SPAN"},
		{50142, 40016, 2.0, "EVENT"},
		{50143, 40016, 3.0, "GENERATION"},
		{50144, 40016, 4.0, "AGENT"},
		{50145, 40016, 5.0, "TOOL"},
		{50146, 40016, 6.0, "CHAIN"},
		{50147, 40016, 7.0, "RETRIEVER"},
		{50148, 40016, 8.0, "EVALUATOR"},
		{50149, 40016, 9.0, "EMBEDDING"},
		{50150, 40016, 10.0, "GUARDRAIL"},
		// LegacyPrismaObservationLevel Enum
		{50151, 40017, 1.0, "DEBUG"},
		{50152, 40017, 2.0, "DEFAULT"},
		{50153, 40017, 3.0, "WARNING"},
		{50154, 40017, 4.0, "ERROR"},
		// LegacyPrismaScoreSource Enum
		{50161, 40018, 1.0, "ANNOTATION"},
		{50162, 40018, 2.0, "API"},
		{50163, 40018, 3.0, "EVAL"},
		// EvaluatorBlockReason Enum
		{50171, 40019, 1.0, "SCORE_THRESHOLD"},
		{50172, 40019, 2.0, "MODEL_RESPONSE"},
		{50173, 40019, 3.0, "TIMEOUT"},
		{50174, 40019, 4.0, "ERROR"},
		{50175, 40019, 5.0, "MANUAL"},
		{50176, 40019, 6.0, "OTHER"},
		// BlobStorageIntegrationType Enum
		{50181, 40020, 1.0, "S3"},
		{50182, 40020, 2.0, "S3_COMPATIBLE"},
		{50183, 40020, 3.0, "AZURE_BLOB_STORAGE"},
		// BlobStorageExportMode Enum
		{50191, 40021, 1.0, "FULL_HISTORY"},
		{50192, 40021, 2.0, "FROM_TODAY"},
		{50193, 40021, 3.0, "FROM_CUSTOM_DATE"},
		// BlobStorageIntegrationFileType Enum
		{50201, 40022, 1.0, "JSON"},
		{50202, 40022, 2.0, "CSV"},
		{50203, 40022, 3.0, "JSONL"},
		// AnalyticsIntegrationExportSource Enum
		{50211, 40023, 1.0, "TRACES_OBSERVATIONS"},
		{50212, 40023, 2.0, "TRACES_OBSERVATIONS_EVENTS"},
		{50213, 40023, 3.0, "EVENTS"},
	}

	// Filter by enumtypid if specified
	typidRe := regexp.MustCompile(`(?i)enumtypid\s*=\s*(\d+)`)
	if m := typidRe.FindStringSubmatch(query); len(m) > 1 {
		var typid int
		fmt.Sscanf(m[1], "%d", &typid)
		var filtered [][]interface{}
		for _, e := range enums {
			if e[1].(int) == typid {
				filtered = append(filtered, e)
			}
		}
		return columns, filtered, nil
	}

	return columns, enums, nil
}

func (c *Catalog) handlePGConstraint(ctx context.Context, query string) ([]string, [][]interface{}, error) {
	columns := []string{"oid", "conname", "connamespace", "contype", "condeferrable",
		"condeferred", "convalidated", "conrelid", "contypid", "conindid", "conparentid",
		"confrelid", "confupdtype", "confdeltype", "confmatchtype", "conislocal",
		"coninhcount", "connoinherit", "conkey", "confkey", "conpfeqop", "conppeqop",
		"conffeqop", "conexclop", "conbin"}

	return columns, nil, nil
}

func (c *Catalog) handlePGDescription(ctx context.Context, query string) ([]string, [][]interface{}, error) {
	columns := []string{"objoid", "classoid", "objsubid", "description"}
	return columns, nil, nil
}

func (c *Catalog) handlePGDatabase(ctx context.Context, query string) ([]string, [][]interface{}, error) {
	columns := []string{"oid", "datname", "datdba", "encoding", "datcollate",
		"datctype", "datistemplate", "datallowconn", "datconnlimit",
		"datfrozenxid", "datminmxid", "dattablespace"}

	databases := [][]interface{}{
		{1, "langfuse", 10, 6, "en_US.UTF-8", "en_US.UTF-8", false, true, -1, 0, 1, 1663},
	}

	return columns, databases, nil
}

func (c *Catalog) handlePGExtension(ctx context.Context, query string) ([]string, [][]interface{}, error) {
	columns := []string{"oid", "extname", "extowner", "extnamespace", "extrelocatable",
		"extversion", "extconfig", "extcondition"}

	// Return common extensions that Langfuse might check for
	extensions := [][]interface{}{
		{100, "plpgsql", 10, 11, false, "1.0", nil, nil},
	}

	return columns, extensions, nil
}

// GetColumnPGOID returns the PostgreSQL OID for a column in a MySQL table.
// Used to infer parameter types for INSERT queries.
func (c *Catalog) GetColumnPGOID(tableName, colName string) uint32 {
	key := colKey{table: strings.ToLower(tableName), col: strings.ToLower(colName)}
	if oid, ok := c.colTypeCache[key]; ok {
		return oid
	}

	ctx := context.Background()
	var dataType string
	err := c.pool.QueryRow(ctx,
		"SELECT DATA_TYPE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?",
		tableName, colName,
	).Scan(&dataType)
	if err != nil {
		return 0
	}

	oid := mysqlDataTypeToPGOID(dataType)
	c.colTypeCache[key] = oid
	return oid
}

func mysqlDataTypeToPGOID(dt string) uint32 {
	switch strings.ToUpper(dt) {
	case "TINYINT":
		return 21 // int2
	case "SMALLINT":
		return 21 // int2
	case "INT", "INTEGER", "MEDIUMINT":
		return 23 // int4
	case "BIGINT":
		return 20 // int8
	case "FLOAT":
		return 700 // float4
	case "DOUBLE":
		return 701 // float8
	case "DECIMAL", "NUMERIC":
		return 25 // text
	case "DATE":
		return 1082 // date
	case "TIME":
		return 1083 // time
	case "DATETIME", "TIMESTAMP":
		return 25 // text
	case "VARCHAR", "CHAR", "TEXT", "TINYTEXT", "MEDIUMTEXT", "LONGTEXT":
		return 25 // text
	case "JSON":
		return 1009 // text[] — Prisma stores String[] as MySQL JSON
	case "BLOB", "TINYBLOB", "MEDIUMBLOB", "LONGBLOB", "BINARY", "VARBINARY":
		return 17 // bytea
	default:
		return 25 // text
	}
}
