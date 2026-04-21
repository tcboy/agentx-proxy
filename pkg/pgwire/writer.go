package pgwire

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strconv"
)

// Writer writes PG wire protocol messages
type Writer struct {
	w io.Writer
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

// writeMessage writes a single wire message with type prefix
func (w *Writer) writeMessage(msgType byte, body []byte) error {
	if _, err := w.w.Write([]byte{msgType}); err != nil {
		return err
	}
	length := uint32(len(body) + 4)
	if err := binary.Write(w.w, binary.BigEndian, length); err != nil {
		return err
	}
	_, err := w.w.Write(body)
	return err
}

// writeMessageNoType writes a message without the type prefix (for startup, etc.)
func (w *Writer) writeMessageNoType(body []byte) error {
	length := uint32(len(body) + 4)
	if err := binary.Write(w.w, binary.BigEndian, length); err != nil {
		return err
	}
	_, err := w.w.Write(body)
	return err
}

// AuthenticationOK sends auth success
func (w *Writer) AuthenticationOK() error {
	body := make([]byte, 4)
	binary.BigEndian.PutUint32(body, AuthOK)
	return w.writeMessage(MsgAuthentication, body)
}

// SendReadyForQuery tells client the server is ready
func (w *Writer) SendReadyForQuery(status byte) error {
	return w.writeMessage(MsgReadyForQuery, []byte{status})
}

// SendParameterStatus sends a parameter status update
func (w *Writer) SendParameterStatus(name, value string) error {
	buf := bytes.NewBuffer(nil)
	buf.WriteString(name)
	buf.WriteByte(0)
	buf.WriteString(value)
	buf.WriteByte(0)
	return w.writeMessage(MsgParameterStatus, buf.Bytes())
}

// SendBackendKeyData sends process ID and secret key
func (w *Writer) SendBackendKeyData(pid uint32, secret uint32) error {
	body := make([]byte, 8)
	binary.BigEndian.PutUint32(body, pid)
	binary.BigEndian.PutUint32(body[4:], secret)
	return w.writeMessage(MsgBackendKeyData, body)
}

// SendRowDescription sends column metadata
func (w *Writer) SendRowDescription(fields []FieldDescription) error {
	buf := bytes.NewBuffer(nil)
	binary.Write(buf, binary.BigEndian, uint16(len(fields)))
	for _, f := range fields {
		buf.WriteString(f.Name)
		buf.WriteByte(0)
		binary.Write(buf, binary.BigEndian, f.TableOID)
		binary.Write(buf, binary.BigEndian, f.TableAttrNumber)
		binary.Write(buf, binary.BigEndian, f.TypeOID)
		binary.Write(buf, binary.BigEndian, f.TypeSize)
		binary.Write(buf, binary.BigEndian, f.TypeModifier)
		binary.Write(buf, binary.BigEndian, f.FormatCode)
	}
	return w.writeMessage(MsgRowDescription, buf.Bytes())
}

// SendDataRow sends a single row of data
func (w *Writer) SendDataRow(values []interface{}, formatCodes []int16) error {
	buf := bytes.NewBuffer(nil)
	binary.Write(buf, binary.BigEndian, uint16(len(values)))
	for i, v := range values {
		fmtCode := int16(0) // text format default
		if i < len(formatCodes) {
			fmtCode = formatCodes[i]
		}
		if v == nil {
			binary.Write(buf, binary.BigEndian, int32(-1))
			continue
		}

		var data []byte
		switch val := v.(type) {
		case []byte:
			data = val
		case string:
			data = []byte(val)
		case int:
			data = []byte(strconv.Itoa(val))
		case int64:
			data = []byte(strconv.FormatInt(val, 10))
		case uint64:
			data = []byte(strconv.FormatUint(val, 10))
		case float64:
			data = []byte(strconv.FormatFloat(val, 'f', -1, 64))
		case bool:
			if val {
				data = []byte("t")
			} else {
				data = []byte("f")
			}
		default:
			data = []byte(fmt.Sprintf("%v", val))
		}

		if fmtCode == 1 { // binary format
			binary.Write(buf, binary.BigEndian, int32(len(data)))
			buf.Write(data)
		} else {
			binary.Write(buf, binary.BigEndian, int32(len(data)))
			buf.Write(data)
		}
	}
	return w.writeMessage(MsgDataRow, buf.Bytes())
}

// SendCommandComplete sends the completion message for a command
func (w *Writer) SendCommandComplete(tag string) error {
	return w.writeMessage(MsgCommandComplete, append([]byte(tag), 0))
}

// SendParseComplete signals parse complete
func (w *Writer) SendParseComplete() error {
	return w.writeMessage(MsgParseComplete, nil)
}

// SendBindComplete signals bind complete
func (w *Writer) SendBindComplete() error {
	return w.writeMessage(MsgBindComplete, nil)
}

// SendCloseComplete signals close complete
func (w *Writer) SendCloseComplete() error {
	return w.writeMessage(MsgCloseComplete, nil)
}

// SendNoData signals no data for describe
func (w *Writer) SendNoData() error {
	return w.writeMessage(MsgNoData, nil)
}

// SendEmptyQueryResponse signals empty query
func (w *Writer) SendEmptyQueryResponse() error {
	return w.writeMessage(MsgEmptyQuery, nil)
}

// SendErrorResponse sends an error to the client
func (w *Writer) SendErrorResponse(fields map[byte]string) error {
	buf := bytes.NewBuffer(nil)
	for code, value := range fields {
		buf.WriteByte(code)
		buf.WriteString(value)
		buf.WriteByte(0)
	}
	buf.WriteByte(0) // terminator
	return w.writeMessage(MsgErrorResponse, buf.Bytes())
}

// SendNoticeResponse sends a notice to the client
func (w *Writer) SendNoticeResponse(fields map[byte]string) error {
	buf := bytes.NewBuffer(nil)
	for code, value := range fields {
		buf.WriteByte(code)
		buf.WriteString(value)
		buf.WriteByte(0)
	}
	buf.WriteByte(0)
	return w.writeMessage(MsgNoticeResponse, buf.Bytes())
}

// SendParameterDescription describes prepared statement parameter types
func (w *Writer) SendParameterDescription(oids []uint32) error {
	buf := bytes.NewBuffer(nil)
	binary.Write(buf, binary.BigEndian, uint16(len(oids)))
	for _, oid := range oids {
		binary.Write(buf, binary.BigEndian, oid)
	}
	return w.writeMessage(MsgParameterDescription, buf.Bytes())
}

// SendStartupMessage is sent during initial connection (not used by proxy)
func (w *Writer) SendStartupMessage(version uint32) error {
	buf := bytes.NewBuffer(nil)
	binary.Write(buf, binary.BigEndian, version)
	return w.writeMessageNoType(buf.Bytes())
}

// SendAuthenticationCleartextPassword requests cleartext password
func (w *Writer) SendAuthenticationCleartextPassword() error {
	body := make([]byte, 4)
	binary.BigEndian.PutUint32(body, AuthCleartext)
	return w.writeMessage(MsgAuthentication, body)
}
