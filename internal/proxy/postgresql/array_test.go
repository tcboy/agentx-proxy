package postgresql

import (
	"testing"
)

func TestArrayConverterToMySQL(t *testing.T) {
	converter := NewArrayConverter("json")

	tests := []struct {
		input    string
		expected string
	}{
		{`{tag1,tag2,tag3}`, `["tag1","tag2","tag3"]`},
		{`{}`, `[]`},
		{`{"hello world","foo"}`, `["hello world","foo"]`},
		{`["already","json"]`, `["already","json"]`},
		{`single`, `["single"]`},
		{`{}`, `[]`},
	}

	for _, tc := range tests {
		result, err := converter.ToMySQL(tc.input)
		if err != nil {
			t.Errorf("ToMySQL(%q) error: %v", tc.input, err)
			continue
		}
		if result != tc.expected {
			t.Errorf("ToMySQL(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestArrayConverterToPG(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		input    string
		expected string
	}{
		{
			name:     "json mode - normal array",
			mode:     "json",
			input:    `["tag1","tag2","tag3"]`,
			expected: `{tag1,tag2,tag3}`,
		},
		{
			name:     "json mode - empty array",
			mode:     "json",
			input:    `[]`,
			expected: `{}`,
		},
		{
			name:     "delimited mode",
			mode:     "delimited",
			input:    `["tag1","tag2"]`,
			expected: `tag1,tag2`,
		},
		{
			name:     "empty string",
			mode:     "json",
			input:    ``,
			expected: `{}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			converter := NewArrayConverter(tc.mode)
			result, err := converter.ToPG(tc.input)
			if err != nil {
				t.Fatalf("ToPG(%q) error: %v", tc.input, err)
			}
			if result != tc.expected {
				t.Errorf("ToPG(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestArrayConverterDefaultMode(t *testing.T) {
	converter := NewArrayConverter("")
	if converter.mode != "json" {
		t.Errorf("default mode = %q, want %q", converter.mode, "json")
	}
}

func TestParsePGArrayElements(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{`tag1,tag2,tag3`, []string{"tag1", "tag2", "tag3"}},
		{`"hello, world","foo"`, []string{"hello, world", "foo"}},
		{`""`, []string{""}},
		{`a`, []string{"a"}},
		{``, nil},
		{`"with ""quotes"""`, []string{`with "quotes"`}},
	}

	for _, tc := range tests {
		result := parsePGArrayElements(tc.input)
		if len(result) != len(tc.expected) {
			t.Errorf("parsePGArrayElements(%q) length = %d, want %d", tc.input, len(result), len(tc.expected))
			continue
		}
		for i, v := range result {
			if v != tc.expected[i] {
				t.Errorf("parsePGArrayElements(%q)[%d] = %q, want %q", tc.input, i, v, tc.expected[i])
			}
		}
	}
}

func TestArrayConverterTranslateQueryInPlace(t *testing.T) {
	converter := NewArrayConverter("json")

	// This function is a passthrough since the main translator handles these
	input := "SELECT * FROM traces WHERE 'tag1' = ANY(tags)"
	result := converter.TranslateQueryInPlace(input)
	if result != input {
		t.Errorf("TranslateQueryInPlace should be passthrough, got %q", result)
	}
}
