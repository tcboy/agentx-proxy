package postgresql

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"github.com/agentx-labs/agentx-proxy/pkg/pgwire"
)

// TestPGWireStartup tests startup message parsing
func TestPGWireStartup(t *testing.T) {
	// Build parameter bytes: key\0value\0key\0value\0\0
	paramBytes := []byte("user\x00langfuse\x00database\x00langfuse_db\x00\x00")
	contentLen := 4 + 4 + len(paramBytes)
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint32(contentLen))
	binary.Write(&buf, binary.BigEndian, uint32(196608))
	buf.Write(paramBytes)

	r := pgwire.NewReader(&buf)
	startup, err := r.ReadStartupMessage()
	if err != nil {
		t.Fatalf("ReadStartupMessage error: %v", err)
	}

	if startup.Parameters["user"] != "langfuse" {
		t.Errorf("expected user=langfuse, got %q", startup.Parameters["user"])
	}
	if startup.Parameters["database"] != "langfuse_db" {
		t.Errorf("expected database=langfuse_db, got %q", startup.Parameters["database"])
	}
}

// TestPGWireQuery tests query message encoding/decoding
func TestPGWireQuery(t *testing.T) {
	tests := []string{
		"SELECT * FROM users",
		"INSERT INTO traces (id, name) VALUES ('abc', 'test')",
		"SELECT version()",
		"SET client_encoding TO 'UTF8'",
		"BEGIN",
		"COMMIT",
	}

	for _, query := range tests {
		var buf bytes.Buffer
		buf.WriteByte(pgwire.MsgClientQuery)

		// Body: query string (null-terminated)
		body := append([]byte(query), 0)
		binary.Write(&buf, binary.BigEndian, uint32(4+len(body)))
		buf.Write(body)

		// Read it back
		r := pgwire.NewReader(&buf)
		msgType, bodyData, err := r.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage(%q) error: %v", query, err)
		}

		if msgType != pgwire.MsgClientQuery {
			t.Errorf("expected msg type 'Q', got %q", string(rune(msgType)))
		}

		parsed := pgwire.ParseQuery(bodyData)
		if parsed.String != query {
			t.Errorf("ParseQuery = %q, want %q", parsed.String, query)
		}
	}
}

// TestPGWireExtendedProtocol tests Parse/Bind/Execute message parsing
func TestPGWireExtendedProtocol(t *testing.T) {
	// Test Parse message
	parseBody := []byte{}
	parseBody = append(parseBody, 0) // empty name
	parseBody = append(parseBody, []byte("SELECT * FROM users WHERE id = $1")...)
	parseBody = append(parseBody, 0)
	parseBody = append(parseBody, 0, 1) // 1 OID
	parseBody = append(parseBody, 0, 0, 0, 25) // OID 25 (text)

	parsed := pgwire.ParseParse(parseBody)
	if parsed.Name != "" {
		t.Errorf("expected empty name, got %q", parsed.Name)
	}
	if parsed.Query != "SELECT * FROM users WHERE id = $1" {
		t.Errorf("expected query, got %q", parsed.Query)
	}
	if len(parsed.ParameterOIDs) != 1 || parsed.ParameterOIDs[0] != 25 {
		t.Errorf("expected 1 OID=25, got %v", parsed.ParameterOIDs)
	}

	// Test Bind message
	bindBody := []byte{}
	bindBody = append(bindBody, 0) // empty portal name
	bindBody = append(bindBody, []byte("my_stmt")...)
	bindBody = append(bindBody, 0)
	bindBody = append(bindBody, 0, 1) // 1 format code
	bindBody = append(bindBody, 0, 0) // text format
	bindBody = append(bindBody, 0, 1) // 1 parameter
	bindBody = append(bindBody, 0, 0, 0, 3) // param len 3
	bindBody = append(bindBody, '1', '2', '3') // param value
	bindBody = append(bindBody, 0, 0) // 0 result format codes

	b := pgwire.ParseBind(bindBody)
	if b.Portal != "" {
		t.Errorf("expected empty portal, got %q", b.Portal)
	}
	if b.PreparedStatement != "my_stmt" {
		t.Errorf("expected stmt=my_stmt, got %q", b.PreparedStatement)
	}
	if len(b.Parameters) != 1 {
		t.Errorf("expected 1 param, got %d", len(b.Parameters))
	}

	// Test Execute message
	execBody := []byte{}
	execBody = append(execBody, 0) // empty portal
	execBody = append(execBody, 0, 0, 0, 0) // max-rows = 0

	e := pgwire.ParseExecute(execBody)
	if e.Portal != "" {
		t.Errorf("expected empty portal, got %q", e.Portal)
	}
	if e.MaxRows != 0 {
		t.Errorf("expected max-rows=0, got %d", e.MaxRows)
	}
}

