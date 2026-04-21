package postgresql

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"
)

// Translator handles PG SQL -> MySQL SQL translation
type Translator struct {
	arrayColumnMode string
	fulltextMode    string
}

func NewTranslator(arrayColumnMode, fulltextMode string) *Translator {
	return &Translator{
		arrayColumnMode: arrayColumnMode,
		fulltextMode:    fulltextMode,
	}
}

// Translate translates a PostgreSQL SQL query to MySQL
func (t *Translator) Translate(sql string) (string, error) {
	result := sql

	result = t.translateTypeCasts(result)
	result = t.translateILIKE(result)
	result = t.translateReturning(result)
	result = t.translateOnConflict(result)
	result = t.translateDateTrunc(result)
	result = t.translateExtractEpoch(result)
	result = t.translateIntervalArith(result)
	result = t.translateGenerateSeries(result)
	result = t.translateJSONBFunctions(result)
	result = t.translateArrayFunctions(result)
	result = t.translateLimit1By(result)
	result = t.translateLateralJoin(result)
	result = t.translateToTsVector(result)
	result = t.translateAnyArray(result)
	result = t.translateFinalKeyword(result)
	result = t.translateMapAccess(result)

	return result, nil
}

func (t *Translator) translateTypeCasts(sql string) string {
	re := regexp.MustCompile(`::("([^"]+)"|'([^']+)')`)
	sql = re.ReplaceAllString(sql, "")

	re = regexp.MustCompile(`::(\w+)`)
	sql = re.ReplaceAllString(sql, "")

	castRe := regexp.MustCompile(`CAST\(([^)]+)\s+AS\s+(?:text|varchar[^)]*|integer|int|bigint|boolean|timestamp[^)]*|date|numeric[^)]*|uuid)\)`)
	sql = castRe.ReplaceAllString(sql, "$1")

	return sql
}

func (t *Translator) translateILIKE(sql string) string {
	re := regexp.MustCompile(`(?i)\bNOT\s+ILIKE\b`)
	sql = re.ReplaceAllString(sql, "NOT LIKE COLLATE utf8mb4_general_ci")

	re = regexp.MustCompile(`(?i)\bILIKE\b`)
	sql = re.ReplaceAllString(sql, "LIKE COLLATE utf8mb4_general_ci")

	return sql
}

func (t *Translator) translateReturning(sql string) string {
	re := regexp.MustCompile(`(?i)^(\s*(?:INSERT|UPDATE|DELETE)\b.+?)\s+RETURNING\s+(.+)$`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) != 3 {
		return sql
	}

	mainSQL := matches[1]
	returningCols := matches[2]

	upper := strings.ToUpper(strings.TrimSpace(mainSQL))
	if strings.HasPrefix(upper, "INSERT") {
		cols := parseColumns(returningCols)
		if len(cols) == 1 && (strings.EqualFold(cols[0], "id") || strings.HasSuffix(strings.ToLower(cols[0]), "_id")) {
			tmpTable := fmt.Sprintf("_ret_%d", rand.Intn(100000))
			return fmt.Sprintf(
				"CREATE TEMPORARY TABLE IF NOT EXISTS %s (id VARCHAR(36)); %s; INSERT INTO %s VALUES (LAST_INSERT_ID()); SELECT id FROM %s; DROP TEMPORARY TABLE %s",
				tmpTable, mainSQL, tmpTable, tmpTable, tmpTable,
			)
		}
		return wrapReturningSelect(mainSQL, returningCols)
	}

	return wrapReturningSelect(mainSQL, returningCols)
}

func (t *Translator) translateOnConflict(sql string) string {
	// Check if this is an ON CONFLICT DO UPDATE
	if !strings.Contains(strings.ToUpper(sql), "ON CONFLICT") {
		return sql
	}

	// ON CONFLICT ... DO UPDATE SET ... -> ON DUPLICATE KEY UPDATE
	// Match ON CONFLICT (...) DO UPDATE SET ... to the end
	re := regexp.MustCompile(`(?i)\s*ON\s+CONFLICT\s*\(([^)]+)\)\s*DO\s+UPDATE\s+SET\s+(.+)`)
	subMatches := re.FindStringSubmatch(sql)
	if len(subMatches) == 3 {
		conflictCols := subMatches[1]
		updateClause := subMatches[2]
		_ = conflictCols

		// Convert excluded. references to VALUES()
		updateClause = regexp.MustCompile(`(?i)(\w+)\.(\w+)`).ReplaceAllStringFunc(updateClause, func(m string) string {
			parts := strings.Split(m, ".")
			if len(parts) == 2 && strings.EqualFold(parts[0], "excluded") {
				return "VALUES(" + parts[1] + ")"
			}
			return m
		})

		// Remove the ON CONFLICT part and replace
		sql = re.ReplaceAllString(sql, " ON DUPLICATE KEY UPDATE "+updateClause)
		return sql
	}

	// ON CONFLICT DO NOTHING -> INSERT IGNORE
	re = regexp.MustCompile(`(?i)\s*ON\s+CONFLICT\b.+?DO\s+NOTHING`)
	if re.MatchString(sql) {
		sql = strings.Replace(sql, "INSERT ", "INSERT IGNORE ", 1)
		sql = re.ReplaceAllString(sql, "")
	}

	return sql
}

