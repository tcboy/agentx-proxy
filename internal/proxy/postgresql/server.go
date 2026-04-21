package postgresql

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"strings"
	"sync"

	"github.com/agentx-labs/agentx-proxy/internal/config"
	"github.com/agentx-labs/agentx-proxy/internal/mysql"
	"github.com/agentx-labs/agentx-proxy/pkg/pgwire"
)

// Server is the PostgreSQL wire protocol proxy server
type Server struct {
	listener   net.Listener
	cfg        *config.Config
	pool       *mysql.Pool
	translator *Translator
	catalog    *Catalog
	arrayConv  *ArrayConverter
	connCount  int
	mu         sync.Mutex
}

func NewServer(cfg *config.Config, pool *mysql.Pool) (*Server, error) {
	ln, err := net.Listen("tcp", cfg.Listen.PostgreSQL)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", cfg.Listen.PostgreSQL, err)
	}

	return &Server{
		listener: ln,
		cfg:      cfg,
		pool:     pool,
		translator: NewTranslator(cfg.Proxy.PGToMySQL.ArrayColumnMode, cfg.Proxy.PGToMySQL.FulltextMode),
		catalog:  NewCatalog(pool),
		arrayConv: NewArrayConverter(cfg.Proxy.PGToMySQL.ArrayColumnMode),
	}, nil
}

func (s *Server) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				slog.Error("accept error", "error", err)
				continue
			}
		}

		s.mu.Lock()
		s.connCount++
		connID := s.connCount
		s.mu.Unlock()

		go s.handleConn(conn, connID)
	}
}

func (s *Server) Close() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// Connection state
type connState struct {
	// Transaction state
	inTransaction   bool
	transactionFailed bool

	// Prepared statements
	preparedStatements map[string]*preparedStmt

	// Portals
	portals map[string]*portal

	// Backend key data
	pid    uint32
	secret uint32

	// Parameters
	parameters map[string]string

	// Writer/Reader
	w *pgwire.Writer
	r *pgwire.Reader
}

type preparedStmt struct {
	name       string
	query      string
	paramOIDs  []uint32
	paramCount int
}

type portal struct {
	name       string
	stmt       *preparedStmt
	params     [][]byte
	formatCodes []int16
}

