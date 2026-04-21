package pgwire

import (
	"bytes"
	"encoding/binary"
	"io"
)

// PostgreSQL Wire Protocol constants
const (
	// Message type bytes
	MsgAuthentication = 'R'
	MsgParameterStatus = 'S'
	MsgBackendKeyData  = 'K'
	MsgReadyForQuery    = 'Z'
	MsgRowDescription   = 'T'
	MsgDataRow         = 'D'
	MsgCommandComplete  = 'C'
	MsgErrorResponse    = 'E'
	MsgNoticeResponse   = 'N'
	MsgParseComplete    = '1'
	MsgBindComplete     = '2'
	MsgCloseComplete    = '3'
	MsgNoData          = 'n'
	MsgPortalSuspended  = 's'
	MsgEmptyQuery      = 'I'
	MsgCopyInResponse   = 'G'
	MsgCopyOutResponse  = 'H'
	MsgCopyData         = 'd'
	MsgCopyDone         = 'c'
	MsgParameterDescription = 't'

	// Client message types
	MsgClientPassword  = 'p'
	MsgClientQuery     = 'Q'
	MsgClientParse     = 'P'
	MsgClientBind      = 'B'
	MsgClientExecute   = 'E'
	MsgClientDescribe  = 'D'
	MsgClientSync      = 'S'
	MsgClientFlush     = 'H'
	MsgClientClose     = 'C'
	MsgClientTerminate = 'X'
	MsgClientCopyData  = 'd'
	MsgClientCopyDone  = 'c'
	MsgClientCopyFail  = 'f'
)

// Auth types
const (
	AuthOK        = 0
	AuthCleartext = 3
	AuthMD5       = 5
	AuthSASL      = 10
)

// Field descriptions for ErrorResponse
const (
	FieldSeverity    = 'S'
	FieldCode        = 'C'
	FieldMessage     = 'M'
	FieldDetail      = 'D'
	FieldHint        = 'H'
	FieldPosition    = 'P'
	FieldWhere       = 'W'
	FieldSchemaName  = 's'
	FieldTableName   = 't'
	FieldColumnName  = 'c'
	FieldDataType    = 'd'
)

// ReadyForQuery status
const (
	StatusIdle       = 'I'
	StatusInTrans    = 'T'
	StatusInFailedTx = 'E'
)

// StartupMessage is the initial message from client
type StartupMessage struct {
	ProtocolVersion uint32
	Parameters      map[string]string
}

// SSLRequest asks if server supports SSL
type SSLRequest struct{}

// CancelRequest cancels a running query
type CancelRequest struct {
	ProcessID uint32
	SecretKey uint32
}

// Query message
type Query struct {
	String string
}

// Parse message (extended protocol)
type Parse struct {
	Name             string
	Query            string
	ParameterOIDs    []uint32
}

// Bind message
type Bind struct {
	Portal             string
	PreparedStatement  string
	ParameterFormatCodes []int16
	Parameters         [][]byte
	ResultFormatCodes  []int16
}

// Execute message
type Execute struct {
	Portal  string
	MaxRows uint32
}

// Describe message
type Describe struct {
	Type byte // 'S' for prepared statement, 'P' for portal
	Name string
}

// Sync message
type Sync struct{}

// Flush message
type Flush struct{}

// Close message
type Close struct {
	Type byte // 'S' or 'P'
	Name string
}

// RowDescription column info
type FieldDescription struct {
	Name            string
	TableOID        uint32
	TableAttrNumber uint16
	TypeOID         uint32
	TypeSize        int16
	TypeModifier    int32
	FormatCode      int16
}

// Reader for PG wire protocol
type Reader struct {
	r io.Reader
}

func NewReader(r io.Reader) *Reader {
	return &Reader{r: r}
}

