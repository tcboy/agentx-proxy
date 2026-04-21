package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/agentx-labs/agentx-proxy/internal/config"
	"github.com/agentx-labs/agentx-proxy/internal/mysql"
)

// HTTPServer implements ClickHouse HTTP interface proxy
type HTTPServer struct {
	server     *http.Server
	cfg        *config.Config
	pool       *mysql.Pool
	translator *CHTranslator
	buffer     *WriteBuffer
}

func NewHTTPServer(cfg *config.Config, pool *mysql.Pool) (*HTTPServer, error) {
	s := &HTTPServer{
		cfg:        cfg,
		pool:       pool,
		translator: NewCHTranslator(),
		buffer:     NewWriteBuffer(cfg.Proxy.CHToMySQL.WriteBufferSize, cfg.Proxy.CHToMySQL.WriteFlushInterval, pool),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)

	s.server = &http.Server{
		Addr:    cfg.Listen.ClickHouseHTTP,
		Handler: mux,
	}

	return s, nil
}

func (s *HTTPServer) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.Listen.ClickHouseHTTP)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		s.server.Shutdown(context.Background())
	}()

	return s.server.Serve(ln)
}

func (s *HTTPServer) Close() error {
	if s.buffer != nil {
		s.buffer.Flush()
	}
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

func (s *HTTPServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Get query from URL params or body
	query := r.URL.Query().Get("query")
	if query == "" && r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		query = string(body)
	}

	query = strings.TrimSpace(query)
	if query == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	slog.Debug("CH HTTP query", "sql", query)

	// Handle system queries
	if resp := s.handleSystemQuery(query); resp != "" {
		w.Header().Set("Content-Type", "text/tab-separated-values")
		w.Write([]byte(resp))
		return
	}

	// Translate CH SQL to MySQL SQL
	translated, err := s.translator.Translate(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	slog.Debug("CH translated", "original", query, "translated", translated)

	// Execute
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if isWriteQuery(translated) {
		// Check for batch insert
		if strings.HasPrefix(strings.ToUpper(translated), "INSERT") {
			s.buffer.Enqueue(translated)
			w.WriteHeader(http.StatusOK)
			return
		}

		result, err := s.pool.Exec(ctx, translated)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		affected, _ := result.RowsAffected()
		w.Write([]byte(fmt.Sprintf("%d\n", affected)))
		return
	}

	rows, err := s.pool.Query(ctx, translated)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Format as TabSeparatedWithNames
	cols, err := rows.Columns()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Column names
	w.Header().Set("Content-Type", "text/tab-separated-values")
	w.Write([]byte(strings.Join(cols, "\t") + "\n"))

	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		strValues := make([]string, len(values))
		for i, v := range values {
			if v == nil {
				strValues[i] = "\\N"
			} else {
				strValues[i] = fmt.Sprintf("%v", v)
			}
		}

		w.Write([]byte(strings.Join(strValues, "\t") + "\n"))
	}
}

func (s *HTTPServer) handleSystemQuery(query string) string {
	upper := strings.ToUpper(strings.TrimSpace(query))

	// SELECT version()
	if strings.Contains(upper, "VERSION()") {
		return "24.8.5.115\n"
	}

	// SELECT currentUser()
	if strings.Contains(upper, "CURRENTUSER()") || strings.Contains(upper, "CURRENT_USER()") {
		return "default\n"
	}

	// SELECT database()
	if strings.Contains(upper, "DATABASE()") && !strings.Contains(upper, "SYSTEM") {
		return "default\n"
	}

	// SHOW TABLES
	if upper == "SHOW TABLES" || upper == "SHOW TABLES FROM default" {
		return s.getTableList()
	}

	// SHOW DATABASES
	if upper == "SHOW DATABASES" {
		return "default\ninformation_schema\n"
	}

	// SELECT * FROM system.tables
	if strings.Contains(upper, "SYSTEM.TABLES") {
		return s.getSystemTables()
	}

	// SELECT * FROM system.columns
	if strings.Contains(upper, "SYSTEM.COLUMNS") {
		return s.getSystemColumns()
	}

	// EXISTS TABLE
	if strings.HasPrefix(upper, "EXISTS") {
		return s.existsTable(query)
	}

	return ""
}

func (s *HTTPServer) getTableList() string {
	return getTableList(s.pool)
}

