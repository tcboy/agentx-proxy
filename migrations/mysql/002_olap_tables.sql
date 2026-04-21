-- 002: OLAP Tables
-- Generated from ClickHouse migrations
-- These tables are automatically created by agentx-proxy on startup.
-- This file is for reference only.

CREATE TABLE IF NOT EXISTS traces (
	id VARCHAR(36) NOT NULL,
	timestamp DATETIME(3) NOT NULL,
	name VARCHAR(255),
	user_id VARCHAR(36),
	metadata JSON,
	release VARCHAR(255),
	version VARCHAR(255),
	project_id VARCHAR(36) NOT NULL,
	public TINYINT(1) NOT NULL DEFAULT 0,
	bookmarked TINYINT(1) NOT NULL DEFAULT 0,
	tags JSON,
	input LONGTEXT,
	output LONGTEXT,
	session_id VARCHAR(36),
	environment VARCHAR(255) NOT NULL DEFAULT 'default',
	created_at DATETIME(3) NOT NULL,
	updated_at DATETIME(3) NOT NULL,
	event_ts DATETIME(3) NOT NULL,
	is_deleted TINYINT(1) NOT NULL DEFAULT 0,
	PRIMARY KEY (id, project_id),
	INDEX idx_project_id (project_id),
	INDEX idx_timestamp (timestamp DESC),
	INDEX idx_project_timestamp (project_id, timestamp DESC),
	INDEX idx_session_id (project_id, session_id),
	INDEX idx_user_id (project_id, user_id),
	INDEX idx_environment (project_id, environment),
	INDEX idx_tags ((CAST(JSON_EXTRACT(tags, '$[*]') AS CHAR(255) ARRAY)))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS observations (
	id VARCHAR(36) NOT NULL,
	trace_id VARCHAR(36),
	project_id VARCHAR(36) NOT NULL,
	type VARCHAR(50) NOT NULL,
	parent_observation_id VARCHAR(36),
	start_time DATETIME(3) NOT NULL,
	end_time DATETIME(3),
	name VARCHAR(255),
	metadata JSON,
	level VARCHAR(50) NOT NULL DEFAULT 'DEFAULT',
	status_message TEXT,
	version VARCHAR(255),
	input LONGTEXT,
	output LONGTEXT,
	provided_model_name VARCHAR(255),
	internal_model_id VARCHAR(36),
	model_parameters JSON,
	provided_usage_details JSON,
	usage_details JSON,
	provided_cost_details JSON,
	cost_details JSON,
	total_cost DECIMAL(65,30),
	completion_start_time DATETIME(3),
	prompt_id VARCHAR(36),
	prompt_name VARCHAR(255),
	prompt_version INT,
	environment VARCHAR(255) NOT NULL DEFAULT 'default',
	usage_pricing_tier_id VARCHAR(36),
	usage_pricing_tier_name VARCHAR(255),
	tool_definitions JSON,
	tool_calls JSON,
	tool_call_names JSON,
	event_ts DATETIME(3) NOT NULL,
	is_deleted TINYINT(1) NOT NULL DEFAULT 0,
	created_at DATETIME(3) NOT NULL,
	updated_at DATETIME(3) NOT NULL,
	PRIMARY KEY (id, project_id),
	INDEX idx_project_id (project_id),
	INDEX idx_trace_id (project_id, trace_id),
	INDEX idx_project_trace (project_id, trace_id),
	INDEX idx_project_type (project_id, type),
	INDEX idx_project_timestamp (project_id, start_time DESC),
	INDEX idx_prompt_name (project_id, prompt_name, prompt_version),
	INDEX idx_environment (project_id, environment)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS ch_scores (
	id VARCHAR(36) NOT NULL,
	timestamp DATETIME(3) NOT NULL,
	project_id VARCHAR(36) NOT NULL,
	trace_id VARCHAR(36),
	observation_id VARCHAR(36),
	name VARCHAR(255) NOT NULL,
	value DECIMAL(65,30),
	source VARCHAR(50) NOT NULL,
	comment TEXT,
	author_user_id VARCHAR(36),
	config_id VARCHAR(36),
	data_type VARCHAR(50) NOT NULL,
	string_value TEXT,
	long_string_value LONGTEXT,
	queue_id VARCHAR(36),
	session_id VARCHAR(36),
	dataset_run_id VARCHAR(36),
	execution_trace_id VARCHAR(36),
	metadata JSON,
	environment VARCHAR(255) NOT NULL DEFAULT 'default',
	event_ts DATETIME(3) NOT NULL,
	is_deleted TINYINT(1) NOT NULL DEFAULT 0,
	created_at DATETIME(3) NOT NULL,
	updated_at DATETIME(3) NOT NULL,
	PRIMARY KEY (id, project_id),
	INDEX idx_project_id (project_id),
	INDEX idx_timestamp (timestamp DESC),
	INDEX idx_project_timestamp (project_id, timestamp DESC),
	INDEX idx_project_trace (project_id, trace_id),
	INDEX idx_project_observation (project_id, observation_id),
	INDEX idx_project_name (project_id, name),
	INDEX idx_project_session (project_id, session_id),
	INDEX idx_project_dataset_run (project_id, dataset_run_id),
	INDEX idx_project_config (project_id, config_id),
	INDEX idx_project_trace_obs (project_id, trace_id, observation_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS event_log (
	id VARCHAR(36) NOT NULL,
	timestamp DATETIME(3) NOT NULL,
	event_type VARCHAR(100) NOT NULL,
	project_id VARCHAR(36) NOT NULL,
	payload JSON,
	event_ts DATETIME(3) NOT NULL,
	created_at DATETIME(3) NOT NULL,
	PRIMARY KEY (id, project_id),
	INDEX idx_project_timestamp (project_id, timestamp DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS blob_storage_file_log (
	id VARCHAR(36) NOT NULL,
	project_id VARCHAR(36) NOT NULL,
	file_path VARCHAR(1024) NOT NULL,
	file_size BIGINT,
	content_type VARCHAR(255),
	created_at DATETIME(3) NOT NULL,
	event_ts DATETIME(3) NOT NULL,
	is_deleted TINYINT(1) NOT NULL DEFAULT 0,
	PRIMARY KEY (id, project_id),
	INDEX idx_project_path (project_id, file_path(255)),
	INDEX idx_project_timestamp (project_id, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS ch_dataset_run_items (
	id VARCHAR(36) NOT NULL,
	project_id VARCHAR(36) NOT NULL,
	dataset_run_id VARCHAR(36) NOT NULL,
	dataset_run_name VARCHAR(255) NOT NULL,
	dataset_item_id VARCHAR(36) NOT NULL,
	trace_id VARCHAR(36),
	dataset_run_metadata JSON,
	dataset_item_metadata JSON,
	created_at DATETIME(3) NOT NULL,
	event_ts DATETIME(3) NOT NULL,
	is_deleted TINYINT(1) NOT NULL DEFAULT 0,
	PRIMARY KEY (id, project_id),
	INDEX idx_project_run (project_id, dataset_run_id),
	INDEX idx_project_item (project_id, dataset_item_id),
	INDEX idx_project_trace (project_id, trace_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS traces_all_amt (
	project_id VARCHAR(36) NOT NULL,
	start_time DATETIME(3) NOT NULL,
	end_time DATETIME(3),
	min_start_time DATETIME(3),
	max_end_time DATETIME(3),
	trace_count BIGINT NOT NULL,
	unique_users JSON,
	unique_sessions JSON,
	unique_names JSON,
	sum_input_cost DECIMAL(65,30),
	sum_total_cost DECIMAL(65,30),
	sum_output_cost DECIMAL(65,30),
	input_units BIGINT,
	output_units BIGINT,
	total_units BIGINT,
	levels JSON,
	bookmarks BIGINT,
	environments JSON,
	PRIMARY KEY (project_id, start_time),
	INDEX idx_project_start (project_id, start_time DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS traces_7d_amt (
	project_id VARCHAR(36) NOT NULL,
	start_time DATETIME(3) NOT NULL,
	end_time DATETIME(3),
	min_start_time DATETIME(3),
	max_end_time DATETIME(3),
	trace_count BIGINT NOT NULL,
	unique_users JSON,
	unique_sessions JSON,
	unique_names JSON,
	sum_input_cost DECIMAL(65,30),
	sum_total_cost DECIMAL(65,30),
	sum_output_cost DECIMAL(65,30),
	input_units BIGINT,
	output_units BIGINT,
	total_units BIGINT,
	levels JSON,
	bookmarks BIGINT,
	environments JSON,
	PRIMARY KEY (project_id, start_time),
	INDEX idx_project_start (project_id, start_time DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS traces_30d_amt (
	project_id VARCHAR(36) NOT NULL,
	start_time DATETIME(3) NOT NULL,
	end_time DATETIME(3),
	min_start_time DATETIME(3),
	max_end_time DATETIME(3),
	trace_count BIGINT NOT NULL,
	unique_users JSON,
	unique_sessions JSON,
	unique_names JSON,
	sum_input_cost DECIMAL(65,30),
	sum_total_cost DECIMAL(65,30),
	sum_output_cost DECIMAL(65,30),
	input_units BIGINT,
	output_units BIGINT,
	total_units BIGINT,
	levels JSON,
	bookmarks BIGINT,
	environments JSON,
	PRIMARY KEY (project_id, start_time),
	INDEX idx_project_start (project_id, start_time DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE OR REPLACE VIEW analytics_traces AS
SELECT
	DATE_FORMAT(start_time, '%Y-%m-%d %H:00:00') AS hour,
	project_id,
	COUNT(*) AS trace_count,
	COUNT(DISTINCT user_id) AS unique_users,
	COUNT(DISTINCT session_id) AS unique_sessions,
	MAX(end_time) AS max_end_time,
	MIN(start_time) AS min_start_time
FROM traces
GROUP BY hour, project_id;

CREATE OR REPLACE VIEW analytics_observations AS
SELECT
	DATE_FORMAT(start_time, '%Y-%m-%d %H:00:00') AS hour,
	project_id,
	type,
	COUNT(*) AS observation_count
FROM observations
GROUP BY hour, project_id, type;

CREATE OR REPLACE VIEW analytics_scores AS
SELECT
	DATE_FORMAT(timestamp, '%Y-%m-%d %H:00:00') AS hour,
	project_id,
	name,
	data_type,
	COUNT(*) AS score_count
FROM ch_scores
GROUP BY hour, project_id, name, data_type;

CREATE OR REPLACE VIEW scores_numeric AS
SELECT * FROM ch_scores WHERE data_type = 'NUMERIC';

CREATE OR REPLACE VIEW scores_categorical AS
SELECT * FROM ch_scores WHERE data_type = 'CATEGORICAL';