func (s *Server) handleConn(conn net.Conn, connID int) {
	defer conn.Close()

	state := &connState{
		preparedStatements: make(map[string]*preparedStmt),
		portals:            make(map[string]*portal),
		parameters:         make(map[string]string),
		w:                  pgwire.NewWriter(conn),
		r:                  pgwire.NewReader(conn),
		pid:                uint32(rand.IntN(100000)),
		secret:             uint32(rand.IntN(100000)),
	}

	slog.Info("new PG connection", "conn_id", connID, "remote", conn.RemoteAddr())
	defer slog.Info("PG connection closed", "conn_id", connID)

	// Handle SSLRequest - clients may ask for SSL upgrade before sending startup message
	// Read 4 bytes length + 4 bytes protocol code
	buf := make([]byte, 8)
	if _, err := io.ReadFull(conn, buf); err != nil {
		slog.Error("read startup probe", "error", err)
		return
	}

	protocolCode := binary.BigEndian.Uint32(buf[4:])
	if protocolCode == 1234<<16|5679 {
		// SSLRequest - we don't support SSL, reply with 'N'
		conn.Write([]byte{'N'})
		// Now read the actual startup message
	}

	// Push back the first 8 bytes so ReadStartupMessage can read them
	// We use a custom approach: create a combined reader
	startupLen := binary.BigEndian.Uint32(buf[:4])
	remaining := make([]byte, startupLen-4)
	if _, err := io.ReadFull(conn, remaining); err != nil {
		slog.Error("read startup params", "error", err)
		return
	}

	// Combine and parse startup manually
	allData := append(buf, remaining...)
	params := make(map[string]string)
	paramData := allData[8:] // skip length(4) + protocol(4)
	if len(paramData) > 1 {
		parts := bytes.Split(paramData[:len(paramData)-1], []byte{0})
		for i := 0; i+1 < len(parts); i += 2 {
			params[string(parts[i])] = string(parts[i+1])
		}
	}

	state.parameters = params

	// Authentication
	if err := state.w.AuthenticationOK(); err != nil {
		slog.Error("auth", "error", err)
		return
	}

	// Parameter status messages
	statusParams := map[string]string{
		"server_version":          "14.0 (AgentX Proxy)",
		"server_encoding":         "UTF8",
		"client_encoding":         "UTF8",
		"application_name":        "agentx-proxy",
		"DateStyle":               "ISO, MDY",
		"IntervalStyle":           "postgres",
		"TimeZone":                "UTC",
		"integer_datetimes":       "on",
		"standard_conforming_strings": "on",
	}

	for k, v := range statusParams {
		state.w.SendParameterStatus(k, v)
		state.parameters[k] = v
	}

	// Backend key data
	state.w.SendBackendKeyData(state.pid, state.secret)

	// Ready for query
	state.w.SendReadyForQuery(pgwire.StatusIdle)

	// Main message loop
	for {
		msgType, body, err := state.r.ReadMessage()
		if err != nil {
			slog.Debug("read message error", "conn_id", connID, "error", err)
			return
		}

		if err := s.handleMessage(state, msgType, body, connID); err != nil {
			slog.Error("handle message", "conn_id", connID, "type", string(rune(msgType)), "error", err)
			if writeErr := state.w.SendErrorResponse(map[byte]string{
				pgwire.FieldSeverity: "ERROR",
				pgwire.FieldCode:     "XX000",
				pgwire.FieldMessage:  err.Error(),
			}); writeErr != nil {
				return
			}
			state.w.SendReadyForQuery(s.getTransactionStatus(state))
		}
	}
}

func (s *Server) handleMessage(state *connState, msgType byte, body []byte, connID int) error {
	switch msgType {
	case pgwire.MsgClientQuery:
		return s.handleSimpleQuery(state, body, connID)

	case pgwire.MsgClientParse:
		return s.handleParse(state, body, connID)

	case pgwire.MsgClientBind:
		return s.handleBind(state, body, connID)

	case pgwire.MsgClientExecute:
		return s.handleExecute(state, body, connID)

	case pgwire.MsgClientDescribe:
		return s.handleDescribe(state, body, connID)

	case pgwire.MsgClientSync:
		return state.w.SendReadyForQuery(s.getTransactionStatus(state))

	case pgwire.MsgClientFlush:
		// Flush - do nothing, all messages are flushed automatically
		return nil

	case pgwire.MsgClientClose:
		return s.handleClose(state, body, connID)

	case pgwire.MsgClientTerminate:
		return nil // Connection will be closed

	case pgwire.MsgClientPassword:
		// Ignore password messages (we already authed with OK)
		return nil

	default:
		slog.Warn("unknown message type", "type", string(rune(msgType)))
		return nil
	}
}

func (s *Server) handleSimpleQuery(state *connState, body []byte, connID int) error {
	query := pgwire.ParseQuery(body).String
	query = strings.TrimRight(query, "\x00")

	slog.Debug("query", "conn_id", connID, "sql", query)

	return s.executeQuery(state, query, connID)
}

func (s *Server) handleParse(state *connState, body []byte, connID int) error {
	msg := pgwire.ParseParse(body)
	slog.Debug("parse", "conn_id", connID, "name", msg.Name, "query", msg.Query)

	state.preparedStatements[msg.Name] = &preparedStmt{
		name:      msg.Name,
		query:     msg.Query,
		paramOIDs: msg.ParameterOIDs,
	}

	return state.w.SendParseComplete()
}

