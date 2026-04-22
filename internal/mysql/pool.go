package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/agentx-labs/agentx-proxy/internal/config"
	_ "github.com/go-sql-driver/mysql"
)

type Pool struct {
	db *sql.DB
}

func NewPool(cfg *config.MySQLConfig) (*Pool, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=UTC&multiStatements=true&allowNativePasswords=true&sql_mode=ANSI_QUOTES",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	return &Pool{db: db}, nil
}

func (p *Pool) DB() *sql.DB {
	return p.db
}

func (p *Pool) Close() error {
	return p.db.Close()
}

// Exec executes a translated MySQL query
func (p *Pool) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if len(args) > 0 {
		return p.db.ExecContext(ctx, query, args...)
	}
	return p.db.ExecContext(ctx, query)
}

// Query executes a translated MySQL query and returns rows
func (p *Pool) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if len(args) > 0 {
		return p.db.QueryContext(ctx, query, args...)
	}
	return p.db.QueryContext(ctx, query)
}

// QueryRow executes a translated MySQL query and returns a single row
func (p *Pool) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if len(args) > 0 {
		return p.db.QueryRowContext(ctx, query, args...)
	}
	return p.db.QueryRowContext(ctx, query)
}

// Begin starts a new transaction
func (p *Pool) Begin(ctx context.Context) (*sql.Tx, error) {
	return p.db.BeginTx(ctx, nil)
}

// ColumnInfo holds metadata about a column
type ColumnInfo struct {
	Name         string
	DatabaseType string
	Nullable     bool
}

// GetTableColumns returns column info for a table
func (p *Pool) GetTableColumns(ctx context.Context, table string) ([]ColumnInfo, error) {
	query := `SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE
		FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = ?
		ORDER BY ORDINAL_POSITION`

	rows, err := p.Query(ctx, query, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []ColumnInfo
	for rows.Next() {
		var name, dbType, nullable string
		if err := rows.Scan(&name, &dbType, &nullable); err != nil {
			return nil, err
		}
		cols = append(cols, ColumnInfo{
			Name:         name,
			DatabaseType: dbType,
			Nullable:     strings.EqualFold(nullable, "YES"),
		})
	}
	return cols, rows.Err()
}

// TableExists checks if a table exists
func (p *Pool) TableExists(ctx context.Context, table string) (bool, error) {
	var count int
	err := p.QueryRow(ctx,
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?",
		table).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