func getTableList(pool *mysql.Pool) string {
	ctx := context.Background()
	rows, err := pool.Query(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'
	`)
	if err != nil {
		return ""
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		tables = append(tables, name)
	}

	return strings.Join(tables, "\n") + "\n"
}

func (s *HTTPServer) getSystemTables() string {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx, `
		SELECT table_name as name, 'MergeTree' as engine
		FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'
	`)
	if err != nil {
		return ""
	}
	defer rows.Close()

	cols := []string{"database", "name", "engine", "is_temporary", "data_paths", "metadata_path", "metadata_modification_time", "metadata_version", "storage_policy", "delayed_insert_threads", "parts", "active_parts", "total_marks", "total_rows", "total_bytes", "lifetime_rows", "lifetime_bytes"}
	_ = cols

	var result strings.Builder
	result.WriteString(strings.Join(cols, "\t") + "\n")

	for rows.Next() {
		var name, engine string
		rows.Scan(&name, &engine)
		result.WriteString(fmt.Sprintf("default\t%s\t%s\t0\t\t\t\t\t\t0\t0\t0\t0\t0\t0\t0\t\t\t\t\n", name, engine))
	}

	return result.String()
}

func (s *HTTPServer) getSystemColumns() string {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx, `
		SELECT table_name, column_name, column_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = DATABASE()
		ORDER BY table_name, ordinal_position
	`)
	if err != nil {
		return ""
	}
	defer rows.Close()

	cols := []string{"database", "table", "name", "type", "default_expression", "comment", "codec_expression", "default_type"}

	var result strings.Builder
	result.WriteString(strings.Join(cols, "\t") + "\n")

	for rows.Next() {
		var table, name, colType string
		var nullable, defaultVal *string
		rows.Scan(&table, &name, &colType, &nullable, &defaultVal)

		chType := pgTypeFromMySQL(colType)
		result.WriteString(fmt.Sprintf("default\t%s\t%s\t%s\t\t\t\t\n", table, name, chType))
	}

	return result.String()
}

func (s *HTTPServer) existsTable(query string) string {
	// Extract table name from EXISTS TABLE <name>
	parts := strings.Fields(query)
	if len(parts) >= 4 {
		tableName := parts[len(parts)-1]
		tableName = strings.Trim(tableName, "`")

		ctx := context.Background()
		exists, err := s.pool.TableExists(ctx, tableName)
		if err != nil {
			return "0\n"
		}
		if exists {
			return "1\n"
		}
	}
	return "0\n"
}

func pgTypeFromMySQL(mysqlType string) string {
	lower := strings.ToLower(mysqlType)
	switch {
	case strings.HasPrefix(lower, "tinyint(1)"):
		return "UInt8"
	case strings.HasPrefix(lower, "smallint"):
		return "Int16"
	case strings.HasPrefix(lower, "mediumint"):
		return "Int24"
	case strings.HasPrefix(lower, "bigint"):
		return "Int64"
	case strings.HasPrefix(lower, "int"), strings.HasPrefix(lower, "integer"):
		return "Int32"
	case strings.HasPrefix(lower, "decimal"):
		return "Decimal64(12)"
	case strings.HasPrefix(lower, "float"):
		return "Float32"
	case strings.HasPrefix(lower, "double"):
		return "Float64"
	case strings.HasPrefix(lower, "datetime"):
		return "DateTime64(3)"
	case strings.HasPrefix(lower, "date"):
		return "Date"
	case strings.HasPrefix(lower, "json"):
		return "Map(LowCardinality(String), String)"
	case strings.HasPrefix(lower, "longtext"), strings.HasPrefix(lower, "mediumtext"),
		strings.HasPrefix(lower, "text"):
		return "String"
	case strings.HasPrefix(lower, "varchar"), strings.HasPrefix(lower, "char"):
		return "String"
	default:
		return "String"
	}
}

func isWriteQuery(q string) bool {
	upper := strings.ToUpper(strings.TrimSpace(q))
	return strings.HasPrefix(upper, "INSERT") ||
		strings.HasPrefix(upper, "UPDATE") ||
		strings.HasPrefix(upper, "DELETE") ||
		strings.HasPrefix(upper, "CREATE") ||
		strings.HasPrefix(upper, "ALTER") ||
		strings.HasPrefix(upper, "DROP") ||
		strings.HasPrefix(upper, "TRUNCATE")
}

// NativeServer implements ClickHouse Native (TCP) protocol proxy
type NativeServer struct {
	listener   net.Listener
	cfg        *config.Config
	pool       *mysql.Pool
	translator *CHTranslator
	buffer     *WriteBuffer
	connCount  int
	mu         sync.Mutex
}