func (s *Server) handleBind(state *connState, body []byte, connID int) error {
	msg := pgwire.ParseBind(body)
	slog.Debug("bind", "conn_id", connID, "portal", msg.Portal, "stmt", msg.PreparedStatement)

	stmt, ok := state.preparedStatements[msg.PreparedStatement]
	if !ok {
		return fmt.Errorf("prepared statement %q not found", msg.PreparedStatement)
	}

	state.portals[msg.Portal] = &portal{
		name:        msg.Portal,
		stmt:        stmt,
		params:      msg.Parameters,
		formatCodes: msg.ParameterFormatCodes,
	}

	return state.w.SendBindComplete()
}

func (s *Server) handleExecute(state *connState, body []byte, connID int) error {
	msg := pgwire.ParseExecute(body)
	slog.Debug("execute", "conn_id", connID, "portal", msg.Portal)

	portal, ok := state.portals[msg.Portal]
	if !ok {
		return fmt.Errorf("portal %q not found", msg.Portal)
	}

	return s.executePreparedStatement(state, portal, connID)
}

func (s *Server) handleDescribe(state *connState, body []byte, connID int) error {
	msg := pgwire.ParseDescribe(body)

	if msg.Type == 'S' {
		// Describe prepared statement
		stmt, ok := state.preparedStatements[msg.Name]
		if !ok {
			return fmt.Errorf("prepared statement %q not found", msg.Name)
		}

		// Return parameter description
		if len(stmt.paramOIDs) > 0 {
			if err := state.w.SendParameterDescription(stmt.paramOIDs); err != nil {
				return err
			}
		} else {
			state.w.SendNoData()
		}
	} else {
		// Describe portal - we don't know the result types ahead of time
		state.w.SendNoData()
	}

	return nil
}

func (s *Server) handleClose(state *connState, body []byte, connID int) error {
	msg := pgwire.ParseClose(body)

	if msg.Type == 'S' {
		delete(state.preparedStatements, msg.Name)
	} else {
		delete(state.portals, msg.Name)
	}

	return state.w.SendCloseComplete()
}

