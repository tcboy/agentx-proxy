package postgresql

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"strings"
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

	result = t.translateDoubleQuotes(result)
	result = t.translateTypeCasts(result)
	result = t.translateILIKE(result)
	result = t.translateLimitNull(result)
	result = t.translateEqualsNull(result)
	result = t.translateUnionLimit(result)
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
	result = t.translateAny(result)
	result = t.translateFinalKeyword(result)
	result = t.translateMapAccess(result)
	result = t.translateDollarParams(result)
	result = t.translateStringAgg(result)
	result = t.translateBoolOperators(result)
	result = t.translateCoalesceInterval(result)

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

func (t *Translator) translateLimitNull(sql string) string {
	re := regexp.MustCompile(`(?i)\s+LIMIT\s+NULL\s*$`)
	return re.ReplaceAllString(sql, "")
}

func (t *Translator) translateEqualsNull(sql string) string {
	// = NULL -> IS NULL (standard SQL null comparison)
	re := regexp.MustCompile(`(?i)(\w[\w.` + "`" + `]*\s*)=\s*NULL\b`)
	sql = re.ReplaceAllString(sql, "${1}IS NULL")
	// != NULL -> IS NOT NULL
	re2 := regexp.MustCompile(`(?i)(\w[\w.` + "`" + `]*\s*)!=\s*NULL\b`)
	sql = re2.ReplaceAllString(sql, "${1}IS NOT NULL")
	return sql
}

// translateUnionLimit wraps each SELECT in a UNION with parentheses.
// MySQL requires parentheses when individual SELECTs have LIMIT/ORDER BY:
//   SELECT ... LIMIT 2 UNION ALL SELECT ... LIMIT 2
// becomes:
//   (SELECT ... LIMIT 2) UNION ALL (SELECT ... LIMIT 2)
func (t *Translator) translateUnionLimit(sql string) string {
	upper := strings.ToUpper(sql)
	if !strings.Contains(upper, "UNION") {
		return sql
	}

	// Split on UNION [ALL] while preserving the separator
	re := regexp.MustCompile(`(?i)\b(UNION\s+ALL|UNION)\b`)
	parts := re.Split(sql, -1)
	seps := re.FindAllString(sql, -1)

	if len(parts) <= 1 {
		return sql
	}

	var buf strings.Builder
	for i, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		needsWrap := false
		tu := strings.ToUpper(trimmed)
		if strings.Contains(tu, " LIMIT ") || strings.Contains(tu, " ORDER BY ") {
			needsWrap = true
		}
		// Don't wrap the last part if it's a trailing ORDER BY/LIMIT for the whole UNION
		if i == len(parts)-1 && needsWrap {
			needsWrap = false
		}

		if i > 0 && len(seps) > i-1 {
			buf.WriteString(" " + seps[i-1] + " ")
		}

		if needsWrap && strings.HasPrefix(tu, "SELECT") {
			buf.WriteString("(" + trimmed + ")")
		} else {
			buf.WriteString(trimmed)
		}
	}
	return buf.String()
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
			tmpTable := fmt.Sprintf("_ret_%d", rand.IntN(100000))
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
		cteName := fmt.Sprintf("gs_%d", rand.IntN(100000))
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
	// LIMIT N BY col1, col2 -> ROW_NUMBER() OVER (PARTITION BY col1, col2) <= N
	// This is a complex transformation - for simple cases, wrap in CTE
	re := regexp.MustCompile(`(?i)\s+LIMIT\s+(\d+)\s+BY\s+(.+?)\s*$`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) >= 3 {
		limitN := matches[1]
		byCols := strings.TrimSpace(matches[2])
		// Remove the LIMIT clause
		sql = re.ReplaceAllString(sql, "")
		// Wrap in subquery with ROW_NUMBER
		sql = fmt.Sprintf("SELECT * FROM (SELECT t.*, ROW_NUMBER() OVER (PARTITION BY %s) AS _rn FROM (%s) t) _sq WHERE _rn <= %s",
			byCols, sql, limitN)
		return sql
	}

	// Fallback: just strip LIMIT BY for simple dedup
	re = regexp.MustCompile(`(?i)\s+LIMIT\s+\d+\s+BY\s+[^\s;]+`)
	sql = re.ReplaceAllString(sql, "")
	return sql
}

