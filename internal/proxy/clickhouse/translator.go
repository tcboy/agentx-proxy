package clickhouse

import (
	"fmt"
	"regexp"
	"strings"
)

// CHTranslator handles ClickHouse SQL -> MySQL SQL translation
type CHTranslator struct{}

func NewCHTranslator() *CHTranslator {
	return &CHTranslator{}
}

// Translate translates a ClickHouse SQL query to MySQL
func (t *CHTranslator) Translate(sql string) (string, error) {
	result := sql

	// Apply translations
	result = t.translateFinal(result)
	result = t.translateLimit1By(result)
	result = t.translateMapAccess(result)
	result = t.translateMapFunctions(result)
	result = t.translateArrayFunctions(result)
	result = t.translateCHDateFunctions(result)
	result = t.translateCHAggregateFunctions(result)
	result = t.translateTuple(result)
	result = t.translateHasFunctions(result)
	result = t.translateCHParameters(result)
	result = t.translateInterval(result)
	result = t.translateTTL(result)
	result = t.translateCast(result)
	result = t.translateToUnixTimestamp64(result)

	return result, nil
}

func (t *CHTranslator) translateFinal(sql string) string {
	re := regexp.MustCompile(`(?i)\bFINAL\b`)
	return re.ReplaceAllString(sql, "")
}

func (t *CHTranslator) translateLimit1By(sql string) string {
	// LIMIT 1 BY col1, col2 -> ROW_NUMBER() OVER (PARTITION BY col1, col2 ORDER BY event_ts DESC) = 1
	re := regexp.MustCompile(`(?i)\bLIMIT\s+(\d+)\s+BY\s+(.+?)\s*$`)
	return re.ReplaceAllStringFunc(sql, func(match string) string {
		subMatches := re.FindStringSubmatch(match)
		if len(subMatches) < 3 {
			return match
		}

		limitN := subMatches[1]
		_ = strings.TrimSpace(subMatches[2]) // byCols - used for potential ROW_NUMBER rewrite

		// Remove LIMIT clause from main query
		sql = strings.TrimSuffix(sql, match)

		// Wrap in subquery with ROW_NUMBER
		return fmt.Sprintf(") AS t WHERE rn <= %s", limitN)
	})

	// For more complex cases, we need to wrap the entire query
	re = regexp.MustCompile(`(?i)(.+?)\s+LIMIT\s+(\d+)\s+BY\s+(.+?)\s*(ORDER|WHERE|GROUP|HAVING|$)`)
	if re.MatchString(sql) {
		// This is a complex case - for now, just strip the LIMIT BY
		sql = regexp.MustCompile(`(?i)\s+LIMIT\s+\d+\s+BY\s+[^\s;]+`).ReplaceAllString(sql, "")
	}

	return sql
}

func (t *CHTranslator) translateMapAccess(sql string) string {
	// col['key'] -> JSON_UNQUOTE(JSON_EXTRACT(col, '$.key'))
	re := regexp.MustCompile(`(\w+)\['([^']+)'?\]`)
	sql = re.ReplaceAllString(sql, "JSON_UNQUOTE(JSON_EXTRACT($1, '$.$2'))")

	re = regexp.MustCompile(`(\w+)\["([^"]+)"?\]`)
	sql = re.ReplaceAllString(sql, "JSON_UNQUOTE(JSON_EXTRACT($1, '$.$2'))")

	return sql
}

func (t *CHTranslator) translateMapFunctions(sql string) string {
	// mapKeys -> JSON_KEYS
	re := regexp.MustCompile(`(?i)mapKeys\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "JSON_KEYS($1)")

	// mapValues -> JSON_EXTRACT all values
	re = regexp.MustCompile(`(?i)mapValues\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "JSON_EXTRACT($1, '$[*]')")

	// mapContains -> JSON_CONTAINS_PATH
	re = regexp.MustCompile(`(?i)mapContains\s*\(([^,]+),\s*'([^']+)'\)`)
	sql = re.ReplaceAllString(sql, "JSON_CONTAINS_PATH($1, 'one', '$.$2')")

	return sql
}