func (s *Server) executeQuery(state *connState, query string, connID int) error {
	// Handle transaction commands
	switch q := normalizeQuery(query); {
	case q == "begin":
		if state.inTransaction {
			return nil
		}
		state.inTransaction = true
		state.transactionFailed = false
		_, err := s.pool.Exec(context.Background(), "BEGIN")
		if err != nil {
			state.transactionFailed = true
			return err
		}
		return state.w.SendCommandComplete("BEGIN")

	case q == "commit" || q == "commit work" || q == "end":
		if !state.inTransaction {
			return nil
		}
		_, err := s.pool.Exec(context.Background(), "COMMIT")
		state.inTransaction = false
		if err != nil {
			state.transactionFailed = false
			return err
		}
		return state.w.SendCommandComplete("COMMIT")

	case q == "rollback" || q == "rollback work":
		if !state.inTransaction {
			return nil
		}
		_, err := s.pool.Exec(context.Background(), "ROLLBACK")
		state.inTransaction = false
		state.transactionFailed = false
		if err != nil {
			return err
		}
		return state.w.SendCommandComplete("ROLLBACK")

	case len(q) >= 9 && q[:9] == "savepoint":
		if !state.inTransaction {
			state.inTransaction = true
		}
		_, err := s.pool.Exec(context.Background(), query)
		if err != nil {
			state.transactionFailed = true
			return err
		}
		return state.w.SendCommandComplete("SAVEPOINT")

	case len(q) >= 17 && q[:17] == "release savepoint":
		_, err := s.pool.Exec(context.Background(), query)
		if err != nil {
			state.transactionFailed = true
			return err
		}
		return state.w.SendCommandComplete("RELEASE")

	case len(q) >= 22 && q[:22] == "rollback to savepoint":
		_, err := s.pool.Exec(context.Background(), query)
		if err != nil {
			state.transactionFailed = true
			return err
		}
		return state.w.SendCommandComplete("ROLLBACK TO SAVEPOINT")

	case q == "set session character set to 'utf8'" || q == "set client_encoding to 'utf8'" ||
		len(q) >= 18 && q[:18] == "set session_replica":
		// Ignore SET commands that MySQL doesn't support in the same way
		return state.w.SendCommandComplete("SET")

	case len(q) >= 4 && q[:4] == "set ":
		// Other SET commands - try to execute them
		_, err := s.pool.Exec(context.Background(), query)
		if err != nil {
			// Ignore SET errors
			slog.Debug("ignoring SET error", "query", query, "error", err)
		}
		return state.w.SendCommandComplete("SET")

	case q == "show transaction isolation level":
		return state.w.SendCommandComplete("SHOW")

	case len(q) >= 5 && q[:5] == "show ":
		// SHOW commands
		return s.handleShowCommand(state, q)
	}

	// Check for pg_catalog queries
	if s.catalog.IsCatalogQuery(query) {
		return s.executeCatalogQuery(state, query, connID)
	}

	// Check for SELECT version()
	if normalizeQuery(query) == "select version()" || (len(query) >= 15 && strings.EqualFold(query[:14], "select version")) {
		fields := []pgwire.FieldDescription{
			{Name: "version", TypeOID: 25, TypeSize: -1, FormatCode: 0},
		}
		state.w.SendRowDescription(fields)
		state.w.SendDataRow([]interface{}{"14.0 (AgentX Proxy)"}, nil)
		return state.w.SendCommandComplete("SELECT 1")
	}

	// Translate PG SQL to MySQL SQL
	translated, err := s.translator.Translate(query)
	if err != nil {
		return fmt.Errorf("translate: %w", err)
	}

	slog.Debug("translated query", "original", query, "translated", translated)

	// Execute the translated query
	return s.executeMySQLQuery(state, translated, connID)
}

func (s *Server) executePreparedStatement(state *connState, p *portal, connID int) error {
	query := p.stmt.query
	slog.Debug("prepared query", "conn_id", connID, "sql", query, "params", len(p.params))

	// Translate the query
	translated, err := s.translator.Translate(query)
	if err != nil {
		return fmt.Errorf("translate: %w", err)
	}

	// If there are parameters, we need to substitute them
	// For now, use prepared statements with MySQL
	if len(p.params) > 0 {
		return s.executeParameterizedMySQLQuery(state, translated, p.params, connID)
	}

	return s.executeMySQLQuery(state, translated, connID)
}

func (s *Server) executeCatalogQuery(state *connState, query string, connID int) error {
	ctx := context.Background()

	columns, data, err := s.catalog.HandleCatalogQuery(ctx, query)
	if err != nil {
		return err
	}

	if columns == nil && data == nil {
		// Pass through to MySQL (information_schema)
		return s.executeMySQLQuery(state, query, connID)
	}

	// Send row description
	fields := make([]pgwire.FieldDescription, len(columns))
	for i, col := range columns {
		fields[i] = pgwire.FieldDescription{
			Name:       col,
			FormatCode: 0, // text format
		}
		// Set reasonable type OIDs
		switch col {
		case "oid", "typnamespace", "typrelid", "typelem", "typarray", "typbasetype", "typtypmod", "typndims", "typcollation",
			"relnamespace", "reltype", "reloftype", "relowner", "relam", "relfilenode", "reltablespace",
			"relpages", "relallvisible", "reltoastrelid", "relnatts", "relchecks",
			"attrelid", "atttypid",
			"indexrelid", "indrelid",
			"pronamespace", "proowner", "prolang", "provariadic", "pronargdefaults", "prorettype",
			"enumtypid", "connamespace", "conrelid", "contypid", "conindid", "conparentid", "confrelid",
			"datdba", "extowner", "extnamespace":
			fields[i].TypeOID = 23 // int4
		case "reltuples", "procost", "prorows", "enumsortorder":
			fields[i].TypeOID = 701 // float8
		case "typnotnull", "relhasindex", "relisshared", "relhaspkey", "typbyval", "attnotnull":
			fields[i].TypeOID = 16 // bool
		}
		if fields[i].TypeOID == 0 {
			fields[i].TypeOID = 25 // text
		}
		fields[i].TypeSize = -1 // variable
	}

	if err := state.w.SendRowDescription(fields); err != nil {
		return err
	}

	// Send data rows
	for _, row := range data {
		if err := state.w.SendDataRow(row, nil); err != nil {
			return err
		}
	}

	return state.w.SendCommandComplete(fmt.Sprintf("SELECT %d", len(data)))
}

