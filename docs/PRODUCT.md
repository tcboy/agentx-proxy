# AgentX Proxy

将 Langfuse 依赖的 PostgreSQL、ClickHouse 协议透明代理到 MySQL，使 Langfuse 可仅依赖 MySQL 运行。

## 背景

Langfuse 是一个开源的 LLM 观测与追踪平台，默认依赖以下基础设施：

| 组件 | 用途 |
|------|------|
| **PostgreSQL** (via Prisma) | 主 OLTP 数据库：用户、组织、项目、Prompt、Dataset、评分、配置等 50+ 张表 |
| **ClickHouse** | OLAP 分析引擎：Traces、Observations、Scores 的海量写入与聚合查询 |
| **S3/MinIO** | 对象存储：事件上传、媒体文件、批量导出 |

团队目前只维护 MySQL。本项目的目标是编写一个 Go 语言 Proxy，拦截 Langfuse 发往 PG/CH 的协议请求，将其翻译并路由到 MySQL，使 Langfuse 无需修改代码即可运行。

## 架构概览

```
┌─────────────┐
│  Langfuse   │
│  (Node.js)  │
└──────┬──────┘
       │
       │ PostgreSQL 协议 (port 5432)
       │ ClickHouse 协议 (port 9000 native / port 8123 HTTP)
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

Proxy 作为中间层，对 Langfuse 表现为标准的 PG/CH 服务端，对后端将请求转译为 MySQL 兼容形式。Redis 由独立实例部署，不经过本代理。

---

## 模块一：PostgreSQL → MySQL 代理

### 1.1 协议层

- 实现 PostgreSQL **有线协议 (Wire Protocol)** 服务端，监听默认 5432 端口
- 接收 Langfuse 的 startup message、query、parse/bind/execute、describe、sync、flush 等消息
- 维持连接状态：prepared statements、portal、transaction 状态机
- 将查询通过 MySQL 驱动发往后端 MySQL，再将 MySQL 结果集映射回 PostgreSQL 的 RowDescription / DataRow / CommandComplete 格式

### 1.2 SQL 语法翻译

以下是 Langfuse 实际使用的 PG 特性，需要逐条翻译为 MySQL 等价语法：

| PG 特性 | 出现位置 | MySQL 映射策略 | 优先级 |
|---------|---------|---------------|--------|
| `ILIKE` | 全文搜索、模糊查询 | 转为 `LIKE` + `COLLATE utf8mb4_general_ci` | P0 |
| `::"EnumType"` 类型强转 | comments.ts 等多处 raw SQL | 去除类型转换，依赖 MySQL 隐式转换或字符串比较 | P0 |
| `ON CONFLICT (col) DO NOTHING / DO UPDATE` | trace_sessions 等 upsert | 转为 `INSERT ... ON DUPLICATE KEY UPDATE` 或 `INSERT IGNORE` | P0 |
| `RETURNING` 子句 | Prisma 生成的 INSERT/UPDATE/DELETE | 转为 `INSERT` + `SELECT LAST_INSERT_ID()` 或重新 SELECT 该行 | P0 |
| `LIMIT 1 BY col` (ClickHouse 风格，也出现在 PG 查询) | 多处 | 使用 MySQL 窗口函数 `ROW_NUMBER() OVER(PARTITION BY col)` | P1 |
| `LEFT JOIN LATERAL` | observations_view 等 CTE 视图 | 转为 `JOIN` + 子查询 + `ORDER BY ... LIMIT 1` | P1 |
| `ARRAY[]` / `ANY()` / 数组包含操作 | traces.tags、prompts.tags | 转为 `JSON` 列 + `JSON_CONTAINS()` / `MEMBER OF()` | P0 |
| `to_tsvector() / plainto_tsquery()` 全文搜索 | comments.ts | 转为 MySQL `MATCH() AGAINST()` FULLTEXT 索引 | P2 |
| `jsonb_set()`, `jsonb_array_elements()`, `jsonb_agg()` | Dashboard 迁移和查询 | 转为 MySQL `JSON_SET()`, `JSON_TABLE()`, `JSON_ARRAYAGG()` | P1 |
| `date_trunc()` | 时间聚合查询 | 转为 MySQL `DATE_FORMAT()` 或 `TIMESTAMP_TRUNC()` (8.0+) | P0 |
| `COALESCE` + `INTERVAL` 算术 | TTL 相关查询 | 转为 MySQL `IFNULL()` + `INTERVAL` 语法 | P0 |
| `EXTRACT(EPOCH FROM ...)` | 时间戳转换 | 转为 MySQL `UNIX_TIMESTAMP()` | P0 |
| `GENERATE_SERIES()` | 时间范围填充 | 转为递归 CTE `WITH RECURSIVE` 或预生成日历表 | P2 |

### 1.3 数组列映射方案

Langfuse 的 Prisma schema 中定义了以下数组列：

```
tags String[]           -- traces, prompts
feature_flags String[]  -- projects
custom_models String[]  -- models
path String[]           -- (迁移中出现)
rangeStart Int[]        -- dashboard widgets
rangeEnd Int[]          -- dashboard widgets
actionActions String[]  -- automations
eventActions String[]   -- automations
```

**映射策略**：在 MySQL 中将 `String[]` 存储为 `JSON` 类型（如 `JSON '["tag1", "tag2"]'`），将 `Int[]` 存储为 `JSON` 类型（如 `JSON '[1, 2, 3]'`）。

查询翻译：

| PG 查询 | MySQL 翻译 |
|---------|-----------|
| `WHERE 'tag1' = ANY(tags)` | `WHERE JSON_CONTAINS(tags, '"tag1"')` |
| `WHERE tags && ARRAY['tag1']` | `WHERE JSON_OVERLAPS(tags, '["tag1"]')` (MySQL 8.0+) |
| `WHERE tags @> ARRAY['tag1','tag2']` | `WHERE JSON_CONTAINS(tags, '["tag1","tag2"]')` |

**GIN 索引替代**：MySQL 不支持 GIN 索引。对于高频查询的数组列，使用 MySQL 8.0 的 `JSON` 多值索引（Multi-Valued Indexes）或生成的虚拟列 + `BTREE` 索引作为替代。

### 1.4 Prisma 兼容

- Langfuse 使用 Prisma ORM，provider 设为 `postgresql`
- Proxy 必须在启动握手阶段向 Prisma 报告自己为 PostgreSQL 兼容服务端
- Prisma 的 `cuid()` 默认值、`@default(dbgenerated("gen_random_uuid()"))` 需要在代理层拦截并生成对应值
- Prisma preview features `views`, `relationJoins`, `metrics` 产生的查询需正常响应
- PostgreSQL 枚举类型（如 `"CommentObjectType"`）在 MySQL 中映射为 `VARCHAR` + 约束，代理层透明处理类型名

### 1.5 系统表与元数据

Prisma 和 Langfuse 会查询 PostgreSQL 系统表获取 schema 信息，Proxy 需伪造以下系统表：

| 系统表 | 用途 | 代理策略 |
|--------|------|---------|
| `pg_catalog.pg_type` | 类型元数据 | 返回静态映射表，将 MySQL 类型映射为 PG OID |
| `pg_catalog.pg_class` | 表/索引元数据 | 从 `information_schema.tables` 映射 |
| `pg_catalog.pg_attribute` | 列元数据 | 从 `information_schema.columns` 映射 |
| `pg_catalog.pg_namespace` | Schema 命名空间 | 返回 `public` 和 `pg_catalog` |
| `pg_catalog.pg_index` | 索引信息 | 从 `information_schema.statistics` 映射 |
| `pg_catalog.pg_proc` | 函数元数据 | 返回 Langfuse 引用的函数列表 |
| `information_schema.*` | 标准元数据 | 直接透传 MySQL 的 information_schema |

### 1.6 事务与连接管理

- 支持 PG 的 `BEGIN`, `COMMIT`, `ROLLBACK` 事务语义，映射到 MySQL 事务
- 支持 PG 的 extended protocol（parse → bind → execute → sync），映射到 MySQL prepared statements
- 连接池管理：复用后端 MySQL 连接，避免为每个 PG 连接创建独立的 MySQL 连接
- 支持 savepoint 语义（Prisma 在某些场景中使用）

---

## 模块二：ClickHouse → MySQL 代理

### 2.1 协议层

ClickHouse 有两种协议，均需支持：

| 协议 | 端口 | 用途 | 代理策略 |
|------|------|------|---------|
| **Native/TCP** | 9000 | 高性能数据读写，二进制协议 | 实现 CH Native 协议解析，翻译 SQL 后通过 MySQL 驱动执行 |
| **HTTP** | 8123 | SQL 查询、INSERT、系统管理 | 实现 CH HTTP 接口，解析请求体中的 SQL，转发到 MySQL |

### 2.2 ClickHouse 特性分析与翻译

#### 2.2.1 表引擎映射

| CH 表/引擎 | 用途 | MySQL 映射 | 优先级 |
|-----------|------|-----------|--------|
| `traces` (ReplacingMergeTree) | 主 Trace 存储，按 event_ts 去重 | MySQL 表 + `UNIQUE KEY` + upsert 语义 | P0 |
| `observations` (ReplacingMergeTree) | Span/Observation 存储 | 同上 | P0 |
| `scores` (ReplacingMergeTree) | 评分存储 | 同上 | P0 |
| `dataset_run_items_rmt` (ReplacingMergeTree) | Dataset run 数据 | 同上 | P0 |
| `traces_all_amt` (AggregatingMergeTree) | 全量聚合，无 TTL | MySQL 聚合表 + 定时刷新 / 实时计算 | P1 |
| `traces_7d_amt` (AggregatingMergeTree) | 7 天聚合 | MySQL 聚合表 + 定时刷新 / 实时计算 | P1 |
| `traces_30d_amt` (AggregatingMergeTree) | 30 天聚合 | MySQL 聚合表 + 定时刷新 / 实时计算 | P1 |
| `traces_null` (Null 引擎触发表) | 物化视图触发器 | MySQL `AFTER INSERT` Trigger → 更新聚合表 | P1 |
| `event_log` (MergeTree) | 事件日志 | 普通 MySQL 表 | P2 |
| `blob_storage_file_log` (ReplacingMergeTree) | S3 文件追踪 | 普通 MySQL 表 + unique key | P2 |

**ReplacingMergeTree 去重策略**：
- CH 使用 `(event_ts, is_deleted)` 作为版本控制，`FINAL` 关键字触发去重读
- MySQL 映射：使用 `id + project_id` 作为唯一键，INSERT 时用 `ON DUPLICATE KEY UPDATE` 覆盖；读时只取最新行

**AggregatingMergeTree 聚合策略**：
- CH 使用 `SimpleAggregateFunction` 和 `AggregateFunction` state 做增量聚合
- MySQL 映射方案二选一：
  - **方案 A（实时）**：将聚合查询直接转为 MySQL 的 `GROUP BY` + 聚合函数，牺牲一定的写入放大换取实时性
  - **方案 B（异步）**：使用 MySQL Trigger 或定时任务维护预聚合表，适合高吞吐场景

#### 2.2.2 SQL 语法翻译

| CH 语法 | 出现场景 | MySQL 翻译 | 优先级 |
|---------|---------|-----------|--------|
| `FROM table FINAL` | 所有 CH 读查询 | 去除 `FINAL`，依赖唯一键保证去重 | P0 |
| `LIMIT 1 BY id, project_id` | 去重读 | `ROW_NUMBER() OVER(PARTITION BY id, project_id ORDER BY event_ts DESC) = 1` | P0 |
| `Map(K,V)` 列定义 | metadata、cost_details 等 | `JSON` 类型列 | P0 |
| `metadata['key']` Map 访问 | 查询过滤 | `JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.key'))` | P0 |
| `hasAny(arr, ['x'])` | 标签/数组过滤 | `JSON_OVERLAPS(arr, '["x"]')` | P0 |
| `hasAll(arr, ['x','y'])` | 标签过滤 | 自定义 JSON 函数组合 | P1 |
| `arrayJoin()` | 数组展开 | `JSON_TABLE()` | P1 |
| `arraySum()`, `arrayFilter()` | 数组聚合 | 自定义 JSON 函数或应用层处理 | P2 |
| `mapKeys()`, `mapValues()` | Map 操作 | `JSON_KEYS()` | P1 |
| `indexOf()` | 数组索引 | 自定义 JSON 函数 | P2 |
| `toDate()`, `toStartOfHour()` | 时间截断 | `DATE()`, `DATE_FORMAT(ts, '%Y-%m-%d %H:00:00')` | P0 |
| `date_diff()` | 时间差计算 | `TIMESTAMPDIFF()` | P0 |
| `INTERVAL 7 DAY` TTL | 数据生命周期 | `WHERE ts > NOW() - INTERVAL 7 DAY`（查询层过滤） | P1 |
| `Tuple(...)` 类型 | 结构化数据列 | `JSON` 类型 | P1 |
| `LowCardinality(String)` | 低基数列 | 普通 `VARCHAR` + 索引 | P2 |
| `Decimal64(12)` | 精确小数 | MySQL `DECIMAL(64,12)` | P0 |
| 参数化查询 `{name: Type}` | CH client 传参 | 转为 MySQL prepared statement `?` 占位符 | P0 |

