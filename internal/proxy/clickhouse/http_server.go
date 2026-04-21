package clickhouse

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/binary"
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
	"github.com/agentx-labs/agentx-proxy/pkg/chproto"
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

		chType := mysqlToCHType(colType)
		result.WriteString(fmt.Sprintf("default\t%s\t%s\t%s\t\t\t\t\n", table, name, chType))
	}

	return result.String()
}

func (s *HTTPServer) existsTable(query string) string {
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

// --- Native Server (CH TCP Protocol with proper VarInt encoding) ---

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

// chConn wraps a net.Conn with buffered I/O implementing ByteReader/ByteWriter.
type chConn struct {
	net.Conn
	br *bufio.Reader
	bw *bufio.Writer
}

func newCHConn(c net.Conn) *chConn {
	return &chConn{
		Conn: c,
		br:   bufio.NewReaderSize(c, 64*1024),
		bw:   bufio.NewWriterSize(c, 64*1024),
	}
}

func (c *chConn) ReadByte() (byte, error) {
	return c.br.ReadByte()
}

func (c *chConn) WriteByte(b byte) error {
	return c.bw.WriteByte(b)
}

func (c *chConn) Read(p []byte) (int, error) {
	return c.br.Read(p)
}

func (c *chConn) Write(p []byte) (int, error) {
	return c.bw.Write(p)
}

func (c *chConn) Flush() error {
	return c.bw.Flush()
}

// byteReaderFrom wraps an io.Reader to io.ByteReader.
type byteReaderFrom struct {
	r io.Reader
}

func (b *byteReaderFrom) ReadByte() (byte, error) {
	var buf [1]byte
	_, err := io.ReadFull(b.r, buf[:])
	return buf[0], err
}

func (s *NativeServer) handleConn(conn net.Conn, connID int) {
	defer conn.Close()

	ch := newCHConn(conn)

	slog.Info("new CH native connection", "conn_id", connID, "remote", conn.RemoteAddr())
	defer slog.Info("CH native connection closed", "conn_id", connID)

	// --- Handshake ---
	// Client sends: version(uint32 LE), minor_version(uint32 LE), revision(uint32 LE),
	//               default_db(VarInt string), user(VarInt string), password(VarInt string)
	br := &byteReaderFrom{r: ch.br}
	clientVersion, err := chproto.ReadFixedUint32(br)
	if err != nil {
		slog.Error("read CH client version", "error", err)
		return
	}
	_, _ = chproto.ReadFixedUint32(br) // minor version
	clientRevision, _ := chproto.ReadFixedUint32(br)

	_, _ = chproto.ReadString(br) // default_db
	user, _ := chproto.ReadString(br)
	_, _ = chproto.ReadString(br) // password

	slog.Info("CH native handshake", "conn_id", connID, "client_version", clientVersion, "client_revision", clientRevision, "user", user)

	// Server responds: version(uint32 LE), display_name(VarInt string),
	//                  revision(uint32 LE), timezone(VarInt string)
	if err := chproto.WriteFixedUint32(ch.bw, chproto.ProtoVersion); err != nil {
		return
	}
	if err := chproto.WriteString(ch.bw, chproto.ProtoDisplayName); err != nil {
		return
	}
	if err := chproto.WriteFixedUint32(ch.bw, chproto.ProtoRevision); err != nil {
		return
	}
	if err := chproto.WriteString(ch.bw, chproto.ProtoTimezone); err != nil {
		return
	}
	ch.Flush()

	// --- Process queries ---
	for {
		packetType, err := ch.br.ReadByte()
		if err != nil {
			return
		}

		switch packetType {
		case chproto.PacketQuery:
			s.handleCHQuery(ch, connID)
		case chproto.PacketData:
			s.skipBlock(ch)
		case chproto.PacketCancel:
			return
		case chproto.PacketPing:
			ch.WriteByte(chproto.ServerPacketPong)
			ch.Flush()
		case chproto.PacketHello:
			// Re-handshake (some clients re-send hello)
			s.handleReHandshake(ch)
			ch.Flush()
		default:
			slog.Warn("unknown CH packet", "conn_id", connID, "type", packetType)
		}
	}
}

func (s *NativeServer) handleReHandshake(ch *chConn) {
	br := &byteReaderFrom{r: ch.br}
	_, _ = chproto.ReadFixedUint32(br)
	_, _ = chproto.ReadFixedUint32(br)
	_, _ = chproto.ReadFixedUint32(br)
	_, _ = chproto.ReadString(br)
	_, _ = chproto.ReadString(br)
	_, _ = chproto.ReadString(br)

	chproto.WriteFixedUint32(ch.bw, chproto.ProtoVersion)
	chproto.WriteString(ch.bw, chproto.ProtoDisplayName)
	chproto.WriteFixedUint32(ch.bw, chproto.ProtoRevision)
	chproto.WriteString(ch.bw, chproto.ProtoTimezone)
}

func (s *NativeServer) handleCHQuery(ch *chConn, connID int) error {
	// Read query fields (VarInt-encoded strings and integers)
	queryID, _ := chproto.ReadString(ch.br)
	_ = queryID

	stage, _ := chproto.ReadVarInt(ch.br)
	_ = stage

	compression, _ := chproto.ReadVarInt(ch.br)
	_ = compression

	// Read client info
	_, _ = chproto.ReadVarInt(ch.br) // interface version
	_, _ = chproto.ReadString(ch.br) // client name
	_, _ = chproto.ReadVarInt(ch.br) // major
	_, _ = chproto.ReadVarInt(ch.br) // minor
	_, _ = chproto.ReadVarInt(ch.br) // patch
	_, _ = chproto.ReadVarInt(ch.br) // revision
	_, _ = chproto.ReadString(ch.br) // timezone
	_, _ = chproto.ReadVarInt(ch.br) // quota key
	_, _ = chproto.ReadVarInt(ch.br) // distributed depth

	// Read initial query
	query, _ := chproto.ReadString(ch.br)

	// Read secondary query if present
	kind, _ := chproto.ReadVarInt(ch.br)
	if kind == chproto.QuerySecondary {
		secondaryQuery, _ := chproto.ReadString(ch.br)
		if secondaryQuery != "" {
			query = secondaryQuery
		}
	}

	if compression == chproto.CompressionEnabled {
		// Skip 4-byte compressed size header; for now assume no actual compression
		_, _ = ch.br.ReadByte()
		_, _ = ch.br.ReadByte()
		_, _ = ch.br.ReadByte()
		_, _ = ch.br.ReadByte()
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return s.sendEmptyBlock(ch)
	}

	slog.Debug("CH native query", "conn_id", connID, "sql", query)

	// Handle system queries
	if resp := s.handleSystemQueryNative(query); resp != nil {
		return s.sendNativeResponse(ch, resp)
	}

	// Translate and execute
	translated, err := s.translator.Translate(query)
	if err != nil {
		return s.sendNativeError(ch, err.Error())
	}

	slog.Debug("CH native translated", "original", query, "translated", translated)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if isWriteQuery(translated) {
		if strings.HasPrefix(strings.ToUpper(translated), "INSERT") {
			s.buffer.Enqueue(translated)
			return s.sendNativeProgress(ch)
		}

		result, err := s.pool.Exec(ctx, translated)
		if err != nil {
			return s.sendNativeError(ch, err.Error())
		}

		affected, _ := result.RowsAffected()
		return s.sendNativeComplete(ch, affected)
	}

	rows, err := s.pool.Query(ctx, translated)
	if err != nil {
		return s.sendNativeError(ch, err.Error())
	}
	defer rows.Close()

	return s.sendNativeData(ch, rows)
}

func (s *NativeServer) skipBlock(ch *chConn) {
	_, _ = chproto.ReadVarInt(ch.br)  // num_columns
	_, _ = chproto.ReadVarInt(ch.br) // num_rows
}

func (s *NativeServer) handleSystemQueryNative(query string) *nativeResponse {
	upper := strings.ToUpper(strings.TrimSpace(query))

	if strings.Contains(upper, "VERSION()") {
		return &nativeResponse{
			columns: []nativeColumn{{"version()", "String"}},
			rows:    [][]interface{}{{chproto.ProtoDBMSVersion}},
		}
	}

	if strings.Contains(upper, "CURRENTUSER()") || strings.Contains(upper, "CURRENT_USER()") {
		return &nativeResponse{
			columns: []nativeColumn{{"currentUser()", "String"}},
			rows:    [][]interface{}{{"default"}},
		}
	}

	if strings.Contains(upper, "DATABASE()") {
		return &nativeResponse{
			columns: []nativeColumn{{"database()", "String"}},
			rows:    [][]interface{}{{"default"}},
		}
	}

	if upper == "SHOW TABLES" || upper == "SHOW TABLES FROM default" {
		tables := getTableList(s.pool)
		rows := [][]interface{}{}
		for _, t := range strings.Split(strings.TrimSpace(tables), "\n") {
			if t != "" {
				rows = append(rows, []interface{}{t})
			}
		}
		return &nativeResponse{
			columns: []nativeColumn{{"name", "String"}},
			rows:    rows,
		}
	}

	if strings.Contains(upper, "SHOW DATABASES") {
		return &nativeResponse{
			columns: []nativeColumn{{"name", "String"}},
			rows:    [][]interface{}{{"default"}, {"information_schema"}},
		}
	}

	if strings.Contains(upper, "SYSTEM.TABLES") {
		return s.getNativeSystemTables()
	}

	if strings.Contains(upper, "SYSTEM.COLUMNS") {
		return s.getNativeSystemColumns()
	}

	if strings.HasPrefix(upper, "EXISTS") {
		return s.nativeExistsTable(query)
	}

	return nil
}

type nativeResponse struct {
	columns []nativeColumn
	rows    [][]interface{}
}

type nativeColumn struct {
	Name string
	Type string
}

func (s *NativeServer) sendEmptyBlock(ch *chConn) error {
	ch.WriteByte(chproto.ServerPacketData)
	chproto.WriteVarInt(ch.bw, 0) // num_columns
	chproto.WriteVarInt(ch.bw, 0) // num_rows
	return ch.Flush()
}

func (s *NativeServer) sendNativeResponse(ch *chConn, resp *nativeResponse) error {
	// Send progress
	ch.WriteByte(chproto.ServerPacketProgress)
	chproto.WriteVarInt(ch.bw, 0) // rows
	chproto.WriteVarInt(ch.bw, 0) // blocks
	chproto.WriteVarInt(ch.bw, 0) // bytes

	// Send data block
	ch.WriteByte(chproto.ServerPacketData)
	chproto.WriteVarInt(ch.bw, uint64(len(resp.columns))) // num_columns
	chproto.WriteVarInt(ch.bw, uint64(len(resp.rows)))    // num_rows

	// Column names
	for _, col := range resp.columns {
		chproto.WriteString(ch.bw, col.Name)
	}
	// Column types
	for _, col := range resp.columns {
		chproto.WriteString(ch.bw, col.Type)
	}
	// Serialization info (0 = default text)
	for range resp.columns {
		chproto.WriteVarInt(ch.bw, 0)
	}
	// Row data (columnar: each column's values together)
	for _, row := range resp.rows {
		for _, val := range row {
			chproto.WriteString(ch.bw, fmt.Sprintf("%v", val))
		}
	}

	// End of stream
	ch.WriteByte(chproto.ServerPacketEndOfStream)
	return ch.Flush()
}

func (s *NativeServer) sendNativeProgress(ch *chConn) error {
	ch.WriteByte(chproto.ServerPacketProgress)
	chproto.WriteVarInt(ch.bw, 0)
	chproto.WriteVarInt(ch.bw, 0)
	chproto.WriteVarInt(ch.bw, 0)
	ch.WriteByte(chproto.ServerPacketEndOfStream)
	return ch.Flush()
}

func (s *NativeServer) sendNativeComplete(ch *chConn, affected int64) error {
	ch.WriteByte(chproto.ServerPacketProgress)
	chproto.WriteVarInt(ch.bw, uint64(affected))
	chproto.WriteVarInt(ch.bw, 1)
	chproto.WriteVarInt(ch.bw, 0)
	ch.WriteByte(chproto.ServerPacketEndOfStream)
	return ch.Flush()
}

func (s *NativeServer) sendNativeError(ch *chConn, msg string) error {
	ch.WriteByte(chproto.ServerPacketException)
	chproto.WriteFixedUint32(ch.bw, 0) // code
	chproto.WriteString(ch.bw, msg)    // message
	chproto.WriteString(ch.bw, "")     // display name
	chproto.WriteString(ch.bw, "")     // stack trace
	ch.WriteByte(0)                    // no cause
	ch.WriteByte(chproto.ServerPacketEndOfStream)
	return ch.Flush()
}

func (s *NativeServer) sendNativeData(ch *chConn, rows *sql.Rows) error {
	// Progress
	ch.WriteByte(chproto.ServerPacketProgress)
	chproto.WriteVarInt(ch.bw, 0)
	chproto.WriteVarInt(ch.bw, 0)
	chproto.WriteVarInt(ch.bw, 0)

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return s.sendNativeError(ch, err.Error())
	}

	colNames, err := rows.Columns()
	if err != nil {
		return s.sendNativeError(ch, err.Error())
	}

	numCols := len(colTypes)

	// Collect all rows
	var allRows [][]interface{}
	for rows.Next() {
		values := make([]interface{}, numCols)
		valuePtrs := make([]interface{}, numCols)
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return s.sendNativeError(ch, err.Error())
		}
		allRows = append(allRows, values)
	}

	// Data block header
	ch.WriteByte(chproto.ServerPacketData)
	chproto.WriteVarInt(ch.bw, uint64(numCols))
	chproto.WriteVarInt(ch.bw, uint64(len(allRows)))

	// Column names
	for i := 0; i < numCols; i++ {
		chproto.WriteString(ch.bw, colNames[i])
	}
	// Column types
	for i := 0; i < numCols; i++ {
		chproto.WriteString(ch.bw, mysqlToCHType(colTypes[i].DatabaseTypeName()))
	}
	// Serialization info
	for i := 0; i < numCols; i++ {
		chproto.WriteVarInt(ch.bw, 0)
	}

	// Columnar data: each column's values in order
	for col := 0; col < numCols; col++ {
		for _, row := range allRows {
			str := ""
			if row[col] != nil {
				str = fmt.Sprintf("%v", row[col])
			}
			chproto.WriteString(ch.bw, str)
		}
	}

	ch.WriteByte(chproto.ServerPacketEndOfStream)
	return ch.Flush()
}