func (s *Server) executeMySQLQuery(state *connState, query string, connID int) error {
	ctx := context.Background()

	// Detect write queries upfront to avoid failed Query attempt
	if isWriteQuery(query) {
		result, err := s.pool.Exec(ctx, query)
		if err != nil {
			return fmt.Errorf("exec: %w (query: %s)", err, query)
		}
		affected, _ := result.RowsAffected()
		tag := getCommandTag(query, affected, 0)
		return state.w.SendCommandComplete(tag)
	}

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("query: %w (query: %s)", err, query)
	}
	defer rows.Close()

	// Get column info
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return err
	}

	columns := make([]pgwire.FieldDescription, len(colTypes))
	colNames, err := rows.Columns()
	if err != nil {
		return err
	}
	for i, ct := range colTypes {
		columns[i] = pgwire.FieldDescription{
			Name:       colNames[i],
			TypeOID:   mysqlTypeToPGOID(ct.DatabaseTypeName()),
			TypeSize:  -1,
			FormatCode: 0,
		}
	}

	if err := state.w.SendRowDescription(columns); err != nil {
		return err
	}

	rowCount := 0
	for rows.Next() {
		values := make([]interface{}, len(colTypes))
		valuePtrs := make([]interface{}, len(colTypes))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return err
		}

		if err := state.w.SendDataRow(values, nil); err != nil {
			return err
		}
		rowCount++
	}

	if err := rows.Err(); err != nil {
		return err
	}

	return state.w.SendCommandComplete(fmt.Sprintf("SELECT %d", rowCount))
}

func (s *Server) executeParameterizedMySQLQuery(state *connState, query string, params [][]byte, connID int) error {
	ctx := context.Background()

	// Convert params to interface slice
	args := make([]interface{}, len(params))
	for i, p := range params {
		if p == nil {
			args[i] = nil
		} else {
			args[i] = string(p)
		}
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		// Try as exec
		result, writeErr := s.pool.Exec(ctx, query, args...)
		if writeErr != nil {
			return fmt.Errorf("exec parameterized: %w (query: %s)", writeErr, query)
		}

		affected, _ := result.RowsAffected()
		tag := getCommandTag(query, affected, 0)
		return state.w.SendCommandComplete(tag)
	}
	defer rows.Close()

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return err
	}

	columns := make([]pgwire.FieldDescription, len(colTypes))
	colNames, err := rows.Columns()
	if err != nil {
		return err
	}
	for i, ct := range colTypes {
		columns[i] = pgwire.FieldDescription{
			Name:       colNames[i],
			TypeOID:   mysqlTypeToPGOID(ct.DatabaseTypeName()),
			TypeSize:  -1,
			FormatCode: 0,
		}
	}

	if err := state.w.SendRowDescription(columns); err != nil {
		return err
	}

	rowCount := 0
	for rows.Next() {
		values := make([]interface{}, len(colTypes))
		valuePtrs := make([]interface{}, len(colTypes))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return err
		}

		if err := state.w.SendDataRow(values, nil); err != nil {
			return err
		}
		rowCount++
	}

	if err := rows.Err(); err != nil {
		return err
	}

	return state.w.SendCommandComplete(fmt.Sprintf("SELECT %d", rowCount))
}