func NewNativeServer(cfg *config.Config, pool *mysql.Pool) (*NativeServer, error) {
	ln, err := net.Listen("tcp", cfg.Listen.ClickHouseNative)
	if err != nil {
		return nil, err
	}

	return &NativeServer{
		listener:   ln,
		cfg:        cfg,
		pool:       pool,
		translator: NewCHTranslator(),
		buffer:     NewWriteBuffer(cfg.Proxy.CHToMySQL.WriteBufferSize, cfg.Proxy.CHToMySQL.WriteFlushInterval, pool),
	}, nil
}

func (s *NativeServer) Start(ctx context.Context) error {
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
				slog.Error("CH native accept error", "error", err)
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

func (s *NativeServer) Close() error {
	if s.buffer != nil {
		s.buffer.Flush()
	}
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

func (s *NativeServer) handleConn(conn net.Conn, connID int) {
	defer conn.Close()

	slog.Info("new CH native connection", "conn_id", connID, "remote", conn.RemoteAddr())
	defer slog.Info("CH native connection closed", "conn_id", connID)

	// CH Native protocol handshake
	// Read client hello
	buf := make([]byte, 1024)
	_, err := conn.Read(buf)
	if err != nil {
		slog.Error("read CH hello", "error", err)
		return
	}

	// Send server hello
	serverHello := []byte{
		0xDC, 0xD4, 0x00, 0x00, // Version: 54460
	}
	// Write version + server name + revision
	conn.Write(serverHello)
	conn.Write([]byte{byte(len("AgentX Proxy"))})
	conn.Write([]byte("AgentX Proxy"))
	conn.Write([]byte{0x00, 0x00}) // Minor version
	conn.Write([]byte{0x01, 0x00, 0x00, 0x00}) // Revision: 54461

	// Process queries
	for {
		// Read packet type
		packetType := make([]byte, 1)
		if _, err := conn.Read(packetType); err != nil {
			return
		}

		switch packetType[0] {
		case 1: // Query
			s.handleCHQuery(conn, connID)
		case 2: // Data
			// Skip data
		case 6: // Cancel
			return
		case 7: // Hello
			// Already handled
		default:
			slog.Warn("unknown CH packet", "type", packetType[0])
		}
	}
}

func (s *NativeServer) handleCHQuery(conn net.Conn, connID int) error {
	// Read query stage
	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err != nil {
		return err
	}

	// Read query type
	if _, err := conn.Read(buf); err != nil {
		return err
	}

	// Read query ID length and value
	idLen := make([]byte, 1)
	if _, err := conn.Read(idLen); err != nil {
		return err
	}
	queryID := make([]byte, idLen[0])
	if _, err := conn.Read(queryID); err != nil {
		return err
	}

	// Read query stage
	if _, err := conn.Read(buf); err != nil {
		return err
	}

	// Read client info
	if _, err := conn.Read(buf); err != nil {
		return err
	}

	// Read query
	queryLenBytes := make([]byte, 4)
	if _, err := conn.Read(queryLenBytes); err != nil {
		return err
	}

	// Handle empty queries
	queryLen := int(queryLenBytes[3])<<24 | int(queryLenBytes[2])<<16 | int(queryLenBytes[1])<<8 | int(queryLenBytes[0])
	if queryLen > 1000000 {
		// Probably a different encoding - read until we find something reasonable
		queryLen = 1024
	}

	queryBuf := make([]byte, queryLen)
	n, err := conn.Read(queryBuf)
	if err != nil {
		return err
	}

	query := string(queryBuf[:n])
	slog.Debug("CH native query", "conn_id", connID, "sql", query)

	// Handle system queries
	if resp := s.handleSystemQueryNative(query); resp != nil {
		return s.sendCHResponse(conn, resp)
	}

	// Translate and execute
	translated, err := s.translator.Translate(query)
	if err != nil {
		return s.sendCHError(conn, err.Error())
	}

	slog.Debug("CH native translated", "original", query, "translated", translated)

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if isWriteQuery(translated) {
		if strings.HasPrefix(strings.ToUpper(translated), "INSERT") {
			s.buffer.Enqueue(translated)
			return s.sendCHProgress(conn)
		}

		result, err := s.pool.Exec(ctx, translated)
		if err != nil {
			return s.sendCHError(conn, err.Error())
		}

		affected, _ := result.RowsAffected()
		return s.sendCHComplete(conn, affected)
	}

	rows, err := s.pool.Query(ctx, translated)
	if err != nil {
		return s.sendCHError(conn, err.Error())
	}
	defer rows.Close()

	return s.sendCHData(conn, rows)
}

func (s *NativeServer) handleSystemQueryNative(query string) *nativeResponse {
	upper := strings.ToUpper(strings.TrimSpace(query))

	if strings.Contains(upper, "VERSION()") {
		return &nativeResponse{
			columns: []string{"version()"},
			rows:    [][]interface{}{{"24.8.5.115"}},
		}
	}

	if strings.Contains(upper, "CURRENTUSER()") || strings.Contains(upper, "CURRENT_USER()") {
		return &nativeResponse{
			columns: []string{"currentUser()"},
			rows:    [][]interface{}{{"default"}},
		}
	}

	if upper == "SHOW TABLES" {
		tables := getTableList(s.pool)
		rows := [][]interface{}{}
		for _, t := range strings.Split(strings.TrimSpace(tables), "\n") {
			if t != "" {
				rows = append(rows, []interface{}{t})
			}
		}
		return &nativeResponse{
			columns: []string{"name"},
			rows:    rows,
		}
	}

	return nil
}

type nativeResponse struct {
	columns []string
	rows    [][]interface{}
}

func (s *NativeServer) sendCHResponse(conn net.Conn, resp *nativeResponse) error {
	// Send columns (Packet type 4 - Data)
	conn.Write([]byte{0x04}) // Data packet

	// Block structure: read, write, skip info, columns, rows
	conn.Write([]byte{0x00}) // read
	conn.Write([]byte{0x00}) // write
	conn.Write([]byte{0x00}) // skip info

	// Columns
	conn.Write([]byte{byte(len(resp.columns))})
	for _, col := range resp.columns {
		conn.Write([]byte(col))
		conn.Write([]byte{0x00}) // type
		conn.Write([]byte{0x00}) // collation
	}

	// Rows
	conn.Write([]byte{byte(len(resp.rows))})
	for _, row := range resp.rows {
		for _, val := range row {
			str := fmt.Sprintf("%v", val)
			conn.Write([]byte(str))
		}
	}

	// Send progress (type 5)
	conn.Write([]byte{0x05, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

	// Send end of stream (type 2)
	conn.Write([]byte{0x02})

	return nil
}

func (s *NativeServer) sendCHProgress(conn net.Conn) error {
	// Progress packet
	conn.Write([]byte{0x05, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	// End of stream
	conn.Write([]byte{0x02})
	return nil
}

func (s *NativeServer) sendCHComplete(conn net.Conn, affected int64) error {
	// Progress
	conn.Write([]byte{0x05, 0x00, 0x00, 0x00, 0x00, 0x00, byte(affected), 0x00, 0x00})
	// Profile info
	conn.Write([]byte{0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	// End of stream
	conn.Write([]byte{0x02})
	return nil
}

func (s *NativeServer) sendCHError(conn net.Conn, msg string) error {
	// Exception packet (type 0x00)
	conn.Write([]byte{0x00})
	conn.Write([]byte{byte(len(msg))})
	conn.Write([]byte(msg))
	conn.Write([]byte{0x00}) // display name
	conn.Write([]byte{0x00}) // stack trace
	conn.Write([]byte{0x02}) // no cause
	return nil
}

func (s *NativeServer) sendCHData(conn net.Conn, rows *sql.Rows) error {
	// Send progress
	conn.Write([]byte{0x05, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

	// Get columns
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return s.sendCHError(conn, err.Error())
	}

	colNames, err := rows.Columns()
	if err != nil {
		return s.sendCHError(conn, err.Error())
	}

	// Send block with column info
	conn.Write([]byte{0x04}) // Data
	conn.Write([]byte{0x00}) // read
	conn.Write([]byte{0x00}) // write
	conn.Write([]byte{0x00}) // skip info
	conn.Write([]byte{byte(len(colTypes))})

	for i, ct := range colTypes {
		name := colNames[i]
		conn.Write([]byte(name))
		// Simple type encoding
		conn.Write([]byte("String"))
		conn.Write([]byte{0x00})
		_ = ct
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

		// Write row data
		for _, v := range values {
			str := ""
			if v != nil {
				str = fmt.Sprintf("%v", v)
			}
			conn.Write([]byte{byte(len(str))})
			conn.Write([]byte(str))
		}
		rowCount++
	}

	// End of stream
	conn.Write([]byte{0x02})

	return nil
}
