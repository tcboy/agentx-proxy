package pgwire

import (
	"bytes"
	"testing"
)

func TestWriteAndReadStartup(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf)

	// Test authentication OK
	err := w.AuthenticationOK()
	if err != nil {
		t.Fatalf("AuthenticationOK error: %v", err)
	}

	// Verify the output
	data := buf.Bytes()
	if len(data) < 5 {
		t.Fatalf("expected at least 5 bytes, got %d", len(data))
	}

	if data[0] != MsgAuthentication {
		t.Errorf("expected auth message type 'R', got %c", data[0])
	}
}

func TestWriteReadyForQuery(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf)

	err := w.SendReadyForQuery(StatusIdle)
	if err != nil {
		t.Fatalf("SendReadyForQuery error: %v", err)
	}

	data := buf.Bytes()
	if len(data) < 5 {
		t.Fatalf("expected at least 5 bytes, got %d", len(data))
	}

	if data[0] != MsgReadyForQuery {
		t.Errorf("expected ReadyForQuery type 'Z', got %c", data[0])
	}

	if data[5] != StatusIdle {
		t.Errorf("expected idle status 'I', got %c", data[5])
	}
}

func TestWriteParameterStatus(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf)

	err := w.SendParameterStatus("server_version", "14.0")
	if err != nil {
		t.Fatalf("SendParameterStatus error: %v", err)
	}

	data := buf.Bytes()
	if len(data) < 5 {
		t.Fatalf("expected at least 5 bytes, got %d", len(data))
	}

	if data[0] != MsgParameterStatus {
		t.Errorf("expected ParameterStatus type 'S', got %c", data[0])
	}
}

func TestWriteRowDescription(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf)

	fields := []FieldDescription{
		{Name: "id", TypeOID: 23, TypeSize: 4, FormatCode: 0},
		{Name: "name", TypeOID: 25, TypeSize: -1, FormatCode: 0},
	}

	err := w.SendRowDescription(fields)
	if err != nil {
		t.Fatalf("SendRowDescription error: %v", err)
	}

	data := buf.Bytes()
	if len(data) < 5 {
		t.Fatalf("expected at least 5 bytes, got %d", len(data))
	}

	if data[0] != MsgRowDescription {
		t.Errorf("expected RowDescription type 'T', got %c", data[0])
	}
}

func TestWriteDataRow(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf)

	values := []interface{}{
		123,
		"test user",
		nil,
		true,
	}

	err := w.SendDataRow(values, nil)
	if err != nil {
		t.Fatalf("SendDataRow error: %v", err)
	}

	data := buf.Bytes()
	if len(data) < 5 {
		t.Fatalf("expected at least 5 bytes, got %d", len(data))
	}

	if data[0] != MsgDataRow {
		t.Errorf("expected DataRow type 'D', got %c", data[0])
	}
}

func TestWriteCommandComplete(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf)

	err := w.SendCommandComplete("INSERT 1")
	if err != nil {
		t.Fatalf("SendCommandComplete error: %v", err)
	}

	data := buf.Bytes()
	if len(data) < 5 {
		t.Fatalf("expected at least 5 bytes, got %d", len(data))
	}

	if data[0] != MsgCommandComplete {
		t.Errorf("expected CommandComplete type 'C', got %c", data[0])
	}
}

func TestWriteErrorResponse(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewWriter(buf)

	err := w.SendErrorResponse(map[byte]string{
		FieldSeverity: "ERROR",
		FieldCode:     "XX000",
		FieldMessage:  "test error",
	})
	if err != nil {
		t.Fatalf("SendErrorResponse error: %v", err)
	}

	data := buf.Bytes()
	if len(data) < 5 {
		t.Fatalf("expected at least 5 bytes, got %d", len(data))
	}

	if data[0] != MsgErrorResponse {
		t.Errorf("expected ErrorResponse type 'E', got %c", data[0])
	}
}

func TestParseQuery(t *testing.T) {
	body := []byte("SELECT * FROM users WHERE id = 1\x00")
	q := ParseQuery(body)

	if q.String != "SELECT * FROM users WHERE id = 1" {
		t.Errorf("ParseQuery = %q, want %q", q.String, "SELECT * FROM users WHERE id = 1")
	}
}
