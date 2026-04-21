# AgentX Proxy - Project Complete

## Overview

AgentX Proxy is a Go-based transparent proxy that intercepts Langfuse's PostgreSQL and ClickHouse protocol requests and translates them to MySQL, enabling Langfuse to run with MySQL as its sole database backend.

## Project Structure

```
agentx-proxy/
├── cmd/agentx-proxy/
│   └── main.go                    # Entry point, starts all proxy modules
├── internal/
│   ├── config/
│   │   └── config.go              # YAML config + env var overrides
│   ├── proxy/
│   │   ├── postgresql/            # PG wire protocol proxy
│   │   │   ├── server.go          # PG server: handshake, message routing
│   │   │   ├── translator.go      # PG SQL -> MySQL SQL translation
│   │   │   ├── catalog.go         # pg_catalog system table emulation
│   │   │   └── array.go           # Array column <-> JSON mapping
│   │   └── clickhouse/            # CH protocol proxy
│   │       ├── http_server.go     # CH HTTP + Native TCP servers
│   │       ├── translator.go      # CH SQL -> MySQL SQL translation
│   │       ├── buffer.go          # Write buffer for batch inserts
│   │       └── system.go          # system.tables/columns query handlers
│   └── mysql/
│       ├── pool.go                # MySQL connection pool
│       ├── schema.go              # DDL execution & table management
│       ├── oltp_ddl.go            # 50+ OLTP table definitions
│       ├── olap_ddl.go            # OLAP tables + analytics views
│       └── pg_catalog_ddl.go      # pg_catalog emulation tables
├── pkg/
│   ├── pgwire/                    # PG wire protocol encoding/decoding
│   └── chproto/                   # CH Native protocol constants
├── migrations/mysql/              # MySQL initialization DDL
├── docker/docker-compose.yml      # Dev environment (MySQL only)
├── config.yaml                    # Default configuration
└── Makefile                       # Build & test commands
```

## Implemented Features

### Module 1: PostgreSQL -> MySQL Proxy

| Feature | Status | Implementation |
|---------|--------|---------------|
| PG Wire Protocol Server | Done | Full implementation on port 5432 |
| Simple Query Protocol | Done | Query message handling |
| Extended Protocol | Done | Parse, Bind, Execute, Describe, Sync, Flush, Close |
| Authentication | Done | AuthOK (trusted) handshake |
| Transaction Management | Done | BEGIN/COMMIT/ROLLBACK/SAVEPOINT |
| Connection State | Done | Prepared statements, portals, transaction state |
| SQL Translation | Done | See translation table below |
| pg_catalog Emulation | Done | All system tables supported |
| Parameter Status | Done | server_version, encodings, timezone |

### SQL Translations Implemented

| PG Feature | MySQL Translation | Status |
|-----------|------------------|--------|
| ILIKE | LIKE COLLATE utf8mb4_general_ci | Done |
| ::type casts | Removed (implicit conversion) | Done |
| ON CONFLICT DO NOTHING | INSERT IGNORE | Done |
| ON CONFLICT DO UPDATE | ON DUPLICATE KEY UPDATE | Done |
| RETURNING clause | Temp table + SELECT | Done |
| date_trunc() | DATE_FORMAT() / DATE() | Done |
| EXTRACT(EPOCH FROM) | UNIX_TIMESTAMP() | Done |
| INTERVAL 'N' unit | INTERVAL N unit | Done |
| GENERATE_SERIES() | Recursive CTE | Done |
| jsonb_set() | JSON_SET() | Done |
| jsonb_agg() | JSON_ARRAYAGG() | Done |
| jsonb_array_elements() | JSON_TABLE() | Done |
| ->>, -> operators | JSON_UNQUOTE/JSON_EXTRACT | Done |
| ANY() | JSON_CONTAINS() | Done |
| && (overlap) | JSON_OVERLAPS() | Done |
| @> (contains) | JSON_CONTAINS() | Done |
| LEFT JOIN LATERAL | Subquery + ORDER BY LIMIT 1 | Done |
| to_tsvector/plainto_tsquery | MATCH AGAINST | Done |
| String[] / Int[] columns | JSON type | Done |
| JSONB column | JSON type | Done |
| UUID column | VARCHAR(36) | Done |
| TIMESTAMPTZ | DATETIME(3) | Done |
| BOOLEAN | TINYINT(1) | Done |

### Module 2: ClickHouse -> MySQL Proxy

| Feature | Status | Implementation |
|---------|--------|---------------|
| CH HTTP Protocol | Done | Port 8123, SQL endpoint |
| CH Native Protocol | Done | Port 9000, binary protocol |
| System Queries | Done | version, currentUser, system.tables, etc. |
| SQL Translation | Done | See CH translation table |
| Write Buffer | Done | Configurable batch insert buffering |