// ReadStartupMessage reads the initial startup message
func (r *Reader) ReadStartupMessage() (*StartupMessage, error) {
	var length uint32
	if err := binary.Read(r.r, binary.BigEndian, &length); err != nil {
		return nil, err
	}

	buf := make([]byte, length-4)
	if _, err := io.ReadFull(r.r, buf); err != nil {
		return nil, err
	}

	// First 4 bytes are protocol version, rest is null-terminated parameters
	params := make(map[string]string)
	paramData := buf[4:]
	if len(paramData) > 1 {
		parts := bytes.Split(paramData[:len(paramData)-1], []byte{0})
		for i := 0; i+1 < len(parts); i += 2 {
			params[string(parts[i])] = string(parts[i+1])
		}
	}

	return &StartupMessage{
		ProtocolVersion: 196608, // 3.0
		Parameters:      params,
	}, nil
}

// ReadMessage reads a single message from the wire
func (r *Reader) ReadMessage() (byte, []byte, error) {
	msgType := make([]byte, 1)
	if _, err := io.ReadFull(r.r, msgType); err != nil {
		return 0, nil, err
	}

	var length uint32
	if err := binary.Read(r.r, binary.BigEndian, &length); err != nil {
		return 0, nil, err
	}

	buf := make([]byte, length-4)
	if _, err := io.ReadFull(r.r, buf); err != nil {
		return 0, nil, err
	}

	return msgType[0], buf, nil
}

// ParseQuery parses a Query message body
func ParseQuery(body []byte) *Query {
	idx := bytes.IndexByte(body, 0)
	return &Query{String: string(body[:idx])}
}

// ParseParse parses a Parse message body
func ParseParse(body []byte) *Parse {
	p := &Parse{}
	idx := bytes.IndexByte(body, 0)
	p.Name = string(body[:idx])
	body = body[idx+1:]

	idx = bytes.IndexByte(body, 0)
	p.Query = string(body[:idx])
	body = body[idx+1:]

	numOIDs := binary.BigEndian.Uint16(body)
	body = body[2:]
	for i := uint16(0); i < numOIDs; i++ {
		p.ParameterOIDs = append(p.ParameterOIDs, binary.BigEndian.Uint32(body[:4]))
		body = body[4:]
	}
	return p
}

// ParseBind parses a Bind message body
func ParseBind(body []byte) *Bind {
	b := &Bind{}
	idx := bytes.IndexByte(body, 0)
	b.Portal = string(body[:idx])
	body = body[idx+1:]

	idx = bytes.IndexByte(body, 0)
	b.PreparedStatement = string(body[:idx])
	body = body[idx+1:]

	// Parameter format codes
	numFmt := binary.BigEndian.Uint16(body)
	body = body[2:]
	for i := uint16(0); i < numFmt; i++ {
		b.ParameterFormatCodes = append(b.ParameterFormatCodes, int16(binary.BigEndian.Uint16(body[:2])))
		body = body[2:]
	}

	// Parameters
	numParams := binary.BigEndian.Uint16(body)
	body = body[2:]
	for i := uint16(0); i < numParams; i++ {
		paramLen := int32(binary.BigEndian.Uint32(body[:4]))
		body = body[4:]
		if paramLen == -1 {
			b.Parameters = append(b.Parameters, nil)
		} else {
			param := make([]byte, paramLen)
			copy(param, body[:paramLen])
			b.Parameters = append(b.Parameters, param)
			body = body[paramLen:]
		}
	}

	// Result format codes
	numResultFmt := binary.BigEndian.Uint16(body)
	body = body[2:]
	for i := uint16(0); i < numResultFmt; i++ {
		b.ResultFormatCodes = append(b.ResultFormatCodes, int16(binary.BigEndian.Uint16(body[:2])))
		body = body[2:]
	}

	return b
}

// ParseExecute parses an Execute message body
func ParseExecute(body []byte) *Execute {
	e := &Execute{}
	idx := bytes.IndexByte(body, 0)
	e.Portal = string(body[:idx])
	body = body[idx+1:]
	e.MaxRows = binary.BigEndian.Uint32(body)
	return e
}

// ParseDescribe parses a Describe message body
func ParseDescribe(body []byte) *Describe {
	return &Describe{
		Type: body[0],
		Name: string(body[1 : len(body)-1]),
	}
}

// ParseClose parses a Close message body
func ParseClose(body []byte) *Close {
	return &Close{
		Type: body[0],
		Name: string(body[1 : len(body)-1]),
	}
}