#### 2.2.3 聚合函数翻译

ClickHouse 使用的聚合函数状态及其 MySQL 映射：

| CH 函数 | 说明 | MySQL 映射 |
|---------|------|-----------|
| `SimpleAggregateFunction(min, ...)` | 最小值聚合 | `MIN()` |
| `SimpleAggregateFunction(max, ...)` | 最大值聚合 | `MAX()` |
| `SimpleAggregateFunction(anyLast, ...)` | 取最后一条 | 按 event_ts 排序后 `MAX()` 或窗口函数 |
| `SimpleAggregateFunction(sumMap, ...)` | Map 累加 | `JSON_MERGE_PATCH` + 数值累加（复杂，需自定义 UDF 或应用层处理） |
| `SimpleAggregateFunction(maxMap, ...)` | Map 取最大 | 同上 |
| `SimpleAggregateFunction(groupUniqArrayArray, ...)` | 去重数组聚合 | `JSON_ARRAYAGG(DISTINCT ...)` |
| `AggregateFunction(argMax, ...)` | 条件最大值 argMax 状态 | 窗口函数 `ROW_NUMBER() OVER(PARTITION BY ... ORDER BY ...)` |
| `argMaxState()` | argMax 中间状态 | 不适用，改为直接查询 |

#### 2.2.4 物化视图与 Trigger

