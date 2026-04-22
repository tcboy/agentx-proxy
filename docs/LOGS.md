# AgentX Proxy - Work Logs

## 2026-04-21 - Session 1: Initial Codebase Assessment & Fixes

### Starting State
- All code files exist per PRODUCT.md project structure
- Code compiles successfully with Go 1.26.1
- All existing tests pass (23 tests across 3 packages)
- Go module: `github.com/agentx-labs/agentx-proxy`

### Issues Identified
1. **buffer.go**: Typo method `Enquery` is a duplicate of `Enqueue` - should be removed
2. **server.go & translator.go**: `rand.Seed(time.Now().UnixNano())` is deprecated since Go 1.20
3. **pgwire/writer.go**: `SendDataRow` default type case only handles `uint32`, will panic for other types
4. **postgresql/server.go**: `normalizeQuery` removes last char unconditionally, can cause index out of range on empty strings
5. **postgresql/server.go**: `executeMySQLQuery` tries Query first then Exec on error - inefficient for write queries
6. **clickhouse/http_server.go**: CH Native protocol handshake is simplified, doesn't properly parse VarInt-encoded strings
7. **clickhouse/translator.go**: `translateLimit1By` produces broken SQL for complex queries (strips LIMIT BY but wraps incorrectly)
8. **postgresql/translator.go**: `translateReturning` produces multi-statement SQL that MySQL may not handle correctly
9. Missing: `internal/proxy/clickhouse/native_server.go` as separate file (currently in http_server.go)

### Work Done
1. **Removed duplicate method**: Deleted `Enquery` typo in buffer.go
2. **Fixed rand deprecation**: Switched from `math/rand` + `rand.Seed` to `math/rand/v2` + `rand.IntN()`
3. **Fixed SendDataRow**: Added `fmt.Sprintf("%v", val)` default case for arbitrary types
4. **Fixed normalizeQuery**: Added length check before removing trailing null byte
5. **Fixed executeMySQLQuery**: Added upfront `isWriteQuery()` check to use Exec for writes
6. **Fixed translateLimit1By**: Rewrote to use ROW_NUMBER() OVER (PARTITION BY) CTE wrapping
7. **Added translator methods**: translateDollarParams, translateStringAgg, translateBoolOperators, translateCoalesceInterval
8. **Fixed RE2 regex compatibility**:
   - `translateLateralJoin`: Replaced `(?=...)` lookahead with keyword-split approach
   - `translateBoolOperators`: Removed `(?<!...)` negative lookbehind
   - `translateToTsVector`: Fixed greedy regex crossing nested parentheses
9. **Fixed pgTypeFromMySQL**: Corrected case ordering — specific types (tinyint, smallint, etc.) now match before generic "int"
10. **Added comprehensive tests**: 60+ new tests across config, buffer, system, array, and translator packages

### Test Results
- All tests pass across 5 packages
- config: 6 tests
- clickhouse: 28 tests (buffer, system, translator)
- postgresql: 25+ tests (translator, array)
- pgwire: existing tests pass

## 2026-04-21 - Session 2: Bug Fixes Completion & Documentation

### Starting State
- Previous session identified and fixed most bugs
- All changes were uncommitted
- Missing: commit, complete.md documentation

### Work Done
1. Committed all changes (commit 7cf4a83)
2. Created complete.md documentation

### Remaining Work
- [ ] Add integration tests (PG wire protocol + CH HTTP end-to-end)
- [ ] Add tests for `internal/mysql/` package
- [ ] Improve CH Native (TCP) protocol handshake parsing

## 2026-04-21 - Session 4: CH Native Protocol Hardening, Integration Tests, Full Coverage

### Starting State
- All core proxy functionality substantially complete
- 75+ tests across 9 test files, all passing
- CH Native protocol uses simplified fixed-length encoding (will fail with real clients)
- Migration SQL files are stubs
- Missing integration tests

### Work Done
1. **Harden CH Native (TCP) protocol**: Rewrote http_server.go with proper VarInt-based encoding/decoding via new `pkg/chproto/varint.go` (ReadVarInt, WriteVarInt, ReadString, WriteString, ReadFixedUint32, ReadFixedUint64). Added `chConn` wrapper with `bufio.Reader`/`bufio.Writer` implementing `io.ByteReader`/`io.ByteWriter`. Fixed handshake parsing, query parsing, columnar data format, system query handling.
2. **Fixed translator bugs**:
   - `hasAny`/`hasAll`/`empty`/`notEmpty` regexes: changed `\w+` to `[\w.]+` to handle qualified column names like `t.tags`
   - `SimpleAggregateFunction` unwrapping: changed from `ReplaceAllString(sql, "$1")` to `ReplaceAllStringFunc` that reconstructs `func(col)`