func (s *NativeServer) getNativeSystemTables() *nativeResponse {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	columns := []nativeColumn{
		{"database", "String"}, {"name", "String"}, {"engine", "String"},
		{"is_temporary", "UInt8"}, {"data_paths", "String"}, {"metadata_path", "String"},
		{"metadata_modification_time", "DateTime64(3)"}, {"metadata_version", "UInt64"},
		{"storage_policy", "String"}, {"delayed_insert_threads", "UInt64"},
		{"parts", "UInt64"}, {"active_parts", "UInt64"}, {"total_marks", "UInt64"},
		{"total_rows", "UInt64"}, {"total_bytes", "UInt64"},
	}

	var result [][]interface{}
	for rows.Next() {
		var name string
		rows.Scan(&name)
		result = append(result, []interface{}{
			"default", name, "MergeTree", uint8(0), "", "",
			"1970-01-01 00:00:00.000", uint64(0), "", uint64(0),
			uint64(0), uint64(0), uint64(0), uint64(0), uint64(0),
		})
	}

	return &nativeResponse{columns: columns, rows: result}
}

func (s *NativeServer) getNativeSystemColumns() *nativeResponse {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx, `
		SELECT table_name, column_name, column_type
		FROM information_schema.columns
		WHERE table_schema = DATABASE()
		ORDER BY table_name, ordinal_position
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	columns := []nativeColumn{
		{"database", "String"}, {"table", "String"}, {"name", "String"},
		{"type", "String"}, {"default_expression", "String"},
		{"comment", "String"}, {"codec_expression", "String"}, {"default_type", "String"},
	}

	var result [][]interface{}
	for rows.Next() {
		var table, name, colType string
		rows.Scan(&table, &name, &colType)
		chType := mysqlToCHType(colType)
		result = append(result, []interface{}{"default", table, name, chType, "", "", "", ""})
	}

	return &nativeResponse{columns: columns, rows: result}
}

func (s *NativeServer) nativeExistsTable(query string) *nativeResponse {
	parts := strings.Fields(query)
	if len(parts) >= 4 {
		tableName := strings.Trim(parts[len(parts)-1], "`")
		ctx := context.Background()
		exists, err := s.pool.TableExists(ctx, tableName)
		if err == nil && exists {
			return &nativeResponse{
				columns: []nativeColumn{{"result", "UInt8"}},
				rows:    [][]interface{}{{uint8(1)}},
			}
		}
	}
	return &nativeResponse{
		columns: []nativeColumn{{"result", "UInt8"}},
		rows:    [][]interface{}{{uint8(0)}},
	}
}

// Unused but kept for binary import compatibility
var _ = binary.LittleEndian