| CH 物化视图 | 数据来源 | 用途 | MySQL 方案 |
|------------|---------|------|-----------|
| `traces_all_amt_mv` | `traces_null` (Null 引擎) | 全量 trace 聚合 | MySQL Trigger + 聚合表 |
| `traces_7d_amt_mv` | `traces_null` | 7 天 trace 聚合 | MySQL Trigger + 聚合表 + 定时清理 |
| `traces_30d_amt_mv` | `traces_null` | 30 天 trace 聚合 | MySQL Trigger + 聚合表 + 定时清理 |
| `analytics_traces` | — | 每小时 trace 分析 | MySQL View |
| `analytics_observations` | — | 每小时 observation 分析 | MySQL View |
| `analytics_scores` | — | 每小时 score 分析 | MySQL View |
| `scores_numeric` | — | 数值型 score 虚拟表 | MySQL View with WHERE 过滤 |
| `scores_categorical` | — | 分类型 score 虚拟表 | MySQL View with WHERE 过滤 |

#### 2.2.5 系统查询

Langfuse 的 ClickHouse 客户端会执行以下系统查询，Proxy 需响应：

| 系统查询 | 用途 | 代理策略 |
|---------|------|---------|
| `SELECT * FROM system.tables WHERE database = 'default'` | 表元数据检查 | 返回伪造的 CH 格式表结构，实际映射到 MySQL 表 |
| `SELECT * FROM system.columns` | 列元数据检查 | 同上 |
| `SHOW TABLES` / `SHOW DATABASES` | 列表 | 返回映射后的表名列表 |
| `SELECT version()` | 版本检查 | 返回 CH 版本号 |
| `SELECT currentUser()` | 当前用户 | 返回固定用户 |
| `EXISTS TABLE ...` | 表存在检查 | 查询 MySQL `information_schema` |