func (t *CHTranslator) translateArrayFunctions(sql string) string {
	// arrayJoin -> JSON_TABLE
	re := regexp.MustCompile(`(?i)arrayJoin\s*\((\w+)\)`)
	sql = re.ReplaceAllString(sql, "JSON_TABLE($1, '$[*]' COLUMNS (value TEXT PATH '$'))")

	// arraySum -> custom aggregation
	re = regexp.MustCompile(`(?i)arraySum\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "(SELECT SUM(CAST(value AS DECIMAL(65,12))) FROM JSON_TABLE($1, '$[*]' COLUMNS (value TEXT PATH '$')))")

	// arrayFilter
	re = regexp.MustCompile(`(?i)arrayFilter\s*\([^,]+,\s*(\w+)\)`)
	sql = re.ReplaceAllString(sql, "JSON_EXTRACT($1, '$[*]')")

	// indexOf
	re = regexp.MustCompile(`(?i)indexOf\s*\((\w+),\s*([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "(SELECT jt.idx FROM JSON_TABLE($1, '$[*]' COLUMNS (idx FOR ORDINALITY, val TEXT PATH '$')) jt WHERE jt.val = $2 LIMIT 1)")

	// arrayElement
	re = regexp.MustCompile(`(?i)arrayElement\s*\((\w+),\s*(\d+)\)`)
	sql = re.ReplaceAllString(sql, "JSON_UNQUOTE(JSON_EXTRACT($1, concat('$[', $2 - 1, ']')))")

	// empty array -> '[]'
	sql = strings.ReplaceAll(sql, "[]", "'[]'")

	// Array literal: ['a', 'b'] -> JSON array
	re = regexp.MustCompile(`\[('(?:[^'\\]|\\.)*'|[\d.]+)(?:\s*,\s*('(?:[^'\\]|\\.)*'|[\d.]+))*\]`)
	sql = re.ReplaceAllStringFunc(sql, func(match string) string {
		return match // Keep as-is for JSON_OVERLAPS
	})

	return sql
}

func (t *CHTranslator) translateCHDateFunctions(sql string) string {
	// toUnixTimestamp64Milli -> UNIX_TIMESTAMP * 1000
	re := regexp.MustCompile(`(?i)toUnixTimestamp64Milli\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "(UNIX_TIMESTAMP($1) * 1000)")

	// toUnixTimestamp -> UNIX_TIMESTAMP
	re = regexp.MustCompile(`(?i)toUnixTimestamp\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "UNIX_TIMESTAMP($1)")

	// toDate -> DATE
	re = regexp.MustCompile(`(?i)\btoDate\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "DATE($1)")

	// toStartOfHour
	re = regexp.MustCompile(`(?i)toStartOfHour\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "DATE_FORMAT($1, '%Y-%m-%d %H:00:00')")

	// toStartOfDay / toStartOfDate
	re = regexp.MustCompile(`(?i)toStartOf(?:Day|Date)\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "DATE($1)")

	// toStartOfMonth
	re = regexp.MustCompile(`(?i)toStartOfMonth\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "DATE_FORMAT($1, '%Y-%m-01')")

	// toStartOfYear
	re = regexp.MustCompile(`(?i)toStartOfYear\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "MAKEDATE(YEAR($1), 1)")

	// toStartOfMinute
	re = regexp.MustCompile(`(?i)toStartOfMinute\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "DATE_FORMAT($1, '%Y-%m-%d %H:%i:00')")

	// toStartOfFiveMinutes
	re = regexp.MustCompile(`(?i)toStartOfFiveMinutes\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "DATE_FORMAT($1, CONCAT(DATE_FORMAT($1, '%Y-%m-%d %H:'), LPAD(FLOOR(MINUTE($1)/5)*5, 2, '0'), ':00'))")

	// toStartOfInterval
	re = regexp.MustCompile(`(?i)toStartOfInterval\s*\(([^,]+),\s*INTERVAL\s+(\d+)\s+(\w+)\)`)
	sql = re.ReplaceAllStringFunc(sql, func(match string) string {
		subMatches := regexp.MustCompile(`(?i)toStartOfInterval\s*\(([^,]+),\s*INTERVAL\s+(\d+)\s+(\w+)\)`).FindStringSubmatch(match)
		if len(subMatches) != 4 {
			return match
		}
		return fmt.Sprintf("DATE_SUB(%s, INTERVAL (TIMESTAMPDIFF(%s, '1970-01-01', %s) %% %s) %s)",
			subMatches[1], strings.ToUpper(subMatches[3]), subMatches[1], subMatches[2], strings.ToUpper(subMatches[3]))
	})

	// toRelativeSecondNum
	re = regexp.MustCompile(`(?i)toRelativeSecondNum\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "UNIX_TIMESTAMP($1)")

	// toRelativeMinuteNum
	re = regexp.MustCompile(`(?i)toRelativeMinuteNum\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "UNIX_TIMESTAMP($1) / 60")

	// toRelativeHourNum
	re = regexp.MustCompile(`(?i)toRelativeHourNum\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "UNIX_TIMESTAMP($1) / 3600")

	// toRelativeDayNum
	re = regexp.MustCompile(`(?i)toRelativeDayNum\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "UNIX_TIMESTAMP($1) / 86400")

	// dateDiff -> TIMESTAMPDIFF
	re = regexp.MustCompile(`(?i)dateDiff\s*\(\s*'(\w+)'\s*,\s*([^,]+)\s*,\s*([^)]+)\s*\)`)
	sql = re.ReplaceAllStringFunc(sql, func(match string) string {
		subMatches := regexp.MustCompile(`(?i)dateDiff\s*\(\s*'(\w+)'\s*,\s*([^,]+)\s*,\s*([^)]+)\s*\)`).FindStringSubmatch(match)
		if len(subMatches) != 4 {
			return match
		}
		unit := strings.ToUpper(subMatches[1])
		switch unit {
		case "MILLISECOND":
			return fmt.Sprintf("TIMESTAMPDIFF(MICROSECOND, %s, %s) / 1000", subMatches[2], subMatches[3])
		default:
			return fmt.Sprintf("TIMESTAMPDIFF(%s, %s, %s)", unit, subMatches[2], subMatches[3])
		}
	})

	// date_diff (underscore variant)
	re = regexp.MustCompile(`(?i)date_diff\s*\(\s*'(\w+)'\s*,\s*([^,]+)\s*,\s*([^)]+)\s*\)`)
	sql = re.ReplaceAllStringFunc(sql, func(match string) string {
		subMatches := regexp.MustCompile(`(?i)date_diff\s*\(\s*'(\w+)'\s*,\s*([^,]+)\s*,\s*([^)]+)\s*\)`).FindStringSubmatch(match)
		if len(subMatches) != 4 {
			return match
		}
		unit := strings.ToUpper(subMatches[1])
		switch unit {
		case "MILLISECOND":
			return fmt.Sprintf("TIMESTAMPDIFF(MICROSECOND, %s, %s) / 1000", subMatches[2], subMatches[3])
		default:
			return fmt.Sprintf("TIMESTAMPDIFF(%s, %s, %s)", unit, subMatches[2], subMatches[3])
		}
	})

	// now() -> NOW() (already MySQL compatible)
	// today() -> CURDATE()
	re = regexp.MustCompile(`(?i)\btoday\s*\(\)`)
	sql = re.ReplaceAllString(sql, "CURDATE()")

	// yesterday() -> CURDATE() - INTERVAL 1 DAY
	re = regexp.MustCompile(`(?i)\byesterday\s*\(\)`)
	sql = re.ReplaceAllString(sql, "CURDATE() - INTERVAL 1 DAY")

	return sql
}

func (t *CHTranslator) translateCHAggregateFunctions(sql string) string {
	// countIf -> COUNT with CASE WHEN
	re := regexp.MustCompile(`(?i)countIf\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "COUNT(CASE WHEN $1 THEN 1 END)")

	// countIf (underscore)
	re = regexp.MustCompile(`(?i)count_if\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "COUNT(CASE WHEN $1 THEN 1 END)")

	// sumIf -> SUM with CASE WHEN
	re = regexp.MustCompile(`(?i)sumIf\s*\(([^,]+),\s*([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "SUM(CASE WHEN $2 THEN $1 END)")

	// avgIf
	re = regexp.MustCompile(`(?i)avgIf\s*\(([^,]+),\s*([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "AVG(CASE WHEN $2 THEN $1 END)")

	// minIf
	re = regexp.MustCompile(`(?i)minIf\s*\(([^,]+),\s*([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "MIN(CASE WHEN $2 THEN $1 END)")

	// maxIf
	re = regexp.MustCompile(`(?i)maxIf\s*\(([^,]+),\s*([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "MAX(CASE WHEN $2 THEN $1 END)")

	// groupUniqArrayArray -> JSON_ARRAYAGG(DISTINCT ...)
	re = regexp.MustCompile(`(?i)groupUniqArrayArray\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "JSON_ARRAYAGG(DISTINCT $1)")

	// groupArray -> JSON_ARRAYAGG
	re = regexp.MustCompile(`(?i)groupArray\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "JSON_ARRAYAGG($1)")

	// groupUniqArray -> JSON_ARRAYAGG(DISTINCT ...)
	re = regexp.MustCompile(`(?i)groupUniqArray\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "JSON_ARRAYAGG(DISTINCT $1)")

	// anyLast -> MAX (approximation)
	re = regexp.MustCompile(`(?i)anyLast\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "MAX($1)")

	// any -> ANY_VALUE (MySQL 8.0+)
	re = regexp.MustCompile(`(?i)\bany\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "ANY_VALUE($1)")

	// uniq -> COUNT(DISTINCT ...)
	re = regexp.MustCompile(`(?i)uniq\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "COUNT(DISTINCT $1)")

	// uniqCombined -> COUNT(DISTINCT ...)
	re = regexp.MustCompile(`(?i)uniqCombined\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "COUNT(DISTINCT $1)")

	// uniqExact -> COUNT(DISTINCT ...)
	re = regexp.MustCompile(`(?i)uniqExact\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "COUNT(DISTINCT $1)")

	// topK -> GROUP_CONCAT with LIMIT
	re = regexp.MustCompile(`(?i)topK\s*\((\d+)\)\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "SUBSTRING_INDEX(GROUP_CONCAT(DISTINCT $2 ORDER BY $2 DESC), ',', $1)")

	// histogram
	re = regexp.MustCompile(`(?i)histogram\s*\((\d+)\)\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "HISTOGRAM($2)") // MySQL 8.0+

	// SimpleAggregateFunction wrappers -> unwrap
	re = regexp.MustCompile(`(?i)SimpleAggregateFunction\(\w+,\s*([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "$1")

	// AggregateFunction wrappers -> unwrap
	re = regexp.MustCompile(`(?i)AggregateFunction\(\w+,\s*([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "$1")

	// argMaxState -> argMax
	re = regexp.MustCompile(`(?i)argMaxState\s*\(([^,]+),\s*([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "SUBSTRING_INDEX(GROUP_CONCAT(CONCAT($1, '|', $2) ORDER BY $2 DESC), '|', 1)")

	// argMax
	re = regexp.MustCompile(`(?i)argMax\s*\(([^,]+),\s*([^)]+)\)`)
	sql = re.ReplaceAllStringFunc(sql, func(match string) string {
		subMatches := regexp.MustCompile(`(?i)argMax\s*\(([^,]+),\s*([^)]+)\)`).FindStringSubmatch(match)
		if len(subMatches) != 3 {
			return match
		}
		return fmt.Sprintf("SUBSTRING_INDEX(GROUP_CONCAT(CONCAT(%s, '|', %s) ORDER BY %s DESC), '|', 1)",
			subMatches[1], subMatches[2], subMatches[2])
	})

	// sumMap -> custom handling
	re = regexp.MustCompile(`(?i)sumMap\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "$1")

	// maxMap
	re = regexp.MustCompile(`(?i)maxMap\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "$1")

	// sumMerge -> unwrap
	re = regexp.MustCompile(`(?i)(\w+)Merge\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "$1($2)")

	// sumMergeState -> unwrap
	re = regexp.MustCompile(`(?i)(\w+)MergeState\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "$1($2)")

	return sql
}

func (t *CHTranslator) translateTuple(sql string) string {
	// tuple(a, b, c) -> JSON_OBJECT or just a list
	re := regexp.MustCompile(`(?i)\btuple\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "JSON_OBJECT($1)")

	return sql
}

func (t *CHTranslator) translateHasFunctions(sql string) string {
	// hasAny(arr, ['x', 'y']) -> JSON_OVERLAPS(arr, '["x","y"]')
	re := regexp.MustCompile(`(?i)hasAny\s*\((\w+),\s*\[([^\]]*)\]\)`)
	sql = re.ReplaceAllStringFunc(sql, func(match string) string {
		subMatches := re.FindStringSubmatch(match)
		if len(subMatches) != 3 {
			return match
		}
		arr := subMatches[1]
		elements := subMatches[2]
		return fmt.Sprintf("JSON_OVERLAPS(%s, '[%s]')", arr, elements)
	})

	// hasAll(arr, ['x', 'y']) -> JSON_CONTAINS for each
	re = regexp.MustCompile(`(?i)hasAll\s*\((\w+),\s*\[([^\]]*)\]\)`)
	sql = re.ReplaceAllStringFunc(sql, func(match string) string {
		subMatches := re.FindStringSubmatch(match)
		if len(subMatches) != 3 {
			return match
		}
		arr := subMatches[1]
		elements := strings.Split(subMatches[2], ",")
		conditions := make([]string, len(elements))
		for i, e := range elements {
			e = strings.TrimSpace(e)
			conditions[i] = fmt.Sprintf("JSON_CONTAINS(%s, %s)", arr, e)
		}
		return "(" + strings.Join(conditions, " AND ") + ")"
	})

	// has(arr, 'x') -> JSON_CONTAINS
	re = regexp.MustCompile(`(?i)\bhas\s*\((\w+),\s*([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "JSON_CONTAINS($1, $2)")

	// empty(arr) -> JSON_LENGTH = 0
	re = regexp.MustCompile(`(?i)\bempty\s*\((\w+)\)`)
	sql = re.ReplaceAllString(sql, "(JSON_LENGTH($1) = 0)")

	// notEmpty(arr) -> JSON_LENGTH > 0
	re = regexp.MustCompile(`(?i)\bnotEmpty\s*\((\w+)\)`)
	sql = re.ReplaceAllString(sql, "(JSON_LENGTH($1) > 0)")

	return sql
}

func (t *CHTranslator) translateCHParameters(sql string) string {
	// {name: Type} -> ? (MySQL parameter)
	re := regexp.MustCompile(`\{(\w+):\s*\w+(?:\([^)]*\))?(?:\?)?\}`)
	sql = re.ReplaceAllString(sql, "?")

	return sql
}

func (t *CHTranslator) translateInterval(sql string) string {
	// INTERVAL N unit -> already MySQL compatible
	// But CH might use different syntax
	return sql
}

func (t *CHTranslator) translateTTL(sql string) string {
	// TTL ... -> WHERE clause filtering
	// CH TTL: TTL timestamp + INTERVAL 7 DAY
	// MySQL: WHERE timestamp > NOW() - INTERVAL 7 DAY
	// This is more of a DDL concern, handled at the schema level
	return sql
}

func (t *CHTranslator) translateCast(sql string) string {
	// CAST(... AS DateTime64(3)) -> CAST(... AS DATETIME(3))
	re := regexp.MustCompile(`(?i)CAST\((.+?)\s+AS\s+DateTime64\(\d+\)\)`)
	sql = re.ReplaceAllString(sql, "CAST($1 AS DATETIME(3))")

	// column::DateTime64(3) -> CAST(column AS DATETIME(3))
	re = regexp.MustCompile(`(\w+)::DateTime64\(\d+\)`)
	sql = re.ReplaceAllString(sql, "CAST($1 AS DATETIME(3))")

	// column::String -> just the column
	re = regexp.MustCompile(`(\w+)::String`)
	sql = re.ReplaceAllString(sql, "$1")

	// column::Float64
	re = regexp.MustCompile(`(\w+)::Float64`)
	sql = re.ReplaceAllString(sql, "CAST($1 AS DECIMAL(65,12))")

	// ::Decimal64(12)
	re = regexp.MustCompile(`(.+?)::Decimal64\(\d+\)`)
	sql = re.ReplaceAllString(sql, "CAST($1 AS DECIMAL(65,12))")

	return sql
}

func (t *CHTranslator) translateToUnixTimestamp64(sql string) string {
	// toUnixTimestamp64Nano -> UNIX_TIMESTAMP * 1e9
	re := regexp.MustCompile(`(?i)toUnixTimestamp64Nano\s*\(([^)]+)\)`)
	sql = re.ReplaceAllString(sql, "(UNIX_TIMESTAMP($1) * 1000000000)")

	return sql
}