// TestPGWireResponseEncoding tests server response message encoding
func TestPGWireResponseEncoding(t *testing.T) {
	var buf bytes.Buffer
	w := pgwire.NewWriter(&buf)

	// AuthenticationOK
	if err := w.AuthenticationOK(); err != nil {
		t.Fatalf("AuthenticationOK error: %v", err)
	}

	// ParameterStatus
	if err := w.SendParameterStatus("server_version", "14.0"); err != nil {
		t.Fatalf("SendParameterStatus error: %v", err)
	}

	// ReadyForQuery
	if err := w.SendReadyForQuery(pgwire.StatusIdle); err != nil {
		t.Fatalf("SendReadyForQuery error: %v", err)
	}

	// Verify output
	data := buf.Bytes()
	if len(data) == 0 {
		t.Fatal("no output data")
	}

	// Check AuthenticationOK (R + length + 0)
	if data[0] != 'R' {
		t.Errorf("expected 'R' for auth, got %q", string(data[0]))
	}

	// Check ParameterStatus (S)
	foundS := false
	for i := 0; i < len(data); i++ {
		if data[i] == 'S' {
			foundS = true
			break
		}
	}
	if !foundS {
		t.Error("expected 'S' for ParameterStatus")
	}

	// Check ReadyForQuery (Z)
	foundZ := false
	for i := 0; i < len(data); i++ {
		if data[i] == 'Z' {
			foundZ = true
			break
		}
	}
	if !foundZ {
		t.Error("expected 'Z' for ReadyForQuery")
	}
}

// TestPGWireRowDescription tests row description encoding
func TestPGWireRowDescription(t *testing.T) {
	var buf bytes.Buffer
	w := pgwire.NewWriter(&buf)

	fields := []pgwire.FieldDescription{
		{Name: "id", TypeOID: 23, TypeSize: 4},
		{Name: "name", TypeOID: 25, TypeSize: -1},
	}

	if err := w.SendRowDescription(fields); err != nil {
		t.Fatalf("SendRowDescription error: %v", err)
	}

	data := buf.Bytes()
	if data[0] != 'T' {
		t.Errorf("expected 'T' for RowDescription, got %q", string(data[0]))
	}
}

// TestPGWireDataRow tests data row encoding
func TestPGWireDataRow(t *testing.T) {
	var buf bytes.Buffer
	w := pgwire.NewWriter(&buf)

	values := []interface{}{"123", "test user"}
	if err := w.SendDataRow(values, nil); err != nil {
		t.Fatalf("SendDataRow error: %v", err)
	}

	data := buf.Bytes()
	if data[0] != 'D' {
		t.Errorf("expected 'D' for DataRow, got %q", string(data[0]))
	}
}

// TestPGWireCommandComplete tests command complete encoding
func TestPGWireCommandComplete(t *testing.T) {
	var buf bytes.Buffer
	w := pgwire.NewWriter(&buf)

	if err := w.SendCommandComplete("SELECT 5"); err != nil {
		t.Fatalf("SendCommandComplete error: %v", err)
	}

	data := buf.Bytes()
	if data[0] != 'C' {
		t.Errorf("expected 'C' for CommandComplete, got %q", string(data[0]))
	}
}

// TestPGWireParseBindExecute tests the full extended protocol flow
func TestPGWireParseBindExecute(t *testing.T) {
	var buf bytes.Buffer

	// Parse
	w := pgwire.NewWriter(&buf)
	if err := w.SendParseComplete(); err != nil {
		t.Fatalf("ParseComplete error: %v", err)
	}

	// Bind
	if err := w.SendBindComplete(); err != nil {
		t.Fatalf("BindComplete error: %v", err)
	}

	// NoData for describe
	w.SendNoData()

	// CommandComplete
	if err := w.SendCommandComplete("SELECT 1"); err != nil {
		t.Fatalf("CommandComplete error: %v", err)
	}

	// Verify
	data := buf.Bytes()
	if data[0] != '1' {
		t.Errorf("expected '1' for ParseComplete, got %q", string(data[0]))
	}
	// Find BindComplete
	found2 := false
	for i := 0; i < len(data); i++ {
		if data[i] == '2' {
			found2 = true
			break
		}
	}
	if !found2 {
		t.Error("expected '2' for BindComplete")
	}
}