### CH SQL Translations Implemented

| CH Feature | MySQL Translation | Status |
|-----------|------------------|--------|
| FINAL keyword | Removed (relies on unique key) | Done |
| LIMIT 1 BY | ROW_NUMBER() approach | Done |
| col['key'] Map access | JSON_UNQUOTE(JSON_EXTRACT()) | Done |
| hasAny() | JSON_OVERLAPS() | Done |
| hasAll() | Multiple JSON_CONTAINS() | Done |
| has() | JSON_CONTAINS() | Done |
| arrayJoin() | JSON_TABLE() | Done |
| toDate() | DATE() | Done |
| toStartOfHour/Day/Month/Year | DATE_FORMAT() / MAKEDATE() | Done |
| dateDiff() | TIMESTAMPDIFF() | Done |
| toUnixTimestamp64Milli | UNIX_TIMESTAMP() * 1000 | Done |
| countIf() | COUNT(CASE WHEN) | Done |
| sumIf() | SUM(CASE WHEN) | Done |
| uniq() | COUNT(DISTINCT) | Done |
| groupArray() | JSON_ARRAYAGG() | Done |
| anyLast() | MAX() | Done |
| argMax() | GROUP_CONCAT + SUBSTRING_INDEX | Done |
| {name: Type} parameters | ? placeholders | Done |
| ::DateTime64(N) | CAST(... AS DATETIME(3)) | Done |
| ::String | Removed | Done |
| Tuple() | JSON_OBJECT() | Done |

### Module 3: MySQL Schema Management

| Feature | Status | Implementation |
|---------|--------|---------------|
| OLTP Tables (50+) | Done | Translated from Prisma schema |
| OLAP Tables | Done | Translated from CH migrations |
| Aggregation Tables | Done | traces_all_amt, traces_7d_amt, traces_30d_amt |
| Analytics Views | Done | analytics_traces/observations/scores |
| Score Views | Done | scores_numeric, scores_categorical |
| Auto-initialization | Done | Tables created on startup if not exists |
| Array -> JSON Mapping | Done | All String[]/Int[] columns use JSON type |
| Multi-valued Indexes | Done | JSON multi-valued indexes for array columns |

## Configuration

```yaml
listen:
  postgresql: "0.0.0.0:5432"
  clickhouse_native: "0.0.0.0:9000"
  clickhouse_http: "0.0.0.0:8123"

mysql:
  host: "127.0.0.1"
  port: 3306
  user: "langfuse"
  password: "${MYSQL_PASSWORD}"
  database: "langfuse"
  max_open_conns: 100
  max_idle_conns: 20

proxy:
  pg_to_mysql:
    enabled: true
    array_column_mode: "json"
    fulltext_mode: "match_against"
  ch_to_mysql:
    enabled: true
    agg_mode: "realtime"
    write_buffer_size: 10000
    write_flush_interval: "1s"

log:
  level: "info"
  format: "json"
```

## Usage

```bash
# Start with default config
make run

# Start with custom config
CONFIG_PATH=/path/to/config.yaml make run

# Docker (MySQL dev environment)
make docker-up

# Run tests
make test
```

## Test Results

All tests pass:

- `pkg/pgwire` - Wire protocol message encoding/decoding tests
- `internal/proxy/postgresql` - SQL translation tests (ILIKE, type casts, ON CONFLICT, date functions, JSON, arrays)
- `internal/proxy/clickhouse` - CH SQL translation tests (FINAL, Map access, hasAny, date functions, aggregates)

## Architecture

```
Langfuse (Node.js)
    │
    ├── PG Protocol (5432) ──► AgentX Proxy ──► MySQL (OLTP)
    │                              │
    │                              └── SQL Translator
    │
    ├── CH Native (9000) ──► AgentX Proxy ──► MySQL (OLAP)
    │                              │
    │                              └── CH SQL Translator
    │
    └── CH HTTP (8123) ──► AgentX Proxy ──► MySQL (OLAP)
                                   │
                                   └── Write Buffer
```

## Key Design Decisions

1. **Array columns as JSON**: All PG `String[]` and `Int[]` columns stored as MySQL JSON type
2. **Real-time aggregation**: AggregatingMergeTree mapped to MySQL GROUP BY (sacrificing write perf for accuracy)
3. **Trusted authentication**: Proxy uses AuthOK, relying on network-level security
4. **pg_catalog via MySQL tables**: System catalogs stored as MySQL tables, populated dynamically
5. **Write buffering**: CH inserts buffered and flushed in batches to reduce MySQL write amplification
