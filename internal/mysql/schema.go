package mysql

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/agentx-labs/agentx-proxy/internal/config"
)

// EnsureSchema creates all required MySQL tables if they don't exist
func EnsureSchema(pool *Pool, cfg *config.Config) error {
	ctx := context.Background()

	slog.Info("ensuring MySQL schema")

	// Create OLTP tables (translated from Prisma schema)
	for _, ddl := range oltpDDLs {
		if err := execDDL(ctx, pool, ddl); err != nil {
			return fmt.Errorf("OLTP DDL: %w\nSQL: %s", err, ddl)
		}
	}

	// Create OLAP tables (translated from ClickHouse migrations)
	for _, ddl := range olapDDLs {
		if err := execDDL(ctx, pool, ddl); err != nil {
			return fmt.Errorf("OLAP DDL: %w\nSQL: %s", err, ddl)
		}
	}

	// Create pg_catalog emulation tables
	for _, ddl := range pgCatalogDDLs {
		if err := execDDL(ctx, pool, ddl); err != nil {
			return fmt.Errorf("pg_catalog DDL: %w\nSQL: %s", err, ddl)
		}
	}

	slog.Info("MySQL schema ensured")
	return nil
}

func execDDL(ctx context.Context, pool *Pool, ddl string) error {
	// Skip empty statements
	ddl = strings.TrimSpace(ddl)
	if ddl == "" {
		return nil
	}

	// Check if table already exists by extracting table name
	table := extractTableName(ddl)
	if table != "" {
		exists, err := pool.TableExists(ctx, table)
		if err != nil {
			slog.Debug("checking table existence", "table", table, "error", err)
			// Continue anyway, the DDL uses IF NOT EXISTS
		}
		if exists {
			slog.Debug("table already exists", "table", table)
			return nil
		}
	}

	_, err := pool.Exec(ctx, ddl)
	if err != nil {
		// Ignore "already exists" errors
		if strings.Contains(err.Error(), "already exists") {
			return nil
		}
		return err
	}
	slog.Info("created table", "table", table)
	return nil
}

func extractTableName(ddl string) string {
	ddl = strings.TrimSpace(ddl)
	if !strings.HasPrefix(strings.ToUpper(ddl), "CREATE TABLE") {
		return ""
	}
	// Extract table name: CREATE TABLE [IF NOT EXISTS] `name`
	parts := strings.Fields(ddl)
	for i, p := range parts {
		if strings.EqualFold(p, "TABLE") && i+1 < len(parts) {
			next := parts[i+1]
			if strings.EqualFold(next, "IF") {
				if i+3 < len(parts) {
					return stripQuotes(parts[i+3])
				}
			}
			return stripQuotes(next)
		}
	}
	return ""
}

func stripQuotes(s string) string {
	s = strings.Trim(s, "`")
	s = strings.Trim(s, "\"")
	s = strings.Trim(s, "'")
	return s
}