func (s *Server) handleShowCommand(state *connState, query string) error {
	// Handle common SHOW commands
	q := normalizeQuery(query)

	showResponses := map[string]string{
		"show server_version":              "14.0 (AgentX Proxy)",
		"show server_version_num":          "140000",
		"show standard_conforming_strings": "on",
		"show timezone":                    "UTC",
		"show datestyle":                   "ISO, MDY",
		"show transaction isolation level": "read committed",
	}

	if resp, ok := showResponses[q]; ok {
		fields := []pgwire.FieldDescription{
			{Name: q[5:], TypeOID: 25, TypeSize: -1, FormatCode: 0},
		}
		state.w.SendRowDescription(fields)
		state.w.SendDataRow([]interface{}{resp}, nil)
		return state.w.SendCommandComplete("SHOW 1")
	}

	// Try executing as-is
	return s.executeMySQLQuery(state, query, 0)
}

// Helpers

func normalizeQuery(q string) string {
	if len(q) > 0 && q[len(q)-1] == 0 {
		q = q[:len(q)-1]
	}
	return strings.ToLower(strings.TrimSpace(q))
}

func isWriteQuery(q string) bool {
	upper := strings.ToUpper(strings.TrimSpace(q))
	return strings.HasPrefix(upper, "INSERT") ||
		strings.HasPrefix(upper, "UPDATE") ||
		strings.HasPrefix(upper, "DELETE") ||
		strings.HasPrefix(upper, "CREATE") ||
		strings.HasPrefix(upper, "ALTER") ||
		strings.HasPrefix(upper, "DROP")
}

func getCommandTag(query string, affected, lastID int64) string {
	upper := strings.ToUpper(strings.TrimSpace(query))
	switch {
	case strings.HasPrefix(upper, "INSERT"):
		return fmt.Sprintf("INSERT %d", affected)
	case strings.HasPrefix(upper, "UPDATE"):
		return fmt.Sprintf("UPDATE %d", affected)
	case strings.HasPrefix(upper, "DELETE"):
		return fmt.Sprintf("DELETE %d", affected)
	case strings.HasPrefix(upper, "CREATE"):
		return "CREATE TABLE"
	case strings.HasPrefix(upper, "ALTER"):
		return "ALTER TABLE"
	case strings.HasPrefix(upper, "DROP"):
		return "DROP TABLE"
	default:
		return "OK"
	}
}

func (s *Server) getTransactionStatus(state *connState) byte {
	if state.transactionFailed {
		return pgwire.StatusInFailedTx
	}
	if state.inTransaction {
		return pgwire.StatusInTrans
	}
	return pgwire.StatusIdle
}

// mysqlTypeToPGOID maps MySQL type names to PostgreSQL OID
func mysqlTypeToPGOID(mysqlType string) uint32 {
	switch mysqlType {
	case "TINYINT":
		return 16 // bool
	case "SMALLINT":
		return 21 // int2
	case "INT", "INTEGER", "MEDIUMINT":
		return 23 // int4
	case "BIGINT":
		return 20 // int8
	case "FLOAT":
		return 700 // float4
	case "DOUBLE":
		return 701 // float8
	case "DECIMAL", "NUMERIC":
		return 1700 // numeric
	case "DATE":
		return 1082 // date
	case "TIME":
		return 1083 // time
	case "DATETIME", "TIMESTAMP":
		return 1114 // timestamp
	case "VARCHAR", "CHAR", "TEXT", "TINYTEXT", "MEDIUMTEXT", "LONGTEXT":
		return 25 // text
	case "JSON":
		return 114 // json
	case "BLOB", "TINYBLOB", "MEDIUMBLOB", "LONGBLOB", "BINARY", "VARBINARY":
		return 17 // bytea
	default:
		return 25 // text
	}
}