func (t *Translator) translateDateTrunc(sql string) string {
	re := regexp.MustCompile(`(?i)date_trunc\s*\(\s*'(\w+)'\s*,\s*([^)]+)\s*\)`)
	sql = re.ReplaceAllStringFunc(sql, func(match string) string {
		subMatches := re.FindStringSubmatch(match)
		if len(subMatches) != 3 {
			return match
		}
		unit := strings.ToLower(subMatches[1])
		col := subMatches[2]
		switch unit {
		case "second":
			return fmt.Sprintf("DATE_FORMAT(%s, '%%Y-%%m-%%d %%H:%%i:%%s')", col)
		case "minute":
			return fmt.Sprintf("DATE_FORMAT(%s, '%%Y-%%m-%%d %%H:%%i:00')", col)
		case "hour":
			return fmt.Sprintf("DATE_FORMAT(%s, '%%Y-%%m-%%d %%H:00:00')", col)
		case "day":
			return fmt.Sprintf("DATE(%s)", col)
		case "week":
			return fmt.Sprintf("DATE_SUB(%s, INTERVAL WEEKDAY(%s) DAY)", col, col)
		case "month":
			return fmt.Sprintf("DATE_FORMAT(%s, '%%Y-%%m-01')", col)
		case "quarter":
			return fmt.Sprintf("MAKEDATE(YEAR(%s), 1) + INTERVAL (QUARTER(%s)-1)*3 MONTH", col, col)
		case "year":
			return fmt.Sprintf("MAKEDATE(YEAR(%s), 1)", col)
		default:
			return fmt.Sprintf("DATE(%s)", col)
		}
	})
	return sql
}

func (t *Translator) translateExtractEpoch(sql string) string {
	re := regexp.MustCompile(`(?i)EXTRACT\s*\(\s*EPOCH\s+FROM\s+([^)]+)\s*\)`)
	sql = re.ReplaceAllString(sql, "UNIX_TIMESTAMP($1)")
	return sql
}

func (t *Translator) translateIntervalArith(sql string) string {
	re := regexp.MustCompile(`INTERVAL\s+'(\d+)'\s+(\w+)`)
	sql = re.ReplaceAllString(sql, "INTERVAL $1 $2")
	return sql
}

func (t *Translator) translateGenerateSeries(sql string) string {
	re := regexp.MustCompile(`(?i)GENERATE_SERIES\s*\(\s*([^,]+)\s*,\s*([^,]+)\s*(?:,\s*([^)]+)\s*)?\)`)
	sql = re.ReplaceAllStringFunc(sql, func(match string) string {
		subMatches := re.FindStringSubmatch(match)
		if len(subMatches) < 3 {
			return match
		}
		start := strings.TrimSpace(subMatches[1])
		end := strings.TrimSpace(subMatches[2])
		step := "1"
		if len(subMatches) > 3 && subMatches[3] != "" {
			step = strings.TrimSpace(subMatches[3])
		}
		cteName := fmt.Sprintf("gs_%d", rand.Intn(100000))
		return fmt.Sprintf(
			"(WITH RECURSIVE %s(n) AS (SELECT %s UNION ALL SELECT n + %s FROM %s WHERE n + %s <= %s) SELECT n FROM %s)",
			cteName, start, step, cteName, step, end, cteName,
		)
	})
	return sql
}