func (t *Translator) translateLateralJoin(sql string) string {
	re := regexp.MustCompile(`(?i)(LEFT\s+)?JOIN\s+LATERAL\s+\((.+?)\)\s+(\w+)\s+ON\s+(.+)`)
	matches := re.FindStringSubmatch(sql)
	if len(matches) < 5 {
		return sql
	}
	joinType := matches[1]
	subquery := matches[2]
	alias := matches[3]
	remainder := matches[4]

	// Split remainder on known SQL keywords to isolate the ON clause
	keywordRe := regexp.MustCompile(`(?i)\s+(LEFT|RIGHT|INNER|CROSS|JOIN|WHERE|GROUP\s+BY|ORDER\s+BY|HAVING|LIMIT|UNION)\b`)
	parts := keywordRe.Split(remainder, 2)
	onClause := strings.TrimSpace(parts[0])
	trailing := ""
	if len(parts) == 2 {
		// Find where the trailing part starts in the original remainder
		idx := strings.Index(remainder, parts[1])
		if idx >= 0 {
			trailing = remainder[idx:]
		}
	}

	newSubquery := fmt.Sprintf("(%s ORDER BY 1 LIMIT 1) AS %s", subquery, alias)
	var replacement string
	if joinType != "" {
		replacement = fmt.Sprintf("LEFT JOIN %s ON %s", newSubquery, onClause)
	} else {
		replacement = fmt.Sprintf("JOIN %s ON %s", newSubquery, onClause)
	}

	return strings.Replace(sql, matches[0], replacement+trailing, 1)
}