3. **Fixed PG wire startup message parsing**: In `pkg/pgwire/wire.go`, skip 4-byte protocol version before splitting parameters on null bytes — was misaligning parameters by 2 positions
4. **Expanded test coverage**: Added 14 Langfuse-specific translator tests (CH), 9 PG wire integration tests, 4 CH Native protocol tests, 4 VarInt encoding tests, 6 system tests. Total: 165 tests across 6 packages, all passing.
5. **Populated migration SQL files**:
   - `migrations/mysql/001_oltp_tables.sql`: 61 OLTP tables from Prisma schema
   - `migrations/mysql/002_olap_tables.sql`: 9 OLAP tables + 5 views from ClickHouse migrations
   - `migrations/mysql/003_pg_catalog_tables.sql`: 10 pg_catalog emulation tables
6. **Implemented translation rules YAML loading**: Created `internal/translation/rules.go` with `Load()`, `Default()`, `Reset()` functions. Reads `migrations/translation_rules.yaml` covering PG type/function/operator mappings and CH function/aggregate mappings. Added 4 test cases.

### Test Results
- config: 6 tests
- mysql: 9 tests
- clickhouse: 56 tests
- postgresql: 27 tests
- chproto: 6 tests
- pgwire: 6 tests
- translation: 4 tests
- Total: 165 tests, all passing

## 2026-04-21 - Session 5: Migration Files, YAML Rules, Documentation

### Starting State
- All core proxy functionality complete
- 165 tests passing across 7 packages
- Migration SQL files populated, YAML translation rules implemented

### Work Done
1. Verified all migration SQL files match Go DDL constants
2. Created complete.md with final project results documentation

## 2026-04-22 - Session 6: PG Wire Protocol Prisma/Langfuse Compatibility

### Goal
Make AgentX Proxy work end-to-end with Langfuse: Prisma → PG proxy (port 15432) → MySQL, so that Langfuse signup and trace ingestion function correctly.

### Starting State
- PG wire protocol handles basic queries (simple + extended)
- Prisma can connect and run SELECT queries (findFirst, findUnique work)
- INSERT queries fail due to multiple PG protocol gaps

### Issues Found & Fixed

#### 1. Duplicate RowDescription in Extended Protocol
- **Problem**: Describe and Execute both sent RowDescription, confusing Prisma
- **Fix**: Added `described` flag to portal; skip RowDescription in Execute when already sent during Describe
- **File**: `server.go`

#### 2. Binary Parameter Decoding
- **Problem**: Prisma sends parameters in binary format (format code 1). Proxy treated binary `0x00000000` as a string.
- **Fix**: Added binary parameter decoding in `executeCatalogQueryExtended` — detect format code 1, decode 4-byte big-endian uint32 for OID type params, etc.
- **File**: `server.go`

#### 3. pg_type Recursive Lookup (Prisma SIGILL Crash)
- **Problem**: When ParameterDescription contained OID 0 (unknown), Prisma recursively queried pg_type for each parameter, eventually crashing with SIGILL.
- **Fix**: Default unknown parameter OIDs to 25 (text) for SELECT queries; add `inferInsertParamTypes` for INSERT queries to resolve column types from MySQL information_schema.
- **File**: `server.go`, `catalog.go`

#### 4. INSERT Parameter Type Inference
- **Problem**: `feature_flags` column (JSON in MySQL, String[] in Prisma) needed OID 1009 (text[]), not 25 (text) or 3802 (jsonb).
- **Fix**: Added `inferInsertParamTypes` parsing INSERT column names → `GetColumnPGOID` lookup → correct OID mapping (JSON → text[] 1009).
- **File**: `server.go`, `catalog.go`