func (t *Translator) translateJSONBFunctions(sql string) string {
	replacements := []struct {
		pattern string
		repl    string
	}{
		{`(?i)jsonb_set\s*\(`, "JSON_SET("},
		{`(?i)jsonb_agg\s*\(`, "JSON_ARRAYAGG("},
		{`(?i)jsonb_object_agg\s*\(`, "JSON_OBJECTAGG("},
		{`(?i)jsonb_build_object\s*\(`, "JSON_OBJECT("},
		{`(?i)jsonb_build_array\s*\(`, "JSON_ARRAY("},
		{`(?i)jsonb_array_length\s*\(`, "JSON_LENGTH("},
		{`(?i)jsonb_typeof\s*\(`, "JSON_TYPE("},
	}

	for _, r := range replacements {
		re := regexp.MustCompile(r.pattern)
		sql = re.ReplaceAllString(sql, r.repl)
	}

	// jsonb_array_elements -> JSON_TABLE
	re := regexp.MustCompile(`(?i)jsonb_array_elements\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "JSON_TABLE($1, '$[*]' COLUMNS (value TEXT PATH '$'))")

	// ->> operator
	re = regexp.MustCompile(`(\w+)\s*->>\s*'([^']+)'`)
	sql = re.ReplaceAllString(sql, "JSON_UNQUOTE(JSON_EXTRACT($1, '$.$2'))")

	// -> operator
	re = regexp.MustCompile(`(\w+)\s*->\s*'([^']+)'`)
	sql = re.ReplaceAllString(sql, "JSON_EXTRACT($1, '$.$2')")

	return sql
}

func (t *Translator) translateArrayFunctions(sql string) string {
	// 'x' = ANY(col) -> JSON_CONTAINS(col, '"x"')
	re := regexp.MustCompile(`(?i)'([^']*)'\s*=\s*ANY\s*\((\w+)\)`)
	sql = re.ReplaceAllString(sql, "JSON_CONTAINS($2, '\"$1\"')")

	// col && $1 -> JSON_OVERLAPS
	re = regexp.MustCompile(`(?i)(\w+)\s*&&\s*(\$\d+)`)
	sql = re.ReplaceAllString(sql, "JSON_OVERLAPS($1, $2)")

	// cardinality -> JSON_LENGTH
	re = regexp.MustCompile(`(?i)cardinality\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "JSON_LENGTH($1)")

	// unnest -> JSON_TABLE
	re = regexp.MustCompile(`(?i)unnest\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "JSON_TABLE($1, '$[*]' COLUMNS (value TEXT PATH '$'))")

	return sql
}

func (t *Translator) translateLimit1By(sql string) string {
	re := regexp.MustCompile(`(?i)\s+LIMIT\s+\d+\s+BY\s+[^\s;]+`)
	sql = re.ReplaceAllString(sql, "")
	return sql
}

func (t *Translator) translateLateralJoin(sql string) string {
	re := regexp.MustCompile(`(?i)(LEFT\s+)?JOIN\s+LATERAL\s+\((.+?)\)\s+(\w+)\s+ON\s+(.+?)(?=\s+(?:LEFT|RIGHT|INNER|JOIN|WHERE|GROUP|ORDER|HAVING|LIMIT|UNION|$))`)
	sql = re.ReplaceAllStringFunc(sql, func(match string) string {
		subMatches := re.FindStringSubmatch(match)
		if len(subMatches) < 5 {
			return match
		}
		joinType := subMatches[1]
		subquery := subMatches[2]
		alias := subMatches[3]
		onClause := subMatches[4]
		newSubquery := fmt.Sprintf("(%s ORDER BY 1 LIMIT 1) AS %s", subquery, alias)
		if joinType != "" {
			return fmt.Sprintf("LEFT JOIN %s ON %s", newSubquery, onClause)
		}
		return fmt.Sprintf("JOIN %s ON %s", newSubquery, onClause)
	})
	return sql
}

func (t *Translator) translateToTsVector(sql string) string {
	if t.fulltextMode == "like" {
		re := regexp.MustCompile(`(?i)to_tsvector\s*\([^,]*,\s*([^)]+)\)`)
		sql = re.ReplaceAllString(sql, "$1")
		re = regexp.MustCompile(`(?i)plainto_tsquery\s*\([^,]*,\s*'([^']+)'\)`)
		sql = re.ReplaceAllString(sql, "'%$1%'")
		re = regexp.MustCompile(`(?i)(\w+)\s*@@\s*'%'([^']+)'%'`)
		sql = re.ReplaceAllString(sql, "$1 LIKE '%$2%'")
	} else {
		re := regexp.MustCompile(`(?i)to_tsvector\s*\([^,]*,\s*([^)]+)\)\s*@@\s*plainto_tsquery\s*\([^,]*,\s*'([^']+)'\)`)
		sql = re.ReplaceAllString(sql, "MATCH($1) AGAINST('$2' IN BOOLEAN MODE)")
	}
	return sql
}

