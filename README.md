# AgentX Proxy

A Go-based transparent proxy that intercepts Langfuse's PostgreSQL and ClickHouse protocol requests and translates them to MySQL, enabling Langfuse to run with MySQL as its sole database backend.

## Background

Langfuse is an open-source LLM observability platform that defaults to requiring:
- **PostgreSQL** (via Prisma) for OLTP: 50+ tables for users, organizations, projects, prompts, datasets, etc.
- **ClickHouse** for OLAP: Traces, Observations, Scores with massive writes and aggregate queries

AgentX Proxy sits between Langfuse and MySQL, translating PG/CH wire protocol and SQL syntax to MySQL-compatible equivalents.

## Architecture

```
┌─────────────┐
│  Langfuse   │
│  (Node.js)  │
└──────┬──────┘
       │
       │ PostgreSQL protocol (5432)
       │ ClickHouse protocol (9000 native / 8123 HTTP)
       ▼
┌───────────────────────────────────┐
│         AgentX Proxy              │
│  ┌──────────┬──────────┐          │
│  │ PG Proxy │ CH Proxy │          │
│  └────┬─────┴────┬─────┘          │
│       │          │                │
│  ┌────▼─────┬────▼─────┐          │
│  │ SQL      │ OLAP     │          │
│  │ Translator│Translator│          │
│  └────┬─────┴────┬─────┘          │
└───────┼──────────┼────────────────┘
        │          │
        ▼          ▼
   ┌────────┐  ┌────────┐
   │ MySQL  │  │ MySQL  │
   │ (OLTP) │  │ (OLAP) │
   └────────┘  └────────┘
```

## Features

### PostgreSQL -> MySQL
- Full PG wire protocol server (simple + extended protocol)
- SQL syntax translation (ILIKE, type casts, ON CONFLICT, RETURNING, date functions, JSONB, arrays)
- pg_catalog system table emulation (pg_type, pg_class, pg_attribute, etc.)
- Prisma ORM compatibility
- Transaction management (BEGIN/COMMIT/ROLLBACK/SAVEPOINT)

### ClickHouse -> MySQL
- HTTP protocol proxy (port 8123)
- Native TCP protocol proxy (port 9000)
- SQL translation (FINAL, LIMIT 1 BY, Map access, array functions, date functions, aggregates)
- Write buffer for batch inserts
- System table emulation (system.tables, system.columns, etc.)

### MySQL Schema
- Auto-initialization of 50+ OLTP tables (translated from Prisma)
- OLAP tables (traces, observations, scores)
- Aggregation tables + analytics views
- Array columns stored as JSON with multi-valued indexes

## Quick Start

### Prerequisites
- Go 1.23+
- MySQL 8.0+

### Build & Run

```bash
# Clone and build
make build

# Start MySQL (if using Docker)
make docker-up

# Run the proxy
make run
```

### Configuration

Copy `config.yaml` and edit to match your MySQL setup:

```bash
cp config.yaml config.local.yaml
# Edit config.local.yaml with your MySQL credentials
CONFIG_PATH=config.local.yaml make run
```

Or use environment variables:

```bash
MYSQL_HOST=127.0.0.1 \
MYSQL_USER=langfuse \
MYSQL_PASSWORD=secret \
MYSQL_DATABASE=langfuse \
make run
```

## Testing

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run benchmarks
make bench
```

## Project Structure

```
agentx-proxy/
├── cmd/agentx-proxy/main.go       # Entry point
├── internal/
│   ├── config/config.go           # Configuration
│   ├── proxy/
│   │   ├── postgresql/            # PG proxy
│   │   └── clickhouse/            # CH proxy
│   └── mysql/                     # MySQL backend
├── pkg/
│   ├── pgwire/                    # PG wire protocol
│   └── chproto/                   # CH native protocol
├── migrations/mysql/              # MySQL DDL
├── docker/docker-compose.yml      # Dev environment
└── config.yaml                    # Default config
```

## Status

| Component | Status |
|-----------|--------|
| PG Wire Protocol Server | Implemented |
| PG SQL Translation | Implemented |
| pg_catalog Emulation | Implemented |
| Prisma Compatibility | Implemented |
| CH HTTP Proxy | Implemented |
| CH Native Proxy | Implemented |
| CH SQL Translation | Implemented |
| Write Buffer | Implemented |
| MySQL Schema Init | Implemented |
| Array -> JSON Mapping | Implemented |
| Analytics Views | Implemented |
| Tests | Passing |
| Integration Tests | Pending |
| Performance Benchmarks | Pending |

## License

MIT