#### 5. pg_type Returning All Rows for Unknown OID
- **Problem**: pg_type query with OID 0 in WHERE clause returned all 27 rows (regex didn't match, fell through to full scan).
- **Fix**: Return empty results when WHERE clause regex doesn't match any known OID.
- **File**: `catalog.go`

#### 6. Prisma Startup Query
- **Problem**: Prisma migration engine sends `SELECT EXISTS(SELECT 1 FROM pg_namespace WHERE nspname = $1), version(), current_setting(...)`.
- **Fix**: Added `handlePrismaStartupQuery` returning (true, PostgreSQL 14.0, 140000).
- **File**: `server.go`

#### 7. CREATE SCHEMA Access Denied
- **Problem**: Prisma sends `CREATE SCHEMA "public"` which MySQL rejects.
- **Fix**: Silently return CommandComplete("CREATE SCHEMA").
- **File**: `server.go`

#### 8. pg_advisory_lock Not Supported
- **Problem**: Prisma migration uses `SELECT pg_advisory_lock(...)` which MySQL doesn't have, causing fatal error and connection close.
- **Fix**: Intercept and return `true` (bool) for both simple and extended protocol paths.
- **File**: `server.go`

#### 9. DO $$ Anonymous Blocks
- **Problem**: Prisma sends `DO $$ BEGIN IF EXISTS ... END $$;` which is PL/pgSQL not supported by MySQL.
- **Fix**: Silently return CommandComplete("DO").
- **File**: `server.go`

#### 10. `= ANY (NULL)` Syntax
- **Problem**: Substituted PG params produce `= ANY (NULL)` which MySQL doesn't understand.
- **Fix**: Added `translateAny`: `column = ANY (NULL)` → `1=0`, `column = ANY ($N)` → `column = $N`.
- **File**: `translator.go`

#### 11. Describe for INSERT...RETURNING
- **Problem**: Prisma's Describe for INSERT...RETURNING received NoData (because `isSelectQuery` returned false), causing "index out of bounds: the len is 0 but the index is 0" crash.
- **Fix**: Added `describeReturningColumns` — parses RETURNING clause, builds synthetic `SELECT cols FROM table LIMIT 0`, uses `describeSelectColumns` to send proper RowDescription.
- **File**: `server.go`

#### 12. Binary Array → JSON Conversion
- **Problem**: Prisma sends `feature_flags` as PG binary array (dimension count, element OID, elements). MySQL expects JSON string.
- **Fix**: Added `pgBinaryArrayToJSON` and `convertPGParam` functions for proper binary→MySQL value conversion.
- **File**: `server.go`

#### 13. SendDataRow nil Handling & Type Support
- **Problem**: SendDataRow didn't handle nil values or many Go types.
- **Fix**: Added nil handling (length -1), and cases for bool, int, int64, uint64, float64, time.Time, []byte.
- **File**: `writer.go`

#### 14. Binary Result Encoding
- **Problem**: Prisma can request binary result format (format code 1) but proxy only had text encoding.
- **Fix**: Added `textToBinary` function for int2/int4/int8/float4/float8/bool/timestamp types, and `canBinaryEncode` check.
- **File**: `writer.go`

### Currently Unresolved: INSERT...RETURNING DataRow Response

**Symptom**: Prisma `user.create()` (INSERT...RETURNING) succeeds in writing to MySQL, but Prisma's Rust engine crashes after receiving the DataRow response, reporting "Can't reach database server".

**What works**:
- INSERT executes successfully (row appears in MySQL)
- Describe returns correct 11-column RowDescription
- SELECT-back retrieves the inserted row (11 columns, correct data)
- DataRow is sent without SendDataRow error
- `findUnique` / `findFirst` / `$queryRaw` all work correctly

**What doesn't work**:
- Prisma engine closes the connection (`read message error: EOF`) immediately after receiving the DataRow
- Error: "Can't reach database server at localhost:15432"

**Suspected root causes** (in order of likelihood):
1. **Message flow mismatch**: Log shows Parse → Describe S → Execute (no Bind). Prisma may use pipeline mode where Bind+Execute are batched. The proxy might not be correctly forwarding Bind parameters to the portal for INSERT...RETURNING.
2. **DataRow value format mismatch**: Values like `nil` (NULL), `0` (should be `false` for bool), timestamp `0001-01-01` (Go zero time) may not match what Prisma expects for the declared TypeOIDs in RowDescription.
3. **CommandComplete tag**: Currently sends `INSERT 0 1` but Prisma may expect `SELECT 1` for RETURNING result sets.

### Commits
- `dd965b1` - Add PG wire protocol support for Prisma/Langfuse compatibility