func (t *Translator) translateAnyArray(sql string) string {
	re := regexp.MustCompile(`(?i)(\w+)\s*&&\s*ARRAY\[([^\]]*)\]`)
	sql = re.ReplaceAllStringFunc(sql, func(match string) string {
		subMatches := re.FindStringSubmatch(match)
		if len(subMatches) != 3 {
			return match
		}
		elements := subMatches[2]
		parts := strings.Split(elements, ",")
		for i, p := range parts {
			p = strings.TrimSpace(p)
			if !strings.HasPrefix(p, "'") {
				parts[i] = "'" + p + "'"
			}
		}
		return fmt.Sprintf("JSON_OVERLAPS(%s, '[%s]')", subMatches[1], strings.Join(parts, ","))
	})

	re = regexp.MustCompile(`(?i)(\w+)\s*@>\s*ARRAY\[([^\]]*)\]`)
	sql = re.ReplaceAllStringFunc(sql, func(match string) string {
		subMatches := re.FindStringSubmatch(match)
		if len(subMatches) != 3 {
			return match
		}
		elements := subMatches[2]
		parts := strings.Split(elements, ",")
		for i, p := range parts {
			p = strings.TrimSpace(p)
			if !strings.HasPrefix(p, "'") {
				parts[i] = "'" + p + "'"
			}
		}
		return fmt.Sprintf("JSON_CONTAINS(%s, '[%s]')", subMatches[1], strings.Join(parts, ","))
	})

	return sql
}

func (t *Translator) translateFinalKeyword(sql string) string {
	re := regexp.MustCompile(`(?i)\bFINAL\b`)
	return re.ReplaceAllString(sql, "")
}

func (t *Translator) translateMapAccess(sql string) string {
	re := regexp.MustCompile(`(\w+)\['([^']+)'\]`)
	sql = re.ReplaceAllString(sql, "JSON_UNQUOTE(JSON_EXTRACT($1, '$.$2'))")

	re = regexp.MustCompile(`(\w+)\["([^"]+)"\]`)
	sql = re.ReplaceAllString(sql, "JSON_UNQUOTE(JSON_EXTRACT($1, '$.$2'))")

	return sql
}

func parseColumns(cols string) []string {
	parts := strings.Split(cols, ",")
	result := make([]string, len(parts))
	for i, p := range parts {
		result[i] = strings.TrimSpace(p)
	}
	return result
}

func wrapReturningSelect(mainSQL, returningCols string) string {
	tmpTable := fmt.Sprintf("_ret_%d", time.Now().UnixNano()%1000000)
	return fmt.Sprintf(
		"CREATE TEMPORARY TABLE IF NOT EXISTS %s AS %s; SELECT %s FROM %s",
		tmpTable, mainSQL, returningCols, tmpTable,
	)
}

// TranslateDDL translates PG DDL to MySQL DDL
func (t *Translator) TranslateDDL(sql string) string {
	result := sql

	result = regexp.MustCompile(`(?i)\bSERIAL\b`).ReplaceAllString(result, "BIGINT AUTO_INCREMENT")
	result = regexp.MustCompile(`(?i)\bTEXT\[\]`).ReplaceAllString(result, "JSON")
	result = regexp.MustCompile(`(?i)\bVARCHAR\s*\(\s*\d+\s*\)\[\]`).ReplaceAllString(result, "JSON")
	result = regexp.MustCompile(`(?i)\bINT\[\]`).ReplaceAllString(result, "JSON")
	result = regexp.MustCompile(`(?i)\bINTEGER\[\]`).ReplaceAllString(result, "JSON")
	result = regexp.MustCompile(`(?i)\bJSONB\b`).ReplaceAllString(result, "JSON")
	result = regexp.MustCompile(`(?i)\bUUID\b`).ReplaceAllString(result, "VARCHAR(36)")
	result = regexp.MustCompile(`(?i)\bTIMESTAMP\s*WITH\s*TIME\s*ZONE\b`).ReplaceAllString(result, "DATETIME(3)")
	result = regexp.MustCompile(`(?i)\bTIMESTAMPTZ\b`).ReplaceAllString(result, "DATETIME(3)")
	result = regexp.MustCompile(`(?i)\bBOOLEAN\b`).ReplaceAllString(result, "TINYINT(1)")
	result = regexp.MustCompile(`(?i)\bGENERATED\s+ALWAYS\s+AS\s+IDENTITY\b`).ReplaceAllString(result, "AUTO_INCREMENT")
	result = regexp.MustCompile(`(?i)\bUSING\s+GIN\b`).ReplaceAllString(result, "")

	return result
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