### 2.3 写入性能考量

ClickHouse 的 `async_insert` 和批量写入是 Langfuse 高吞吐的核心：

- Langfuse 通过 `@clickhouse/client` 的 `async_insert: true` 和 `wait_for_async_insert: true` 配置批量写入
- Proxy 需实现写缓冲：接收 Langfuse 的批量 INSERT，攒批后通过 MySQL `LOAD DATA INFILE` 或批量 `INSERT ... VALUES (...), (...), ...` 写入
- 目标：支持每秒万级行写入，不成为 Langfuse 的写入瓶颈

---

## 模块三：MySQL 后端 schema 管理

### 3.1 Schema 同步

Langfuse 的 Prisma migrations 和 ClickHouse migrations 包含建表和改表 SQL。Proxy 需要：

1. **拦截迁移 SQL**：识别 Prisma 的 `_prisma_migrations` 表和 ClickHouse 的 migration 查询
2. **自动翻译 DDL**：将 PG/CH 的 DDL 翻译为 MySQL DDL
3. **Schema 版本追踪**：维护代理自身的 schema 版本，避免重复翻译

### 3.2 MySQL DDL 翻译规则

| PG/CH DDL | MySQL DDL |
|-----------|-----------|
| `CREATE TABLE ... SERIAL PRIMARY KEY` | `CREATE TABLE ... BIGINT AUTO_INCREMENT PRIMARY KEY` |
| `TEXT[]` / `VARCHAR[]` | `JSON` |
| `INT[]` / `INTEGER[]` | `JSON` |
| `JSONB` | `JSON` |
| `UUID` | `VARCHAR(36)` 或 `CHAR(36)` |
| `TIMESTAMP WITH TIME ZONE` | `DATETIME(3)` |
| `DECIMAL(65,30)` | `DECIMAL(65,30)` |
| `BOOLEAN` | `TINYINT(1)` |
| `GENERATED ALWAYS AS IDENTITY` | `AUTO_INCREMENT` |
| `USING GIN` 索引 | 多值索引 `(CAST(col AS CHAR(255)) ARRAY)` 或普通 BTREE |
| `USING HASH` 索引 | `USING HASH`（MySQL 仅支持 MEMORY 引擎的 HASH 索引，转为 BTREE） |