// TestTranslatorFullPipeline tests the full translation pipeline for common queries
func TestTranslatorFullPipeline(t *testing.T) {
	tr := NewTranslator("json", "match_against")

	tests := []struct {
		name     string
		input    string
		validate func(string) error
	}{
		{
			name:  "ILIKE query",
			input: "SELECT * FROM users WHERE name ILIKE '%test%'",
			validate: func(s string) error {
				if bytes.Contains([]byte(s), []byte("ILIKE")) {
					return io.EOF
				}
				if !bytes.Contains([]byte(s), []byte("LIKE")) {
					return io.EOF
				}
				return nil
			},
		},
		{
			name:  "Type cast removal",
			input: "SELECT id::text, name::varchar FROM users",
			validate: func(s string) error {
				if bytes.Contains([]byte(s), []byte("::")) {
					return io.EOF
				}
				return nil
			},
		},
		{
			name:  "ON CONFLICT DO NOTHING",
			input: "INSERT INTO users (id, name) VALUES ('1', 'test') ON CONFLICT (id) DO NOTHING",
			validate: func(s string) error {
				if bytes.Contains([]byte(s), []byte("ON CONFLICT")) {
					return io.EOF
				}
				if !bytes.Contains([]byte(s), []byte("INSERT IGNORE")) {
					return io.EOF
				}
				return nil
			},
		},
		{
			name:  "ON CONFLICT DO UPDATE",
			input: "INSERT INTO users (id, name) VALUES ('1', 'test') ON CONFLICT (id) DO UPDATE SET name = excluded.name",
			validate: func(s string) error {
				if !bytes.Contains([]byte(s), []byte("ON DUPLICATE KEY")) {
					return io.EOF
				}
				return nil
			},
		},
		{
			name:  "date_trunc",
			input: "SELECT date_trunc('hour', created_at) FROM traces",
			validate: func(s string) error {
				if bytes.Contains([]byte(s), []byte("date_trunc")) {
					return io.EOF
				}
				if !bytes.Contains([]byte(s), []byte("DATE_FORMAT")) {
					return io.EOF
				}
				return nil
			},
		},
		{
			name:  "EXTRACT EPOCH",
			input: "SELECT EXTRACT(EPOCH FROM created_at) FROM traces",
			validate: func(s string) error {
				if bytes.Contains([]byte(s), []byte("EXTRACT")) {
					return io.EOF
				}
				if !bytes.Contains([]byte(s), []byte("UNIX_TIMESTAMP")) {
					return io.EOF
				}
				return nil
			},
		},
		{
			name:  "Dollar parameters",
			input: "SELECT * FROM users WHERE id = $1",
			validate: func(s string) error {
				if bytes.Contains([]byte(s), []byte("$")) {
					return io.EOF
				}
				if !bytes.Contains([]byte(s), []byte("?")) {
					return io.EOF
				}
				return nil
			},
		},
		{
			name:  "JSONB functions",
			input: "SELECT jsonb_set(data, '{key}', 'value') FROM users",
			validate: func(s string) error {
				if bytes.Contains([]byte(s), []byte("jsonb_set")) {
					return io.EOF
				}
				if !bytes.Contains([]byte(s), []byte("JSON_SET")) {
					return io.EOF
				}
				return nil
			},
		},
		{
			name:  "ANY array operation",
			input: "SELECT * FROM users WHERE 'admin' = ANY(roles)",
			validate: func(s string) error {
				if bytes.Contains([]byte(s), []byte("ANY")) {
					return io.EOF
				}
				if !bytes.Contains([]byte(s), []byte("JSON_CONTAINS")) {
					return io.EOF
				}
				return nil
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tr.Translate(tc.input)
			if err != nil {
				t.Fatalf("Translate error: %v", err)
			}

			if err := tc.validate(result); err != nil {
				t.Errorf("validation failed for %q: got %q", tc.name, result)
			}
		})
	}
}
