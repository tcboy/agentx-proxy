package chproto

import (
	"encoding/binary"
	"io"
)

// VarInt encoding (LEB128-like) used throughout ClickHouse Native protocol.
// Each byte: 7 data bits + 1 continuation bit (MSB).
// Continuation bit set (>= 0x80) means more bytes follow.

// ReadVarInt reads a variable-length integer from r.
func ReadVarInt(r io.ByteReader) (uint64, error) {
	var result uint64
	var shift uint
	for i := 0; i < 10; i++ { // max 10 bytes for uint64
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		result |= uint64(b&0x7F) << shift
		if b < 0x80 {
			return result, nil
		}
		shift += 7
	}
	return 0, io.ErrUnexpectedEOF
}

// WriteVarInt writes a variable-length integer to w.
func WriteVarInt(w io.ByteWriter, v uint64) error {
	for v >= 0x80 {
		if err := w.WriteByte(byte(v) | 0x80); err != nil {
			return err
		}
		v >>= 7
	}
	return w.WriteByte(byte(v))
}

// ReadString reads a VarInt-length-prefixed string from r.
func ReadString(r io.ByteReader) (string, error) {
	length, err := ReadVarInt(r)
	if err != nil {
		return "", err
	}
	buf := make([]byte, length)
	for i := uint64(0); i < length; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		buf[i] = b
	}
	return string(buf), nil
}

// WriteString writes a VarInt-length-prefixed string to w.
func WriteString(w io.ByteWriter, s string) error {
	if err := WriteVarInt(w, uint64(len(s))); err != nil {
		return err
	}
	for i := 0; i < len(s); i++ {
		if err := w.WriteByte(s[i]); err != nil {
			return err
		}
	}
	return nil
}

// byteReader wraps an io.Reader to implement io.ByteReader.
type byteReader struct {
	r io.Reader
}

func (b *byteReader) ReadByte() (byte, error) {
	var buf [1]byte
	_, err := io.ReadFull(b.r, buf[:])
	return buf[0], err
}

func (b *byteReader) Read(p []byte) (int, error) {
	return b.r.Read(p)
}

// NewByteReader wraps an io.Reader to provide ByteReader interface.
func NewByteReader(r io.Reader) io.ByteReader {
	return &byteReader{r: r}
}

// ReadFixedUint32 reads a little-endian uint32 from r.
func ReadFixedUint32(r io.ByteReader) (uint32, error) {
	var buf [4]byte
	for i := 0; i < 4; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		buf[i] = b
	}
	return binary.LittleEndian.Uint32(buf[:]), nil
}

// ReadFixedUint64 reads a little-endian uint64 from r.
func ReadFixedUint64(r io.ByteReader) (uint64, error) {
	var buf [8]byte
	for i := 0; i < 8; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		buf[i] = b
	}
	return binary.LittleEndian.Uint64(buf[:]), nil
}

// WriteFixedUint32 writes a little-endian uint32 to w.
func WriteFixedUint32(w io.ByteWriter, v uint32) error {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, v)
	for _, b := range buf {
		if err := w.WriteByte(b); err != nil {
			return err
		}
	}
	return nil
}

// WriteFixedUint64 writes a little-endian uint64 to w.
func WriteFixedUint64(w io.ByteWriter, v uint64) error {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, v)
	for _, b := range buf {
		if err := w.WriteByte(b); err != nil {
			return err
		}
	}
	return nil
}
