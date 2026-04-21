package clickhouse

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/agentx-labs/agentx-proxy/internal/mysql"
)

// WriteBuffer buffers INSERT statements and flushes them in batches
type WriteBuffer struct {
	mu           sync.Mutex
	buffer       []string
	maxSize      int
	flushInterval time.Duration
	pool         *mysql.Pool
	ticker       *time.Ticker
	stopCh       chan struct{}
}

func NewWriteBuffer(maxSize int, flushInterval time.Duration, pool *mysql.Pool) *WriteBuffer {
	wb := &WriteBuffer{
		buffer:       make([]string, 0, maxSize),
		maxSize:      maxSize,
		flushInterval: flushInterval,
		pool:         pool,
		stopCh:       make(chan struct{}),
	}

	// Start flush goroutine
	go wb.flushLoop()

	return wb
}

func (wb *WriteBuffer) Enquery(sql string) {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	wb.buffer = append(wb.buffer, sql)

	if len(wb.buffer) >= wb.maxSize {
		wb.flushLocked()
	}
}

func (wb *WriteBuffer) Enqueue(sql string) {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	wb.buffer = append(wb.buffer, sql)

	if len(wb.buffer) >= wb.maxSize {
		wb.flushLocked()
	}
}

func (wb *WriteBuffer) flushLoop() {
	wb.ticker = time.NewTicker(wb.flushInterval)
	defer wb.ticker.Stop()

	for {
		select {
		case <-wb.ticker.C:
			wb.Flush()
		case <-wb.stopCh:
			return
		}
	}
}

func (wb *WriteBuffer) Flush() {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	wb.flushLocked()
}

func (wb *WriteBuffer) flushLocked() {
	if len(wb.buffer) == 0 {
		return
	}

	queries := make([]string, len(wb.buffer))
	copy(queries, wb.buffer)
	wb.buffer = wb.buffer[:0]

	// Execute queries in batch
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try to combine INSERTs into a single multi-statement
	tx, err := wb.pool.Begin(ctx)
	if err != nil {
		slog.Error("begin tx for buffer flush", "error", err)
		// Execute individually
		for _, q := range queries {
			if _, err := wb.pool.Exec(ctx, q); err != nil {
				slog.Error("buffer flush query error", "query", q, "error", err)
			}
		}
		return
	}

	for _, q := range queries {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			slog.Error("buffer flush query error", "query", q, "error", err)
			tx.Rollback()
			return
		}
	}

	if err := tx.Commit(); err != nil {
		slog.Error("buffer flush commit error", "error", err)
	}

	slog.Debug("buffer flushed", "count", len(queries))
}

func (wb *WriteBuffer) Close() {
	close(wb.stopCh)
	wb.Flush()
}

// BatchInsert combines multiple INSERT VALUES into a single statement
func BatchInserts(inserts []string) string {
	if len(inserts) == 0 {
		return ""
	}

	// Parse INSERT INTO table (cols) VALUES (...) statements
	type insertInfo struct {
		prefix string // INSERT INTO table (cols) VALUES
		values []string
	}

	// Group by table
	tableMap := make(map[string]*insertInfo)
	for _, q := range inserts {
		upper := strings.ToUpper(q)
		if !strings.HasPrefix(upper, "INSERT") {
			continue
		}

		// Find VALUES keyword
		valuesIdx := strings.Index(upper, "VALUES")
		if valuesIdx == -1 {
			continue
		}

		prefix := q[:valuesIdx+6] // Include "VALUES"
		values := q[valuesIdx+6:]

		// Extract table name for grouping
		tableName := extractTableName(q)
		if _, ok := tableMap[tableName]; !ok {
			tableMap[tableName] = &insertInfo{prefix: prefix}
		}

		info := tableMap[tableName]
		// Strip the VALUES keyword and surrounding parens
		values = strings.TrimSpace(values)
		if strings.HasPrefix(strings.ToUpper(values), "VALUES") {
			values = strings.TrimSpace(values[6:])
		}
		info.values = append(info.values, values)
	}

	// Build combined statements
	var result strings.Builder
	first := true
	for _, info := range tableMap {
		if len(info.values) == 0 {
			continue
		}

		if !first {
			result.WriteString("; ")
		}
		first = false

		result.WriteString(info.prefix)
		result.WriteString(" ")
		result.WriteString(strings.Join(info.values, ", "))
	}

	return result.String()
}

func extractTableName(query string) string {
	upper := strings.ToUpper(query)
	intoIdx := strings.Index(upper, "INTO")
	if intoIdx == -1 {
		return ""
	}
	rest := query[intoIdx+4:]
	rest = strings.TrimSpace(rest)
	// Remove ( if present
	if idx := strings.IndexByte(rest, '('); idx != -1 {
		return strings.TrimSpace(rest[:idx])
	}
	// Remove VALUES if present
	if idx := strings.Index(strings.ToUpper(rest), "VALUES"); idx != -1 {
		return strings.TrimSpace(rest[:idx])
	}
	return strings.TrimSpace(rest)
}
