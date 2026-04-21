package postgresql

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ArrayConverter handles PG array <-> MySQL JSON conversions
type ArrayConverter struct {
	mode string // "json" or "delimited"
}

func NewArrayConverter(mode string) *ArrayConverter {
	if mode == "" {
		mode = "json"
	}
	return &ArrayConverter{mode: mode}
}

// ToMySQL converts a PG array literal to MySQL JSON
func (c *ArrayConverter) ToMySQL(pgArray string) (string, error) {
	// Parse PG array format: {elem1,elem2,elem3} or ["elem1","elem2"]
	pgArray = strings.TrimSpace(pgArray)

	if strings.HasPrefix(pgArray, "{") && strings.HasSuffix(pgArray, "}") {
		// PG native array format
		inner := pgArray[1 : len(pgArray)-1]
		if inner == "" {
			return "[]", nil
		}

		elements := parsePGArrayElements(inner)
		jsonArr, err := json.Marshal(elements)
		if err != nil {
			return "", err
		}
		return string(jsonArr), nil
	}

	if strings.HasPrefix(pgArray, "[") {
		// Already JSON format
		return pgArray, nil
	}

	// Single value
	return fmt.Sprintf("[%q]", pgArray), nil
}

// ToPG converts a MySQL JSON value back to PG array format
func (c *ArrayConverter) ToPG(jsonStr string) (string, error) {
	jsonStr = strings.TrimSpace(jsonStr)

	if jsonStr == "[]" || jsonStr == "" {
		return "{}", nil
	}

	if c.mode == "delimited" {
		// Return as comma-delimited string
		var elements []string
		if err := json.Unmarshal([]byte(jsonStr), &elements); err != nil {
			return "", err
		}
		return strings.Join(elements, ","), nil
	}

	// Convert JSON array to PG array format
	var elements []string
	if err := json.Unmarshal([]byte(jsonStr), &elements); err != nil {
		// Try as generic JSON array
		var rawElements []interface{}
		if err := json.Unmarshal([]byte(jsonStr), &rawElements); err != nil {
			return jsonStr, nil
		}
		for _, e := range rawElements {
			elements = append(elements, fmt.Sprintf("%v", e))
		}
	}

	return "{" + strings.Join(elements, ",") + "}", nil
}

// TranslateQueryInPlace translates array references in a query
func (c *ArrayConverter) TranslateQueryInPlace(sql string) string {
	// Replace array column access patterns
	// This is a simpler approach - handle the most common patterns

	// col @> ARRAY[...] -> JSON_CONTAINS
	// col && ARRAY[...] -> JSON_OVERLAPS
	// 'x' = ANY(col) -> JSON_CONTAINS(col, '"x"')

	// These are handled by the main translator
	return sql
}

// parsePGArrayElements splits PG array elements handling quoted strings
func parsePGArrayElements(inner string) []string {
	var elements []string
	var current strings.Builder
	inQuotes := false
	escaped := false

	for _, ch := range inner {
		if escaped {
			current.WriteRune(ch)
			escaped = false
			continue
		}

		switch ch {
		case '\\':
			escaped = true
		case '"':
			inQuotes = !inQuotes
			current.WriteRune(ch)
		case ',':
			if inQuotes {
				current.WriteRune(ch)
			} else {
				elem := strings.TrimSpace(current.String())
				// Remove surrounding quotes if present
				if len(elem) >= 2 && elem[0] == '"' && elem[len(elem)-1] == '"' {
					elem = elem[1 : len(elem)-1]
					// Unescape quotes
					elem = strings.ReplaceAll(elem, `""`, `"`)
				}
				if elem != "" || current.Len() > 0 {
					elements = append(elements, elem)
				}
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}

	// Last element
	elem := strings.TrimSpace(current.String())
	if len(elem) >= 2 && elem[0] == '"' && elem[len(elem)-1] == '"' {
		elem = elem[1 : len(elem)-1]
		elem = strings.ReplaceAll(elem, `""`, `"`)
	}
	if elem != "" || current.Len() > 0 {
		elements = append(elements, elem)
	}

	return elements
}
