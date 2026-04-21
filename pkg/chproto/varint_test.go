package chproto

import (
	"bytes"
	"testing"
)

func TestVarIntRoundTrip(t *testing.T) {
	tests := []uint64{
		0, 1, 127, 128, 255, 256,
		16383, 16384, 65535, 65536,
		1<<21 - 1, 1 << 21,
		1<<28 - 1, 1 << 28,
		1<<35 - 1, 1 << 35,
		1<<42 - 1, 1 << 42,
		1<<49 - 1, 1 << 49,
		1<<56 - 1, 1 << 56,
		1<<63 - 1,
	}

	for _, v := range tests {
		var buf bytes.Buffer
		w := &byteWriter{&buf}
		if err := WriteVarInt(w, v); err != nil {
			t.Fatalf("WriteVarInt(%d) error: %v", v, err)
		}

		r := &byteReader{bytes.NewReader(buf.Bytes())}
		got, err := ReadVarInt(r)
		if err != nil {
			t.Fatalf("ReadVarInt error for value %d: %v", v, err)
		}
		if got != v {
			t.Errorf("VarInt round-trip: wrote %d, read %d", v, got)
		}
	}
}

func TestVarIntEncoding(t *testing.T) {
	tests := []struct {
		value    uint64
		expected []byte
	}{
		{0, []byte{0x00}},
		{1, []byte{0x01}},
		{127, []byte{0x7F}},
		{128, []byte{0x80, 0x01}},
		{255, []byte{0xFF, 0x01}},
		{16383, []byte{0xFF, 0x7F}},
		{16384, []byte{0x80, 0x80, 0x01}},
	}

	for _, tc := range tests {
		var buf bytes.Buffer
		w := &byteWriter{&buf}
		if err := WriteVarInt(w, tc.value); err != nil {
			t.Fatalf("WriteVarInt(%d) error: %v", tc.value, err)
		}
		got := buf.Bytes()
		if !bytes.Equal(got, tc.expected) {
			t.Errorf("WriteVarInt(%d) = %v, want %v", tc.value, got, tc.expected)
		}
	}
}

func TestStringRoundTrip(t *testing.T) {
	tests := []string{
		"", "a", "hello", "AgentX Proxy",
		"SELECT * FROM traces WHERE project_id = 'test'",
		"ClickHouse", "UTC",
	}

	for _, s := range tests {
		var buf bytes.Buffer
		w := &byteWriter{&buf}
		if err := WriteString(w, s); err != nil {
			t.Fatalf("WriteString(%q) error: %v", s, err)
		}

		r := &byteReader{bytes.NewReader(buf.Bytes())}
		got, err := ReadString(r)
		if err != nil {
			t.Fatalf("ReadString error: %v", err)
		}
		if got != s {
			t.Errorf("String round-trip: wrote %q, read %q", s, got)
		}
	}
}

func TestFixedUint32RoundTrip(t *testing.T) {
	tests := []uint32{
		0, 1, 255, 256, 65535, 65536,
		1<<24 - 1, 1 << 24,
		1<<32 - 1,
		54460, 54461,
	}

	for _, v := range tests {
		var buf bytes.Buffer
		w := &byteWriter{&buf}
		if err := WriteFixedUint32(w, v); err != nil {
			t.Fatalf("WriteFixedUint32(%d) error: %v", v, err)
		}

		r := &byteReader{bytes.NewReader(buf.Bytes())}
		got, err := ReadFixedUint32(r)
		if err != nil {
			t.Fatalf("ReadFixedUint32 error: %v", err)
		}
		if got != v {
			t.Errorf("FixedUint32 round-trip: wrote %d, read %d", v, got)
		}
	}
}

func TestFixedUint64RoundTrip(t *testing.T) {
	tests := []uint64{
		0, 1, 255, 65535,
		1 << 32, 1<<40 - 1,
		1<<63 - 1,
	}

	for _, v := range tests {
		var buf bytes.Buffer
		w := &byteWriter{&buf}
		if err := WriteFixedUint64(w, v); err != nil {
			t.Fatalf("WriteFixedUint64(%d) error: %v", v, err)
		}

		r := &byteReader{bytes.NewReader(buf.Bytes())}
		got, err := ReadFixedUint64(r)
		if err != nil {
			t.Fatalf("ReadFixedUint64 error: %v", err)
		}
		if got != v {
			t.Errorf("FixedUint64 round-trip: wrote %d, read %d", v, got)
		}
	}
}

func TestConstants(t *testing.T) {
	if ProtoVersion != 54460 {
		t.Errorf("ProtoVersion = %d, want 54460", ProtoVersion)
	}
	if ProtoRevision != 54461 {
		t.Errorf("ProtoRevision = %d, want 54461", ProtoRevision)
	}
	if ProtoDBMSVersion != "24.8.5.115" {
		t.Errorf("ProtoDBMSVersion = %s, want 24.8.5.115", ProtoDBMSVersion)
	}
	if PacketQuery != 1 {
		t.Errorf("PacketQuery = %d, want 1", PacketQuery)
	}
	if ServerPacketEndOfStream != 5 {
		t.Errorf("ServerPacketEndOfStream = %d, want 5", ServerPacketEndOfStream)
	}
	if CompressionEnabled != 1 {
		t.Errorf("CompressionEnabled = %d, want 1", CompressionEnabled)
	}
	if CompressionDisabled != 0 {
		t.Errorf("CompressionDisabled = %d, want 0", CompressionDisabled)
	}
	if QueryStageComplete != 0 {
		t.Errorf("QueryStageComplete = %d, want 0", QueryStageComplete)
	}
	if QueryInitial != 0 {
		t.Errorf("QueryInitial = %d, want 0", QueryInitial)
	}
	if QuerySecondary != 1 {
		t.Errorf("QuerySecondary = %d, want 1", QuerySecondary)
	}
}

// byteWriter wraps bytes.Buffer to implement io.ByteWriter
type byteWriter struct {
	*bytes.Buffer
}

func (w *byteWriter) WriteByte(b byte) error {
	return w.Buffer.WriteByte(b)
}