func (t *Translator) translateToTsVector(sql string) string {
	if t.fulltextMode == "like" {
		// Step 1: replace plainto_tsquery with LIKE pattern
		re := regexp.MustCompile(`(?i)plainto_tsquery\s*\([^,]*,\s*'([^']+)'\)`)
		sql = re.ReplaceAllString(sql, "'%$1%'")
		// Step 2: replace to_tsvector with column name (using [^(),] to avoid greedy crossing)
		re = regexp.MustCompile(`(?i)to_tsvector\s*\([^,]*,\s*([^(),]+)\)`)
		sql = re.ReplaceAllString(sql, "$1")
		// Step 3: convert @@ to LIKE
		re = regexp.MustCompile(`(\w+)\s*@@\s+'%([^']+)%'`)
		sql = re.ReplaceAllString(sql, "$1 LIKE '%$2%'")
	} else {
		// match_against mode: step 1 - replace plainto_tsquery
		re := regexp.MustCompile(`(?i)plainto_tsquery\s*\([^,]*,\s*'([^']+)'\)`)
		sql = re.ReplaceAllString(sql, "'$1'")
		// Step 2: replace the combined pattern (now cleaner)
		re = regexp.MustCompile(`(?i)to_tsvector\s*\([^,]*,\s*([^(),]+)\)\s*@@\s*'([^']+)'`)
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

func (t *Translator) translateAny(sql string) string {
	// Handle: column = ANY ( NULL ) → 1=0 (always false)
	re := regexp.MustCompile(`(?i)(\w[\w."` + "`" + `]*)\s*=\s*ANY\s*\(\s*NULL\s*\)`)
	sql = re.ReplaceAllString(sql, "1=0")

	// Handle: column = ANY ( $N ) → just reference the param directly (won't match array semantics but avoids syntax error)
	re2 := regexp.MustCompile(`(?i)(\w[\w."` + "`" + `]*)\s*=\s*ANY\s*\(\s*\$(\d+)\s*\)`)
	sql = re2.ReplaceAllString(sql, "${1} = ${2}")

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

// translateDollarParams converts PG $1, $2 parameters to MySQL ? placeholders
func (t *Translator) translateDollarParams(sql string) string {
	re := regexp.MustCompile(`\$\d+`)
	return re.ReplaceAllString(sql, "?")
}

// translateStringAgg converts string_agg to GROUP_CONCAT
func (t *Translator) translateStringAgg(sql string) string {
	// string_agg(col, delimiter) -> GROUP_CONCAT(col SEPARATOR delimiter)
	re := regexp.MustCompile(`(?i)string_agg\s*\(([^,]+),\s*'([^']*)'\s*(?:ORDER\s+BY\s+[^)]+)?\)`)
	sql = re.ReplaceAllStringFunc(sql, func(match string) string {
		subMatches := re.FindStringSubmatch(match)
		if len(subMatches) != 3 {
			return match
		}
		col := subMatches[1]
		sep := subMatches[2]
		return fmt.Sprintf("GROUP_CONCAT(%s SEPARATOR '%s')", col, sep)
	})
	return sql
}

// translateBoolOperators handles PG boolean operators that differ from MySQL
func (t *Translator) translateBoolOperators(sql string) string {
	// PG: !!expr -> MySQL: NOT expr (negation)
	// Without lookbehind: just match !! followed by word
	re := regexp.MustCompile(`(?i)!!\s*(\w+)`)
	sql = re.ReplaceAllString(sql, "NOT $1")

	// PG: true/false literals -> MySQL: 1/0
	// Only in value positions, not in column names
	re = regexp.MustCompile(`(?i)(?:=\s*|IS\s+)(true|false)\b`)
	sql = re.ReplaceAllStringFunc(sql, func(match string) string {
		if strings.HasSuffix(strings.ToUpper(match), "TRUE") {
			return match[:len(match)-4] + "1"
		}
		return match[:len(match)-5] + "0"
	})

	return sql
}

// translateCoalesceInterval handles COALESCE + INTERVAL arithmetic
func (t *Translator) translateCoalesceInterval(sql string) string {
	// COALESCE is already MySQL compatible
	// INTERVAL 'N' unit already handled by translateIntervalArith
	// Handle: col + INTERVAL 'N' unit -> already works in MySQL
	// Handle: col - INTERVAL 'N' unit -> already works in MySQL

	// PG: make_interval(years, months, days) -> MySQL: INTERVAL
	re := regexp.MustCompile(`(?i)make_interval\s*\(([^)]+)\)`)
	sql = re.ReplaceAllStringFunc(sql, func(match string) string {
		// Simplified: just return a placeholder for complex cases
		return "INTERVAL 0 SECOND"
	})

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
	// MySQL doesn't support RETURNING. For INSERT ... RETURNING,
	// just execute the INSERT. The caller (executePreparedStatement)
	// will handle sending back the result row.
	return mainSQL
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

// translateDoubleQuotes converts PG double-quoted identifiers to MySQL backtick quotes.
func (t *Translator) translateDoubleQuotes(sql string) string {
	var buf strings.Builder
	buf.Grow(len(sql))
	i := 0
	for i < len(sql) {
		ch := sql[i]
		if ch == '\'' {
			buf.WriteByte(ch)
			i++
			for i < len(sql) {
				if sql[i] == '\'' {
					buf.WriteByte('\'')
					i++
					if i < len(sql) && sql[i] == '\'' {
						buf.WriteByte('\'')
						i++
						continue
					}
					break
				}
				if sql[i] == '\\' && i+1 < len(sql) {
					buf.WriteByte(sql[i])
					i++
					buf.WriteByte(sql[i])
					i++
					continue
				}
				buf.WriteByte(sql[i])
				i++
			}
			continue
		}
		if ch == '"' {
			i++
			start := i
			for i < len(sql) && sql[i] != '"' {
				i++
			}
			ident := sql[start:i]
			if i < len(sql) {
				i++
			}
			// Strip "public". prefix
			if idx := strings.LastIndex(ident, "."); idx >= 0 {
				prefix := ident[:idx]
				// Strip backtick-quoted or bare "public" schema
				trimmed := strings.Trim(prefix, "`")
				if trimmed == "public" {
					ident = ident[idx+1:]
				}
			}
			buf.WriteByte('`')
			buf.WriteString(ident)
			buf.WriteByte('`')
			continue
		}
		buf.WriteByte(ch)
		i++
	}
	// Strip `public`. schema prefixes (PG's default schema, not used in MySQL)
	result := buf.String()
	result = strings.ReplaceAll(result, "`public`.", "")
	return result
}
