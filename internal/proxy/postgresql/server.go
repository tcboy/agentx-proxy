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
	"regexp"
	"strings"
	"sync"
	"time"

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

	// Track whether Describe already sent RowDescription for this portal
	describedPortals map[string]bool

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
	name             string
	stmt             *preparedStmt
	params           [][]byte
	formatCodes      []int16
	resultFormatCodes []int16
	described        bool
}

func (s *Server) handleConn(conn net.Conn, connID int) {
	defer conn.Close()

	state := &connState{
		preparedStatements: make(map[string]*preparedStmt),
		portals:            make(map[string]*portal),
		describedPortals:   make(map[string]bool),
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
		// Now read the actual startup message (a fresh 8-byte header)
		if _, err := io.ReadFull(conn, buf); err != nil {
			slog.Error("read startup after SSL deny", "error", err)
			return
		}
	}

	// Parse the startup message
	startupLen := binary.BigEndian.Uint32(buf[:4])
	if startupLen < 8 {
		slog.Error("invalid startup length", "len", startupLen)
		return
	}
	remaining := make([]byte, startupLen-8) // already read 8 bytes (4 len + 4 protocol)
	if _, err := io.ReadFull(conn, remaining); err != nil {
		slog.Error("read startup params", "error", err)
		return
	}

	// Parse parameters from the startup message (after length + protocol)
	params := make(map[string]string)
	if len(remaining) > 1 {
		parts := bytes.Split(remaining[:len(remaining)-1], []byte{0})
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

	if err := s.executeQuery(state, query, connID); err != nil {
		return err
	}

	// Don't send ReadyForQuery for DEALLOCATE - pgx sends these in pipeline mode
	// and doesn't expect ReadyForQuery for them
	nq := normalizeQuery(query)
	if len(nq) >= 11 && nq[:11] == "deallocate " {
		return nil
	}

	return state.w.SendReadyForQuery(s.getTransactionStatus(state))
}

func (s *Server) handleParse(state *connState, body []byte, connID int) error {
	msg := pgwire.ParseParse(body)
	slog.Debug("parse", "conn_id", connID, "name", msg.Name, "query", msg.Query, "paramOIDs", msg.ParameterOIDs)

	state.preparedStatements[msg.Name] = &preparedStmt{
		name:      msg.Name,
		query:     msg.Query,
		paramOIDs: msg.ParameterOIDs,
	}

	return state.w.SendParseComplete()
}

func (s *Server) handleBind(state *connState, body []byte, connID int) error {
	msg := pgwire.ParseBind(body)
	stmt, ok := state.preparedStatements[msg.PreparedStatement]
	if !ok {
		return fmt.Errorf("prepared statement %q not found", msg.PreparedStatement)
	}

	state.portals[msg.Portal] = &portal{
		name:              msg.Portal,
		stmt:              stmt,
		params:            msg.Parameters,
		formatCodes:       msg.ParameterFormatCodes,
		resultFormatCodes: msg.ResultFormatCodes,
		described:         state.describedPortals[msg.PreparedStatement],
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
	slog.Debug("describe", "conn_id", connID, "type", string(rune(msg.Type)), "name", msg.Name)

	if msg.Type == 'S' {
		// Describe prepared statement
		stmt, ok := state.preparedStatements[msg.Name]
		if !ok {
			return fmt.Errorf("prepared statement %q not found", msg.Name)
		}

			// Always send ParameterDescription with the correct number of params.
			// When client sends 0 OIDs, count $N in the query.
			paramOIDs := stmt.paramOIDs
			if len(paramOIDs) == 0 {
				paramOIDs = countParams(stmt.query)
			}
			// pg_type WHERE oid = $1: param should be OID 26 (oid type), not 0.
			if len(paramOIDs) == 1 && paramOIDs[0] == 0 {
				uq := strings.ToUpper(stmt.query)
				if strings.Contains(uq, "PG_TYPE") && strings.Contains(uq, ".OID") {
					paramOIDs = []uint32{26}
				}
			}
			// For INSERT/UPDATE queries, infer param types from table column types.
			uq := strings.ToUpper(stmt.query)
			if strings.Contains(uq, "INSERT INTO") {
				if inferred := s.inferInsertParamTypes(stmt.query); inferred != nil {
					for i, oid := range inferred {
						if i < len(paramOIDs) && paramOIDs[i] == 0 {
							paramOIDs[i] = oid
						}
					}
				}
			}
			// Default remaining unknown param OIDs to text (OID 25).
			for i := range paramOIDs {
				if paramOIDs[i] == 0 {
					paramOIDs[i] = 25 // text
				}
			}
			if err := state.w.SendParameterDescription(paramOIDs); err != nil {
				return err
			}
			// Store resolved param OIDs back into the prepared statement
			stmt.paramOIDs = paramOIDs

		// Try to determine result columns for known queries
		nq := normalizeQuery(stmt.query)
		if nq == "select version()" || (len(nq) >= 14 && nq[:14] == "select version") {
			fields := []pgwire.FieldDescription{
				{Name: "version", TypeOID: 25, TypeSize: -1, FormatCode: 0},
			}
			if err := state.w.SendRowDescription(fields); err != nil {
				return err
			}
			state.describedPortals[msg.Name] = true
		} else if strings.Contains(nq, "pg_namespace") && strings.Contains(nq, "version()") && strings.Contains(nq, "current_setting") {
			fields := []pgwire.FieldDescription{
				{Name: "exists", TypeOID: 25, TypeSize: -1, FormatCode: 0},
				{Name: "version", TypeOID: 25, TypeSize: -1, FormatCode: 0},
				{Name: "numeric_version", TypeOID: 25, TypeSize: -1, FormatCode: 0},
			}
			if err := state.w.SendRowDescription(fields); err != nil {
				return err
			}
			state.describedPortals[msg.Name] = true
		} else if isSelectQuery(stmt.query) {
			dquery := substituteParams(stmt.query)
			if err := s.describeSelectColumns(state, dquery); err != nil {
				slog.Debug("describe columns failed, sending NoData", "error", err)
				state.w.SendNoData()
			} else {
				state.describedPortals[msg.Name] = true
			}
		} else if strings.Contains(uq, "RETURNING") && (strings.Contains(uq, "INSERT INTO") || strings.Contains(uq, "UPDATE")) {
			if err := s.describeReturningColumns(state, stmt.query); err != nil {
				slog.Debug("describe returning columns failed, sending NoData", "error", err)
				state.w.SendNoData()
			} else {
				state.describedPortals[msg.Name] = true
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

func isSelectQuery(q string) bool {
	nq := normalizeQuery(q)
	return len(nq) >= 6 && nq[:6] == "select"
}


// substituteParams replaces $N parameter placeholders with NULL for DESCRIBE queries
func substituteParams(query string) string {
	re := regexp.MustCompile(`\$\d+`)
	return re.ReplaceAllString(query, "NULL")
}

// countParams counts the highest $N parameter in a query and returns a slice
// of uint32 OIDs matching the parameter count. We use OID 0 (unknown) so that
// pgx uses its default text encoding for each Go type.
func countParams(query string) []uint32 {
	re := regexp.MustCompile(`\$(\d+)`)
	matches := re.FindAllStringSubmatch(query, -1)
	if len(matches) == 0 {
		return []uint32{}
	}
	maxParam := 0
	for _, m := range matches {
		n := 0
		fmt.Sscanf(m[1], "%d", &n)
		if n > maxParam {
			maxParam = n
		}
	}
	oids := make([]uint32, maxParam)
	// OID 0 = unknown, lets pgx pick the right encoding per Go type
	return oids
}

// inferInsertParamTypes extracts column names from INSERT INTO ... (cols) VALUES ($1,...)
// and maps them to PG OIDs using the table's MySQL column types.
func (s *Server) inferInsertParamTypes(query string) []uint32 {
	re := regexp.MustCompile(`(?i)INSERT\s+INTO\s+"?\w*"?\."?(\w+)"?\s*\(([^)]+)\)\s*VALUES\s*\(([^)]+)\)`)
	m := re.FindStringSubmatch(query)
	if len(m) < 4 {
		return nil
	}
	tableName := strings.Trim(m[1], `"`)
	colStr := m[2]
	valStr := m[3]

	cols := splitColumnList(colStr)
	vals := splitColumnList(valStr)
	if len(cols) != len(vals) {
		return nil
	}

	paramOIDs := make(map[int]uint32)
	for i, v := range vals {
		v = strings.TrimSpace(v)
		re2 := regexp.MustCompile(`^\$(\d+)$`)
		mm := re2.FindStringSubmatch(v)
		if len(mm) < 2 {
			continue
		}
		paramIdx := 0
		fmt.Sscanf(mm[1], "%d", &paramIdx)
		colName := strings.Trim(strings.TrimSpace(cols[i]), `"`)
		pgOID := s.catalog.GetColumnPGOID(tableName, colName)
		if pgOID != 0 {
			paramOIDs[paramIdx] = pgOID
		}
	}

	if len(paramOIDs) == 0 {
		return nil
	}

	maxParam := 0
	for k := range paramOIDs {
		if k > maxParam {
			maxParam = k
		}
	}
	result := make([]uint32, maxParam)
	for i := 1; i <= maxParam; i++ {
		if oid, ok := paramOIDs[i]; ok {
			result[i-1] = oid
		}
	}
	return result
}

func splitColumnList(s string) []string {
	var result []string
	depth := 0
	current := ""
	for _, ch := range s {
		if ch == '(' {
			depth++
		} else if ch == ')' {
			depth--
		} else if ch == ',' && depth == 0 {
			result = append(result, current)
			current = ""
			continue
		}
		current += string(ch)
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func (s *Server) describeSelectColumns(state *connState, query string) error {
	// pg_type fast path: return RowDescription with exact PostgreSQL column types.
	// In real PG: typname→name(19), typtype→char(18), typelem/typbasetype/typrelid→oid(26).
	uq := strings.ToUpper(query)
	if strings.Contains(uq, "PG_TYPE") && strings.Contains(uq, "TYPTYPE") {
		return state.w.SendRowDescription([]pgwire.FieldDescription{
			{Name: "typname", TypeOID: 19, TypeSize: 64, FormatCode: 0},
			{Name: "typtype", TypeOID: 18, TypeSize: 1, FormatCode: 0},
			{Name: "typelem", TypeOID: 26, TypeSize: 4, FormatCode: 0},
			{Name: "typbasetype", TypeOID: 26, TypeSize: 4, FormatCode: 0},
			{Name: "typrelid", TypeOID: 26, TypeSize: 4, FormatCode: 0},
		})
	}

	translated, err := s.translator.Translate(query)
	if err != nil {
		return err
	}

	// Add LIMIT 0 if not already present
	if !strings.Contains(strings.ToUpper(translated), " LIMIT ") {
		translated += " LIMIT 0"
	}

slog.Debug("describe query", "original", query, "translated", translated)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := s.pool.Query(ctx, translated)
	if err != nil {
		return err
	}
	defer rows.Close()

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return err
	}
	colNames, err := rows.Columns()
	if err != nil {
		return err
	}

	fields := make([]pgwire.FieldDescription, len(colTypes))
	for i, ct := range colTypes {
		fields[i] = pgwire.FieldDescription{
			Name:       colNames[i],
			TypeOID:    mysqlTypeToPGOID(ct.DatabaseTypeName()),
			TypeSize:   -1,
			FormatCode: 0,
		}
	}

	return state.w.SendRowDescription(fields)
}

// describeReturningColumns builds a synthetic SELECT to describe the RETURNING columns
// of an INSERT/UPDATE ... RETURNING statement.
func (s *Server) describeReturningColumns(state *connState, query string) error {
	// Extract RETURNING columns
	re := regexp.MustCompile(`(?i)RETURNING\s+(.+)$`)
	m := re.FindStringSubmatch(query)
	if len(m) < 2 {
		return fmt.Errorf("no RETURNING clause found")
	}
	cols := parseReturningColumns(m[1])

	// Extract table name
	reTbl := regexp.MustCompile(`(?i)(?:INSERT\s+INTO|UPDATE)\s+"?\w*"?\."?(\w+)"?`)
	tm := reTbl.FindStringSubmatch(query)
	if len(tm) < 2 {
		return fmt.Errorf("could not extract table name")
	}
	tableName := tm[1]

	// Build synthetic SELECT with LIMIT 0 to describe columns
	selectCols := make([]string, len(cols))
	for i, c := range cols {
		selectCols[i] = fmt.Sprintf("`%s`", c)
	}
	synthetic := fmt.Sprintf("SELECT %s FROM `%s` LIMIT 0",
		strings.Join(selectCols, ", "), tableName)

	slog.Debug("describe returning columns", "table", tableName, "cols", cols, "synthetic", synthetic)

	return s.describeSelectColumns(state, synthetic)
}

// parseSelectColumns extracts column names from a SELECT query's column list.
// Handles: SELECT t.oid, typname, c.relname AS name FROM ...
func parseSelectColumns(query string) []string {
	upper := strings.ToUpper(query)

	// Find SELECT ... FROM
	selectIdx := strings.Index(upper, "SELECT ")
	if selectIdx < 0 {
		return nil
	}
	afterSelect := query[selectIdx+7:]

	// Find FROM (but not subquery FROM)
	fromIdx := findTopLevelFrom(strings.ToUpper(afterSelect))
	if fromIdx < 0 {
		return nil
	}
	colPart := strings.TrimSpace(afterSelect[:fromIdx])

	if colPart == "*" {
		return nil // wildcard - return nil to signal "all columns"
	}

	// Split by commas (respecting parens)
	var cols []string
	depth := 0
	start := 0
	for i := 0; i < len(colPart); i++ {
		switch colPart[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				cols = append(cols, extractColumnName(colPart[start:i]))
				start = i + 1
			}
		}
	}
	cols = append(cols, extractColumnName(colPart[start:]))
	return cols
}

// findTopLevelFrom finds the first FROM at parenthesis depth 0
func findTopLevelFrom(upper string) int {
	depth := 0
	for i := 0; i < len(upper)-4; i++ {
		switch {
		case upper[i] == '(':
			depth++
		case upper[i] == ')':
			depth--
		case depth == 0 && upper[i:i+4] == "FROM" && (i == 0 || upper[i-1] == ' ' || upper[i-1] == '\n' || upper[i-1] == '\t'):
			// Make sure it's "FROM " not "FROMX"
			if i+4 >= len(upper) || upper[i+4] == ' ' || upper[i+4] == '\n' || upper[i+4] == '\t' {
				return i
			}
		}
	}
	return -1
}

// extractColumnName extracts the column name from a SELECT expression
// Handles: "t.oid" -> "oid", "typname" -> "typname", "c.relname AS name" -> "name"
func extractColumnName(expr string) string {
	expr = strings.TrimSpace(expr)

	// Check for AS alias
	if asIdx := strings.LastIndex(strings.ToUpper(expr), " AS "); asIdx >= 0 {
		return strings.TrimSpace(expr[asIdx+4:])
	}

	// Check for table.column
	if dotIdx := strings.LastIndex(expr, "."); dotIdx >= 0 {
		return expr[dotIdx+1:]
	}

	return expr
}

// catalogColumnOIDs maps common pg_catalog column names to PG type OIDs
var catalogColumnOIDs = map[string]uint32{
	// Common OID columns
	"oid": 23, "typnamespace": 23, "typrelid": 23, "typelem": 23, "typarray": 23,
	"typbasetype": 23, "typtypmod": 23, "typndims": 23, "typcollation": 23,
	"relnamespace": 23, "reltype": 23, "reloftype": 23, "relowner": 23,
	"relam": 23, "relfilenode": 23, "reltablespace": 23, "relpages": 23,
	"reltuples": 701, "relallvisible": 23, "reltoastrelid": 23, "relnatts": 23,
	"relchecks": 23, "attrelid": 23, "atttypid": 23,
	"indexrelid": 23, "indrelid": 23, "indnatts": 23, "indnkeyatts": 23,
	"pronamespace": 23, "proowner": 23, "prolang": 23, "provariadic": 23,
	"pronargs": 21, "pronargdefaults": 21, "prorettype": 23,
	"enumtypid": 23, "connamespace": 23, "conrelid": 23, "contypid": 23,
	"conindid": 23, "confrelid": 23,
	"datdba": 23, "extowner": 23, "extnamespace": 23,
	// Boolean columns
	"typnotnull": 16, "relhasindex": 16, "relisshared": 16, "relhaspkey": 16,
	"attnotnull": 16, "atthasdef": 16, "indisunique": 16, "indisprimary": 16,
	// String columns (default)
	"typname": 25, "typtype": 25, "typcategory": 25,
	"typinput": 25, "typoutput": 25, "typalign": 25, "typstorage": 25,
	"relname": 25, "relkind": 25, "relpersistence": 25,
	"attname": 25, "attstorage": 25, "attalign": 25,
	"indkey": 25,
	"nspname": 25,
	"conname": 25, "contype": 25,
	"proname": 25, "prokind": 25,
	"enumlabel": 25,
	"datname": 25,
	"extname": 25, "extversion": 25,
	// Float columns
	"procost": 701, "prorows": 701, "enumsortorder": 701,
}

// describeCatalogQuery handles the describe phase for catalog queries.
// It uses the catalog handler to get column info and filters to match the SELECT clause.
func (s *Server) describeCatalogQuery(query string) []pgwire.FieldDescription {
	ctx := context.Background()
	allColumns, _, err := s.catalog.HandleCatalogQuery(ctx, query)
	if err != nil || allColumns == nil {
		return nil
	}

	selectedCols := parseSelectColumns(query)
	if selectedCols == nil {
		// SELECT * or couldn't parse - return all columns
		selectedCols = allColumns
	}

	// Build a column name → index map for the catalog's full column list
	colIndex := make(map[string]int, len(allColumns))
	for i, c := range allColumns {
		colIndex[c] = i
	}

	fields := make([]pgwire.FieldDescription, 0, len(selectedCols))
	for _, col := range selectedCols {
		colLower := strings.ToLower(col)
		if _, ok := colIndex[colLower]; !ok {
			// Column not found in catalog - skip or use text default
			oid := catalogColumnOIDs[colLower]
			if oid == 0 {
				oid = 25 // text
			}
			fields = append(fields, pgwire.FieldDescription{
				Name: colLower, TypeOID: oid, TypeSize: -1, FormatCode: 0,
			})
			continue
		}
		oid := catalogColumnOIDs[colLower]
		if oid == 0 {
			oid = 25 // text
		}
		fields = append(fields, pgwire.FieldDescription{
			Name: colLower, TypeOID: oid, TypeSize: -1, FormatCode: 0,
		})
	}

	return fields
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
	// Handle empty/comment-only queries (e.g. pgx's "-- ping")
	nq := normalizeQuery(query)
	if nq == "" || strings.HasPrefix(nq, "--") {
		return state.w.SendEmptyQueryResponse()
	}

	// Ignore CREATE SCHEMA — MySQL doesn't use schemas
	if strings.HasPrefix(nq, "create schema") {
		return state.w.SendCommandComplete("CREATE SCHEMA")
	}

	// Handle DO $$ ... $$ blocks (PL/pgSQL anonymous blocks from Prisma migration engine)
	if strings.HasPrefix(nq, "do") {
		return state.w.SendCommandComplete("DO")
	}

	// Handle pg_advisory_lock / pg_advisory_unlock — Prisma uses these for migration locking
	if strings.Contains(nq, "pg_advisory_lock") || strings.Contains(nq, "pg_advisory_unlock") {
		if err := state.w.SendRowDescription([]pgwire.FieldDescription{
			{Name: "pg_advisory_lock", TypeOID: 16, TypeSize: 1, FormatCode: 0}, // bool
		}); err != nil {
			return err
		}
		if err := state.w.SendDataRow([]interface{}{true}, nil); err != nil {
			return err
		}
		return state.w.SendCommandComplete("SELECT 1")
	}

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


		case len(q) >= 11 && q[:11] == "deallocate ":
			// DEALLOCATE prepared statement - just acknowledge
			return state.w.SendCommandComplete("DEALLOCATE")

	case len(q) >= 4 && q[:4] == "set ":
		// SET TRANSACTION ISOLATION LEVEL - MySQL can't change this inside a txn
		if strings.Contains(q, "isolation level") {
			return state.w.SendCommandComplete("SET")
		}
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

	// Handle special queries that don't need MySQL execution
	nq := normalizeQuery(query)

	// Handle pg_advisory_lock / pg_advisory_unlock in extended protocol
	if strings.Contains(nq, "pg_advisory_lock") || strings.Contains(nq, "pg_advisory_unlock") {
		slog.Debug("handling pg_advisory_lock in extended protocol", "conn_id", connID, "described", p.described)
		if !p.described {
			if err := state.w.SendRowDescription([]pgwire.FieldDescription{
				{Name: "pg_advisory_lock", TypeOID: 16, TypeSize: 1, FormatCode: 0},
			}); err != nil {
				return err
			}
		}
		if err := state.w.SendDataRow([]interface{}{true}, nil); err != nil {
			return err
		}
		return state.w.SendCommandComplete("SELECT 1")
	}

	// Handle Prisma migration engine startup query:
	// SELECT EXISTS(SELECT 1 FROM pg_namespace WHERE nspname = $1), version(), current_setting('server_version_num')::integer as numeric_version
	if strings.Contains(nq, "pg_namespace") && strings.Contains(nq, "version()") && strings.Contains(nq, "current_setting") {
		slog.Debug("handling Prisma migration engine startup query", "conn_id", connID)
		return s.handlePrismaStartupQuery(state, p, connID)
	}

	if nq == "select version()" || (len(nq) >= 14 && nq[:14] == "select version") {
		slog.Debug("handling SELECT version() in extended protocol", "conn_id", connID, "described", p.described)
		if !p.described {
			fields := []pgwire.FieldDescription{
				{Name: "version", TypeOID: 25, TypeSize: -1, FormatCode: 0},
			}
			if err := state.w.SendRowDescription(fields); err != nil {
				return fmt.Errorf("send row desc: %w", err)
			}
		}
		if err := state.w.SendDataRow([]interface{}{"PostgreSQL 14.0 on x86_64-pc-linux-gnu, compiled by gcc, 64-bit"}, nil); err != nil {
			return fmt.Errorf("send data row: %w", err)
		}
		if err := state.w.SendCommandComplete("SELECT 1"); err != nil {
			return fmt.Errorf("send command complete: %w", err)
		}
		return nil
	}

	// Check for pg_catalog queries in extended protocol
	if s.catalog.IsCatalogQuery(query) {
		slog.Debug("handling catalog query in extended protocol", "conn_id", connID, "sql", query)
		return s.executeCatalogQueryExtended(state, p, connID)
	}

	// Translate the query
	translated, err := s.translator.Translate(query)
	if err != nil {
		return fmt.Errorf("translate: %w", err)
	}

	// For INSERT/UPDATE/DELETE ... RETURNING, handle specially
	hasRet := strings.Contains(strings.ToUpper(query), "RETURNING")
	isWrite := isWriteQuery(translated)
	slog.Debug("write returning check", "conn_id", connID, "hasReturning", hasRet, "isWrite", isWrite, "translated", translated)
	if hasRet && isWrite {
		return s.executeWriteReturning(state, translated, query, p, connID)
	}

	// If there are parameters, we need to substitute them
	if len(p.params) > 0 {
		return s.executeParameterizedMySQLQuery(state, translated, p, connID)
	}

	return s.executeMySQLQuery(state, translated, connID, p)
}

// handlePrismaStartupQuery handles Prisma's migration engine startup query that checks
// for pg_namespace, version(), and current_setting() in a single query
func (s *Server) handlePrismaStartupQuery(state *connState, p *portal, connID int) error {
	fields := []pgwire.FieldDescription{
		{Name: "exists", TypeOID: 25, TypeSize: -1, FormatCode: 0},
		{Name: "version", TypeOID: 25, TypeSize: -1, FormatCode: 0},
		{Name: "numeric_version", TypeOID: 25, TypeSize: -1, FormatCode: 0},
	}

	if !p.described {
		if err := state.w.SendRowDescription(fields); err != nil {
			return err
		}
	}

	// Check if the schema (from $1 parameter) is "public" - always return true
	schemaExists := true
	if len(p.params) > 0 && p.params[0] != nil {
		paramSchema := strings.ToLower(string(p.params[0]))
		if paramSchema != "public" && paramSchema != "pg_catalog" {
			schemaExists = false
		}
	}

	if err := state.w.SendDataRow([]interface{}{
		func() string { if schemaExists { return "true" }; return "false" }(),
		"PostgreSQL 14.0 on x86_64-pc-linux-gnu, compiled by gcc, 64-bit",
		"140000",
	}, nil); err != nil {
		return err
	}

	return state.w.SendCommandComplete("SELECT 1")
}

// executeCatalogQueryExtended handles catalog queries in the extended protocol
// by substituting parameters and routing to the catalog handler
func (s *Server) executeCatalogQueryExtended(state *connState, p *portal, connID int) error {
	slog.Debug("executeCatalogQueryExtended", "conn_id", connID, "described", p.described, "param_count", len(p.params), "formatCodes", p.formatCodes)
	// Substitute $N parameters with actual values for catalog query handling
	query := p.stmt.query
	if len(p.params) > 0 {
		for i := len(p.params) - 1; i >= 0; i-- {
			paramIdx := i + 1
			var val string
			if p.params[i] == nil {
				val = "NULL"
			} else {
				raw := p.params[i]
				// Binary format: 4-byte big-endian integer (OID, int4, etc.)
				binary := len(p.formatCodes) > 0 && p.formatCodes[0] == 1
				if binary && len(raw) == 4 {
					intVal := uint32(raw[0])<<24 | uint32(raw[1])<<16 | uint32(raw[2])<<8 | uint32(raw[3])
					val = fmt.Sprintf("%d", intVal)
				} else if binary && len(raw) == 2 {
					intVal := uint32(raw[0])<<8 | uint32(raw[1])
					val = fmt.Sprintf("%d", intVal)
				} else if binary && len(raw) == 8 {
					intVal := uint64(raw[0])<<56 | uint64(raw[1])<<48 | uint64(raw[2])<<40 | uint64(raw[3])<<32 |
						uint64(raw[4])<<24 | uint64(raw[5])<<16 | uint64(raw[6])<<8 | uint64(raw[7])
					val = fmt.Sprintf("%d", intVal)
				} else {
					val = fmt.Sprintf("'%s'", strings.ReplaceAll(string(raw), "'", "''"))
				}
			}
			slog.Debug("param substitution", "conn_id", connID, "param_idx", paramIdx, "raw_hex", fmt.Sprintf("%x", p.params[i]), "val", val)
			query = strings.ReplaceAll(query, fmt.Sprintf("$%d", paramIdx), val)
		}
	}
	slog.Debug("substituted query", "conn_id", connID, "query", query)

	return s.executeCatalogQuery(state, query, connID, p.described)
}

func (s *Server) executeCatalogQuery(state *connState, query string, connID int, described ...bool) error {
	ctx := context.Background()

	allColumns, data, err := s.catalog.HandleCatalogQuery(ctx, query)
	if err != nil {
		return err
	}

	if allColumns == nil && data == nil {
		return s.executeMySQLQuery(state, query, connID)
	}

	// Determine which columns the query selects
	selectedCols := parseSelectColumns(query)
	columns := allColumns
	var colIndices []int

	if selectedCols != nil {
		colIndexMap := make(map[string]int, len(allColumns))
		for i, c := range allColumns {
			colIndexMap[c] = i
		}
		colIndices = make([]int, 0, len(selectedCols))
		columns = make([]string, 0, len(selectedCols))
		for _, sc := range selectedCols {
			scLower := strings.ToLower(sc)
			if idx, ok := colIndexMap[scLower]; ok {
				colIndices = append(colIndices, idx)
				columns = append(columns, scLower)
			} else {
				colIndices = append(colIndices, -1)
				columns = append(columns, scLower)
			}
		}
	}

	// Send row description (skip if already sent during Describe phase)
	skipRowDesc := len(described) > 0 && described[0]
	slog.Debug("executeCatalogQuery row description", "conn_id", connID, "skipRowDesc", skipRowDesc, "num_columns", len(columns))
	if !skipRowDesc {
		fields := make([]pgwire.FieldDescription, len(columns))
		for i, col := range columns {
			oid := catalogColumnOIDs[col]
			if oid == 0 {
				oid = 25
			}
			fields[i] = pgwire.FieldDescription{
				Name:       col,
				TypeOID:    oid,
				TypeSize:   -1,
				FormatCode: 0,
			}
		}

		if err := state.w.SendRowDescription(fields); err != nil {
			return err
		}
	}

	// Send data rows filtered to selected columns
	slog.Debug("catalog query result", "conn_id", connID, "num_rows", len(data), "columns", columns)
	for i, row := range data {
		var filteredRow []interface{}
		if colIndices != nil {
			filteredRow = make([]interface{}, len(colIndices))
			for j, idx := range colIndices {
				if idx >= 0 && idx < len(row) {
					filteredRow[j] = row[idx]
				} else {
					filteredRow[j] = nil
				}
			}
		} else {
			filteredRow = row
		}
		slog.Debug("sending data row", "conn_id", connID, "row_idx", i, "values", filteredRow)
		if err := state.w.SendDataRow(filteredRow, nil); err != nil {
			return err
		}
	}

	return state.w.SendCommandComplete(fmt.Sprintf("SELECT %d", len(data)))
}

func (s *Server) executeMySQLQuery(state *connState, query string, connID int, p ...*portal) error {
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

		// Determine result format codes from portal (for extended protocol binary results)
		var typeOIDs []uint32
		_skipRow := false
		if len(p) > 0 && p[0] != nil {
			typeOIDs = make([]uint32, len(columns))
			for i, col := range columns {
				typeOIDs[i] = col.TypeOID
			}
			_skipRow = p[0].described

			// If binary format requested, update RowDescription FormatCodes
			if len(p[0].resultFormatCodes) > 0 && p[0].resultFormatCodes[0] == 1 {
				for i := range columns {
					if canBinaryEncode(columns[i].TypeOID) {
						columns[i].FormatCode = 1
					}
				}
			}
		}

		// Build per-column format codes matching RowDescription
		colFmt := make([]int16, len(columns))
		for i, col := range columns {
			colFmt[i] = col.FormatCode
		}

		if !_skipRow {
		if err := state.w.SendRowDescription(columns); err != nil {
			return err
		}
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

		if err := state.w.SendDataRow(values, colFmt, typeOIDs); err != nil {
			return err
		}
		rowCount++
	}

	if err := rows.Err(); err != nil {
		return err
	}

	return state.w.SendCommandComplete(fmt.Sprintf("SELECT %d", rowCount))
}

func (s *Server) executeParameterizedMySQLQuery(state *connState, query string, p *portal, connID int) error {
	ctx := context.Background()

	// Convert params to interface slice, handling binary array types
	args := make([]interface{}, len(p.params))
	for i, param := range p.params {
		fc := int16(0)
		if len(p.formatCodes) > 0 {
			fc = p.formatCodes[0]
		}
		var oid uint32
		if i < len(p.stmt.paramOIDs) {
			oid = p.stmt.paramOIDs[i]
		}
		args[i] = convertPGParam(param, fc, oid)
		slog.Debug("param", "conn_id", connID, "idx", i, "fc", fc, "oid", oid, "raw_hex", fmt.Sprintf("%x", param), "val", fmt.Sprintf("%v", args[i]))
	}

	// Detect write queries upfront
	if isWriteQuery(query) {
		result, err := s.pool.Exec(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("exec parameterized: %w (query: %s)", err, query)
		}
		affected, _ := result.RowsAffected()
		tag := getCommandTag(query, affected, 0)
		return state.w.SendCommandComplete(tag)
	}

	slog.Debug("executing parameterized MySQL query", "conn_id", connID, "query", query, "args", fmt.Sprintf("%v", args))
	rows, err := s.pool.Query(ctx, query, args...)
	slog.Debug("parameterized query result", "conn_id", connID, "error", err)
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

		// Determine result format codes from portal
		var resultFmt []int16
		typeOIDs := make([]uint32, len(columns))
		for i, col := range columns {
			typeOIDs[i] = col.TypeOID
		}
		if len(p.resultFormatCodes) > 0 && p.resultFormatCodes[0] == 1 {
			resultFmt = p.resultFormatCodes
			for i := range columns {
				if canBinaryEncode(columns[i].TypeOID) {
					columns[i].FormatCode = 1
				}
			}
		}

	if !p.described {
		if err := state.w.SendRowDescription(columns); err != nil {
			return err
		}
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

		if err := state.w.SendDataRow(values, resultFmt, typeOIDs); err != nil {
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
// canBinaryEncode returns true if we can encode the given OID in binary format.
func canBinaryEncode(oid uint32) bool {
	switch oid {
	case 16, 21, 23, 20, 700, 701, 1114: // bool, int2, int4, int8, float4, float8, timestamp
		return true
	default:
		return false
	}
}

func mysqlTypeToPGOID(mysqlType string) uint32 {
	switch mysqlType {
	case "TINYINT":
		return 21 // int2 (MySQL stores 0/1, not true/false)
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
		return 25 // text (avoid binary numeric encoding issues)
	case "DATE":
		return 1082 // date
	case "TIME":
		return 1083 // time
	case "DATETIME", "TIMESTAMP":
		return 25 // text (avoid binary timestamp encoding issues)
	case "VARCHAR", "CHAR", "TEXT", "TINYTEXT", "MEDIUMTEXT", "LONGTEXT":
		return 25 // text
	case "JSON":
		return 3802 // jsonb
	case "BLOB", "TINYBLOB", "MEDIUMBLOB", "LONGBLOB", "BINARY", "VARBINARY":
		return 17 // bytea
	default:
		return 25 // text
	}
}


// executeWriteReturning handles INSERT/UPDATE ... RETURNING by:
// 1. Executing the write (INSERT)
// 2. SELECTing back the row by primary key
// 3. Sending DataRow + CommandComplete
func (s *Server) executeWriteReturning(state *connState, translated, origQuery string, p *portal, connID int) error {
	ctx := context.Background()

	// Convert params to interface slice, handling binary array types
	args := make([]interface{}, len(p.params))
	for i, param := range p.params {
		fc := int16(0)
		if len(p.formatCodes) > 0 {
			fc = p.formatCodes[0]
		}
		var oid uint32
		if i < len(p.stmt.paramOIDs) {
			oid = p.stmt.paramOIDs[i]
		}
		args[i] = convertPGParam(param, fc, oid)
	}

	// Step 1: Execute the write
	result, err := s.pool.Exec(ctx, translated, args...)
	if err != nil {
		return fmt.Errorf("exec parameterized: %w (query: %s)", err, translated)
	}
	_ = result

	// Step 2: Extract RETURNING columns and table name
	re := regexp.MustCompile(`(?i)RETURNING\s+(.+)$`)
	m := re.FindStringSubmatch(origQuery)
	if len(m) < 2 {
		return state.w.SendCommandComplete("INSERT 0 1")
	}
	returningCols := m[1]
	cols := parseReturningColumns(returningCols)

	reTbl := regexp.MustCompile(`(?i)(?:INSERT\s+INTO|UPDATE)\s+"?\w*"?\."?(\w+)"?`)
	tm := reTbl.FindStringSubmatch(origQuery)
	if len(tm) < 2 {
		return state.w.SendCommandComplete("INSERT 0 1")
	}
	tableName := tm[1]

	// Step 3: SELECT back the row using the first param (id)
	if len(p.params) == 0 {
		return state.w.SendCommandComplete("INSERT 0 1")
	}
	idVal := string(p.params[0])
	selectCols := make([]string, len(cols))
	for i, c := range cols {
		selectCols[i] = fmt.Sprintf("`%s`", c)
	}
	selectQuery := fmt.Sprintf("SELECT %s FROM `%s` WHERE `id` = ?",
		strings.Join(selectCols, ", "), tableName)

	rows, err := s.pool.Query(ctx, selectQuery, idVal)
	if err != nil {
		return state.w.SendCommandComplete("INSERT 0 1")
	}
	defer rows.Close()

	colTypes, _ := rows.ColumnTypes()
	scanVals := make([]interface{}, len(colTypes))
	for i := range scanVals {
		scanVals[i] = new(interface{})
	}
	for rows.Next() {
		if err := rows.Scan(scanVals...); err != nil {
			continue
		}
		rowVals := make([]interface{}, len(scanVals))
		for i, v := range scanVals {
			ptr := v.(*interface{})
			if *ptr == nil {
				rowVals[i] = nil
				continue
			}
			switch tv := (*ptr).(type) {
			case []byte:
				rowVals[i] = string(tv)
			case time.Time:
				rowVals[i] = tv.Format("2006-01-02 15:04:05.000 -0700")
			default:
				rowVals[i] = fmt.Sprintf("%v", *ptr)
			}
		}
		if err := state.w.SendDataRow(rowVals, nil); err != nil {
			return err
		}
	}

	return state.w.SendCommandComplete("INSERT 0 1")
}

func parseReturningColumns(cols string) []string {
	parts := strings.Split(cols, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		// Strip schema: "public"."users"."id" -> "id"
		if idx := strings.LastIndex(p, "."); idx >= 0 {
			p = p[idx+1:]
		}
		p = strings.Trim(p, "\"`")
		// Strip AS alias
		if idx := strings.Index(strings.ToUpper(p), " AS "); idx >= 0 {
			p = p[:idx]
		}
		result = append(result, p)
	}
	return result
}

// pgBinaryArrayToJSON converts a PostgreSQL binary-encoded array parameter
// to a JSON string suitable for MySQL JSON columns.
func pgBinaryArrayToJSON(data []byte) string {
	if len(data) == 0 {
		return "[]"
	}
	// PG binary array format:
	// int32: number of dimensions
	// int32: flags (1 = has null bitmap)
	// int32: element OID
	// For each dimension: int32 length
	// Null bitmap (variable)
	// Elements (variable)
	
	dims := int(binary.BigEndian.Uint32(data[0:4]))
	if dims == 0 {
		return "[]"
	}
	
	flags := int(binary.BigEndian.Uint32(data[4:8]))
	_ = flags
	elemOID := binary.BigEndian.Uint32(data[8:12])
	_ = elemOID
	
	// For 1-dimensional array
	if dims == 1 {
		length := int(binary.BigEndian.Uint32(data[12:16]))
		lowerBound := int(binary.BigEndian.Uint32(data[16:20]))
		_ = lowerBound
		
		// Null bitmap
		nullBitmapSize := (length + 7) / 8
		nullBitmap := data[20 : 20+nullBitmapSize]
		
		// Elements start after null bitmap
		offset := 20 + nullBitmapSize
		elements := make([]string, 0, length)
		
		for i := 0; i < length; i++ {
			// Check null bitmap
			byteIdx := i / 8
			bitIdx := uint(i % 8)
			isNull := byteIdx < len(nullBitmap) && (nullBitmap[byteIdx]&(1<<bitIdx)) == 0
			
			if isNull {
				elements = append(elements, "null")
				continue
			}
			
			// Read element length
			if offset+4 > len(data) {
				break
			}
			elemLen := int(int32(binary.BigEndian.Uint32(data[offset : offset+4])))
			offset += 4
			
			if elemLen == -1 {
				elements = append(elements, "null")
				continue
			}
			
			if offset+elemLen > len(data) {
				break
			}
			elemData := string(data[offset : offset+elemLen])
			// Escape for JSON
			escaped := strings.ReplaceAll(elemData, `\`, `\\`)
			escaped = strings.ReplaceAll(escaped, `"`, `\"`)
			elements = append(elements, `"`+escaped+`"`)
			offset += elemLen
		}
		
		return "[" + strings.Join(elements, ",") + "]"
	}
	
	return "[]"
}

// convertPGParam converts a PG wire protocol parameter to a MySQL-compatible value.
func convertPGParam(data []byte, formatCode int16, paramOID uint32) interface{} {
	if data == nil {
		return nil
	}
	if formatCode == 0 {
		// Text format
		return string(data)
	}
	// Binary format
	// Check if it's an array type (OID >= 1000 for array types)
	if paramOID >= 1000 && len(data) > 12 {
		return pgBinaryArrayToJSON(data)
	}
	// For scalar types in binary format, return as string
	return string(data)
}