### 3.3 自动初始化

Proxy 启动时自动在 MySQL 中创建：

1. Langfuse Prisma 对应的 50+ 张 OLTP 表（翻译后的 MySQL schema）
2. ClickHouse 对应的 traces/observations/scores 等 OLAP 表（MySQL 版本）
3. 必要的索引和约束

---

## 技术栈

| 组件 | 选型 | 理由 |
|------|------|------|
| 语言 | Go 1.23+ | 高性能网络编程、并发模型简洁、部署简单 |
| PG 协议库 | 自实现 / `github.com/jackc/pgx` 服务端侧 fork | 需要服务端侧 PG wire protocol 支持 |
| MySQL 驱动 | `github.com/go-sql-driver/mysql` | 成熟稳定、支持 prepared statements |
| CH Native 协议 | 自实现 | 开源无成熟 Go 服务端实现 |
| 连接池 | `sync.Pool` + 自管理 | 精细控制复用策略 |
| 配置 | YAML + env var | 便于容器化部署 |

---

## 配置示例

```yaml
# config.yaml
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
  conn_max_lifetime: "10m"

proxy:
  # PG → MySQL 翻译开关
  pg_to_mysql:
    enabled: true
    # 数组列映射策略
    array_column_mode: "json"  # json | delimited
    # 全文搜索策略
    fulltext_mode: "match_against"  # match_against | like

  # CH → MySQL 翻译开关
  ch_to_mysql:
    enabled: true
    # AggregatingMergeTree 策略
    agg_mode: "realtime"  # realtime | async
    # 写缓冲配置
    write_buffer_size: 10000
    write_flush_interval: "1s"

log:
  level: "info"  # debug | info | warn | error
  format: "json"
```

---

## 项目结构

```
agentx-proxy/
├── cmd/
│   └── agentx-proxy/
│       └── main.go               # 入口，启动所有 proxy 模块
├── internal/
│   ├── config/
│   │   └── config.go             # YAML 配置加载与校验
│   ├── proxy/
│   │   ├── postgresql/           # PG wire protocol 代理
│   │   │   ├── server.go         # PG 服务端：握手、消息路由
│   │   │   ├── wire.go           # PG 有线协议编解码
│   │   │   ├── translator.go     # PG SQL → MySQL SQL 翻译
│   │   │   ├── catalog.go        # pg_catalog 系统表伪造
│   │   │   └── array.go          # 数组列 ↔ JSON 映射
│   │   └── clickhouse/           # CH 协议代理
│   │       ├── native_server.go  # CH Native (TCP) 服务端
│   │       ├── http_server.go    # CH HTTP 服务端
│   │       ├── translator.go     # CH SQL → MySQL SQL 翻译
│   │       ├── engine.go         # CH 表引擎 → MySQL 表映射
│   │       ├── aggregate.go      # 聚合函数翻译
│   │       ├── buffer.go         # 写缓冲与批量刷新
│   │       └── system.go         # system.tables 等系统查询伪造
│   └── mysql/
│       ├── pool.go               # MySQL 连接池
│       ├── schema.go             # DDL 翻译与自动建表
│       └── query.go              # 通用 MySQL 查询执行
├── pkg/
│   ├── pgwire/                   # PG wire protocol 编解码（可独立使用）
│   └── chproto/                  # CH Native 协议编解码
├── migrations/
│   ├── mysql/                    # MySQL 初始化 DDL
│   │   ├── 001_oltp_tables.sql
│   │   └── 002_olap_tables.sql
│   └── translation_rules.yaml    # 静态翻译规则配置
├── testdata/
│   ├── langfuse_queries/         # 从 Langfuse 代码提取的真实查询
│   └── expected_results/         # 期望的 MySQL 翻译结果
├── docker/
│   └── docker-compose.yml        # 开发环境：MySQL only
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

---

## 开发阶段

### Phase 1：基础框架 + PG 核心代理（4 周）

- PG wire protocol 服务端实现
- MySQL 连接池
- 基础 SQL 翻译（ILIKE, 类型强转, ON CONFLICT, RETURNING, 日期函数）
- pg_catalog 系统表伪造
- 验证：Langfuse 的 Prisma ORM 可正常连接、迁移、执行 CRUD

### Phase 2：数组列 + JSONB 翻译（2 周）

- `String[]` / `Int[]` → JSON 映射
- GIN 索引 → MySQL 多值索引
- `jsonb_set()` / `jsonb_array_elements()` → MySQL JSON 函数
- 验证：Langfuse 的 tags 过滤、Dashboard 功能正常

### Phase 3：ClickHouse 代理（4 周）

- CH Native 协议服务端实现
- CH HTTP 协议服务端实现
- ReplacingMergeTree → MySQL upsert 映射
- `FINAL` / `LIMIT 1 BY` 翻译
- `Map<K,V>` → JSON 映射
- 写缓冲与批量刷新
- 验证：Langfuse 的 Trace/Observation 写入和查询正常

### Phase 4：聚合表 + 物化视图（2 周）

- AggregatingMergeTree → MySQL 聚合表
- Null 引擎 → MySQL Trigger
- 物化视图 → MySQL View
- 聚合函数翻译（sumMap, maxMap, argMax）
- 验证：Langfuse 的分析面板、统计图表正常

### Phase 5：系统集成测试 + 性能优化（2 周）

- Langfuse E2E 测试（连接 AgentX Proxy 而非 PG/CH）
- 性能基准测试（QPS、延迟、吞吐）
- 连接池调优、写缓冲参数调优
- 可观测性：Prometheus 指标、日志、追踪

---

## 风险与限制

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| MySQL JSON 函数性能不如 PG JSONB | 数组/JSON 查询延迟升高 | 添加虚拟列 + 索引；高频查询路径走专用列 |
| AggregatingMergeTree 的增量聚合难以完全等价 | 分析数据可能有延迟或不精确 | 采用实时 GROUP BY 方案，牺牲写入性能换取精确性 |
| Prisma 新版本引入新的 PG 特性 | 需要持续维护翻译层 | 建立 Langfuse 版本兼容性矩阵，自动化回归测试 |
| ClickHouse 写入吞吐优势无法完全复现 | 大数据量下写入成为瓶颈 | 写缓冲 + 批量 INSERT；必要时建议用户保留 ClickHouse |
| PG 全文搜索质量差异 | 搜索结果不准确 | MySQL FULLTEXT 索引 + ngram 分词器（支持中文） |

---

## 验收标准

1. Langfuse 可通过 AgentX Proxy 连接 MySQL 完成启动
2. Langfuse 的 Prisma migrations 可成功执行，所有 50+ 表正确创建
3. Traces/Observations/Scores 的写入和查询功能正常
4. Tags 过滤、全文搜索、模糊查询返回正确结果
5. 分析面板（基于 ClickHouse 聚合）数据准确
6. 在 1000 QPS 写入下，p99 延迟 < 200ms
7. Langfuse 的 E2E 测试套件通过率 > 95%
