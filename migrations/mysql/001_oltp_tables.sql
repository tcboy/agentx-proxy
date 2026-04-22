-- 001: OLTP Tables
-- Aligned with Langfuse Prisma schema (reference/langfuse/packages/shared/prisma/schema.prisma)
-- These tables are automatically created by agentx-proxy on startup.
-- This file is for reference only.
--
-- MySQL type mapping from Prisma:
--   String       -> VARCHAR(255) (TEXT for long content)
--   Int          -> INT
--   BigInt       -> BIGINT
--   Float        -> DOUBLE
--   Decimal      -> DECIMAL(65,30)
--   Boolean      -> TINYINT(1)
--   DateTime     -> DATETIME(3)
--   Json         -> JSON
--   String[]     -> JSON (array stored as JSON)
--   Int[]        -> JSON (array stored as JSON)
--   Enum         -> VARCHAR(50) (stored as enum label string)

-- ========================================
-- Auth / Identity Tables
-- ========================================

CREATE TABLE IF NOT EXISTS accounts (
	id VARCHAR(36) PRIMARY KEY,
	user_id VARCHAR(36) NOT NULL,
	type VARCHAR(255) NOT NULL,
	provider VARCHAR(255) NOT NULL,
	provider_account_id VARCHAR(255) NOT NULL,
	refresh_token TEXT,
	access_token TEXT,
	expires_at BIGINT,
	expires_in INT,
	ext_expires_in INT,
	token_type VARCHAR(255),
	scope TEXT,
	id_token TEXT,
	session_state VARCHAR(255),
	refresh_token_expires_in INT,
	created_at INT,
	INDEX idx_user_id (user_id),
	UNIQUE KEY uk_provider_account (provider, provider_account_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS sessions (
	id VARCHAR(36) PRIMARY KEY,
	session_token VARCHAR(255) NOT NULL UNIQUE,
	user_id VARCHAR(36) NOT NULL,
	expires DATETIME(3) NOT NULL,
	INDEX idx_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS users (
	id VARCHAR(36) PRIMARY KEY,
	name VARCHAR(255),
	email VARCHAR(255) UNIQUE,
	email_verified DATETIME(3),
	password VARCHAR(255),
	image TEXT,
	admin TINYINT(1) NOT NULL DEFAULT 0,
	v4_beta_enabled TINYINT(1) NOT NULL DEFAULT 0,
	feature_flags JSON,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS verification_tokens (
	identifier VARCHAR(255) NOT NULL,
	token VARCHAR(255) NOT NULL,
	expires DATETIME(3) NOT NULL,
	PRIMARY KEY (identifier, token),
	UNIQUE KEY uk_token (token)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ========================================
-- Organization / Project Tables
-- ========================================

CREATE TABLE IF NOT EXISTS organizations (
	id VARCHAR(36) PRIMARY KEY,
	name VARCHAR(255) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	cloud_config JSON,
	metadata JSON,
	cloud_billing_cycle_anchor DATETIME(3),
	cloud_billing_cycle_updated_at DATETIME(3),
	cloud_current_cycle_usage INT,
	cloud_free_tier_usage_threshold_state VARCHAR(255),
	ai_features_enabled TINYINT(1) NOT NULL DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS organization_memberships (
	id VARCHAR(36) PRIMARY KEY,
	org_id VARCHAR(36) NOT NULL,
	user_id VARCHAR(36) NOT NULL,
	role VARCHAR(50) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_org_user (org_id, user_id),
	INDEX idx_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS projects (
	id VARCHAR(36) PRIMARY KEY,
	org_id VARCHAR(36) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	deleted_at DATETIME(3),
	name VARCHAR(255) NOT NULL,
	retention_days INT,
	has_traces TINYINT(1) NOT NULL DEFAULT 0,
	metadata JSON,
	INDEX idx_org_id (org_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS project_memberships (
	org_membership_id VARCHAR(36) NOT NULL,
	project_id VARCHAR(36) NOT NULL,
	user_id VARCHAR(36) NOT NULL,
	role VARCHAR(50) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	PRIMARY KEY (project_id, user_id),
	INDEX idx_user_id (user_id),
	INDEX idx_project_id (project_id),
	INDEX idx_org_membership_id (org_membership_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS membership_invitations (
	id VARCHAR(36) PRIMARY KEY,
	email VARCHAR(255) NOT NULL,
	org_id VARCHAR(36) NOT NULL,
	org_role VARCHAR(50) NOT NULL,
	project_id VARCHAR(36),
	project_role VARCHAR(50),
	invited_by_user_id VARCHAR(36),
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_email_org (email, org_id),
	INDEX idx_project_id (project_id),
	INDEX idx_org_id (org_id),
	INDEX idx_email (email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ========================================
-- API Keys
-- ========================================

CREATE TABLE IF NOT EXISTS api_keys (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	note TEXT,
	public_key VARCHAR(255) NOT NULL UNIQUE,
	hashed_secret_key VARCHAR(255) NOT NULL UNIQUE,
	fast_hashed_secret_key VARCHAR(255) UNIQUE,
	display_secret_key VARCHAR(255) NOT NULL,
	last_used_at DATETIME(3),
	expires_at DATETIME(3),
	project_id VARCHAR(36),
	organization_id VARCHAR(36),
	scope VARCHAR(50) NOT NULL DEFAULT 'PROJECT',
	INDEX idx_organization_id (organization_id),
	INDEX idx_project_id (project_id),
	INDEX idx_public_key (public_key),
	INDEX idx_hashed_secret_key (hashed_secret_key),
	INDEX idx_fast_hashed_secret_key (fast_hashed_secret_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ========================================
-- LLM Configuration Tables
-- ========================================

CREATE TABLE IF NOT EXISTS llm_api_keys (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	provider VARCHAR(255) NOT NULL,
	adapter VARCHAR(255) NOT NULL,
	display_secret_key VARCHAR(255) NOT NULL,
	secret_key VARCHAR(255) NOT NULL,
	base_url VARCHAR(255),
	custom_models JSON,
	with_default_models TINYINT(1) NOT NULL DEFAULT 1,
	extra_headers TEXT,
	extra_header_keys JSON,
	config JSON,
	project_id VARCHAR(36) NOT NULL,
	UNIQUE KEY uk_project_provider (project_id, provider)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS models (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	project_id VARCHAR(36),
	model_name VARCHAR(255) NOT NULL,
	match_pattern VARCHAR(255) NOT NULL,
	start_date DATETIME(3),
	input_price DECIMAL(65,30),
	output_price DECIMAL(65,30),
	total_price DECIMAL(65,30),
	unit VARCHAR(50),
	tokenizer_id VARCHAR(255),
	tokenizer_config JSON,
	UNIQUE KEY uk_project_model_start (project_id, model_name, start_date, unit),
	INDEX idx_model_name (model_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS pricing_tiers (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	model_id VARCHAR(36) NOT NULL,
	name VARCHAR(255) NOT NULL,
	is_default TINYINT(1) NOT NULL DEFAULT 0,
	priority INT NOT NULL,
	conditions JSON NOT NULL,
	UNIQUE KEY uk_model_priority (model_id, priority),
	UNIQUE KEY uk_model_name (model_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS prices (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	model_id VARCHAR(36) NOT NULL,
	project_id VARCHAR(36),
	pricing_tier_id VARCHAR(36) NOT NULL,
	usage_type VARCHAR(50) NOT NULL,
	price DECIMAL(65,30) NOT NULL,
	UNIQUE KEY uk_model_usage_tier (model_id, usage_type, pricing_tier_id),
	INDEX idx_pricing_tier_id (pricing_tier_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS default_llm_models (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	project_id VARCHAR(36) NOT NULL,
	llm_api_key_id VARCHAR(36) NOT NULL,
	provider VARCHAR(255) NOT NULL,
	adapter VARCHAR(255) NOT NULL,
	model VARCHAR(255) NOT NULL,
	model_params JSON,
	UNIQUE KEY uk_project_id (project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ========================================
-- Trace / Observation / Score Tables (OLTP, Prisma-managed)
-- ========================================
-- Note: These are the OLTP tables used by Prisma for CRUD operations.
-- The ClickHouse path uses separate OLAP tables defined in 002_olap_tables.sql.

CREATE TABLE IF NOT EXISTS trace_sessions (
	id VARCHAR(36) NOT NULL,
	project_id VARCHAR(36) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	bookmarked TINYINT(1) NOT NULL DEFAULT 0,
	public TINYINT(1) NOT NULL DEFAULT 0,
	environment VARCHAR(255) NOT NULL DEFAULT 'default',
	PRIMARY KEY (id, project_id),
	INDEX idx_project_id (project_id),
	INDEX idx_project_created (project_id, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- traces table: created by 002_olap_tables.sql if it doesn't exist.
-- For OLTP Prisma access, the schema must include these columns:
-- id, external_id, timestamp, name, user_id, metadata, release, version,
-- project_id, public, bookmarked, tags, input, output, session_id,
-- created_at, updated_at
-- The OLAP version in 002 already creates this table with compatible columns.
-- If you need OLTP-only columns, add them via ALTER TABLE after startup.

-- observations table: created by 002_olap_tables.sql if it doesn't exist.
-- For OLTP Prisma access, the schema must include these columns:
-- id, trace_id, project_id, type, start_time, end_time, name, metadata,
-- parent_observation_id, level, status_message, version, created_at, updated_at,
-- model, internal_model, internal_model_id, model_parameters,
-- input, output, prompt_tokens, completion_tokens, total_tokens, unit,
-- input_cost, output_cost, total_cost, calculated_input_cost, calculated_output_cost,
-- calculated_total_cost, completion_start_time, prompt_id
-- The OLAP version in 002 creates this table with overlapping + additional columns.

-- scores table (Prisma: LegacyPrismaScore)
CREATE TABLE IF NOT EXISTS scores (
	id VARCHAR(36) NOT NULL,
	timestamp DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	project_id VARCHAR(36) NOT NULL,
	name VARCHAR(255) NOT NULL,
	value DOUBLE,
	source VARCHAR(50) NOT NULL,
	author_user_id VARCHAR(36),
	comment TEXT,
	trace_id VARCHAR(36) NOT NULL,
	observation_id VARCHAR(36),
	config_id VARCHAR(36),
	string_value TEXT,
	queue_id VARCHAR(36),
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	data_type VARCHAR(50) NOT NULL DEFAULT 'NUMERIC',
	PRIMARY KEY (id, project_id),
	INDEX idx_timestamp (timestamp),
	INDEX idx_value (value),
	INDEX idx_project_name (project_id, name),
	INDEX idx_author_user_id (author_user_id),
	INDEX idx_config_id (config_id),
	INDEX idx_trace_id (trace_id),
	INDEX idx_observation_id (observation_id),
	INDEX idx_source (source),
	INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS score_configs (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	project_id VARCHAR(36) NOT NULL,
	name VARCHAR(255) NOT NULL,
	data_type VARCHAR(50) NOT NULL,
	is_archived TINYINT(1) NOT NULL DEFAULT 0,
	min_value DOUBLE,
	max_value DOUBLE,
	categories JSON,
	description TEXT,
	UNIQUE KEY uk_project_id_name (project_id, name),
	INDEX idx_data_type (data_type),
	INDEX idx_is_archived (is_archived),
	INDEX idx_project_id (project_id),
	INDEX idx_categories (categories),
	INDEX idx_created_at (created_at),
	INDEX idx_updated_at (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ========================================
-- Prompt Tables
-- ========================================

CREATE TABLE IF NOT EXISTS prompts (
	id VARCHAR(36) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	project_id VARCHAR(36) NOT NULL,
	created_by VARCHAR(36) NOT NULL,
	prompt JSON NOT NULL,
	name VARCHAR(255) NOT NULL,
	version INT NOT NULL,
	type VARCHAR(255) NOT NULL DEFAULT 'text',
	is_active TINYINT(1),
	config JSON NOT NULL,
	tags JSON,
	labels JSON,
	commit_message TEXT,
	PRIMARY KEY (id, project_id),
	UNIQUE KEY uk_project_name_version (project_id, name, version),
	INDEX idx_project_id (project_id, id),
	INDEX idx_created_at (created_at),
	INDEX idx_updated_at (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS prompt_dependencies (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	project_id VARCHAR(36) NOT NULL,
	parent_id VARCHAR(36) NOT NULL,
	child_name VARCHAR(255) NOT NULL,
	child_label VARCHAR(255),
	child_version INT,
	INDEX idx_project_parent (project_id, parent_id),
	INDEX idx_project_child_name (project_id, child_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS prompt_protected_labels (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	project_id VARCHAR(36) NOT NULL,
	label VARCHAR(255) NOT NULL,
	UNIQUE KEY uk_project_label (project_id, label)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ========================================
-- Dataset Tables
-- ========================================

CREATE TABLE IF NOT EXISTS datasets (
	id VARCHAR(36) NOT NULL,
	project_id VARCHAR(36) NOT NULL,
	name VARCHAR(255) NOT NULL,
	description TEXT,
	metadata JSON,
	remote_experiment_url TEXT,
	remote_experiment_payload JSON,
	input_schema JSON,
	expected_output_schema JSON,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	PRIMARY KEY (id, project_id),
	UNIQUE KEY uk_project_name (project_id, name),
	INDEX idx_created_at (created_at),
	INDEX idx_updated_at (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS dataset_items (
	id VARCHAR(36) NOT NULL,
	project_id VARCHAR(36) NOT NULL,
	status VARCHAR(50),
	input JSON,
	expected_output JSON,
	metadata JSON,
	source_trace_id VARCHAR(36),
	source_observation_id VARCHAR(36),
	dataset_id VARCHAR(36) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	valid_from DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	valid_to DATETIME(3),
	is_deleted TINYINT(1) NOT NULL DEFAULT 0,
	PRIMARY KEY (id, project_id, valid_from),
	INDEX idx_project_valid_to (project_id, valid_to),
	INDEX idx_project_id_valid_from (project_id, id, valid_from),
	INDEX idx_source_trace_id (source_trace_id),
	INDEX idx_source_observation_id (source_observation_id),
	INDEX idx_dataset_id (dataset_id),
	INDEX idx_created_at (created_at),
	INDEX idx_updated_at (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS dataset_runs (
	id VARCHAR(36) NOT NULL,
	project_id VARCHAR(36) NOT NULL,
	name VARCHAR(255) NOT NULL,
	description TEXT,
	metadata JSON,
	dataset_id VARCHAR(36) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	PRIMARY KEY (id, project_id),
	UNIQUE KEY uk_dataset_project_name (dataset_id, project_id, name),
	INDEX idx_dataset_id (dataset_id),
	INDEX idx_created_at (created_at),
	INDEX idx_updated_at (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS dataset_run_items (
	id VARCHAR(36) NOT NULL,
	project_id VARCHAR(36) NOT NULL,
	dataset_run_id VARCHAR(36) NOT NULL,
	dataset_item_id VARCHAR(36) NOT NULL,
	trace_id VARCHAR(36),
	observation_id VARCHAR(36),
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	PRIMARY KEY (id, project_id),
	INDEX idx_dataset_run_id (dataset_run_id),
	INDEX idx_dataset_item_id (dataset_item_id),
	INDEX idx_observation_id (observation_id),
	INDEX idx_trace_id (trace_id),
	INDEX idx_created_at (created_at),
	INDEX idx_updated_at (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ========================================
-- Evaluation / Job Tables
-- ========================================

CREATE TABLE IF NOT EXISTS eval_templates (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	project_id VARCHAR(36),
	name VARCHAR(255) NOT NULL,
	version INT NOT NULL,
	prompt TEXT NOT NULL,
	partner VARCHAR(255),
	model VARCHAR(255),
	provider VARCHAR(255),
	model_params JSON,
	vars JSON,
	output_schema JSON NOT NULL,
	UNIQUE KEY uk_project_name_version (project_id, name, version),
	INDEX idx_project_id (project_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS job_configurations (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	project_id VARCHAR(36) NOT NULL,
	job_type VARCHAR(50) NOT NULL,
	status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE',
	blocked_at DATETIME(3),
	block_reason VARCHAR(255),
	block_message TEXT,
	eval_template_id VARCHAR(36),
	score_name VARCHAR(255) NOT NULL,
	filter JSON NOT NULL,
	target_object VARCHAR(255) NOT NULL,
	variable_mapping JSON NOT NULL,
	sampling DECIMAL(65,30) NOT NULL,
	delay INT NOT NULL,
	time_scope JSON,
	INDEX idx_project_id (project_id, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS job_executions (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	project_id VARCHAR(36) NOT NULL,
	job_configuration_id VARCHAR(36) NOT NULL,
	job_template_id VARCHAR(36),
	status VARCHAR(50) NOT NULL,
	start_time DATETIME(3),
	end_time DATETIME(3),
	error TEXT,
	job_input_trace_id VARCHAR(36),
	job_input_trace_timestamp DATETIME(3),
	job_input_observation_id VARCHAR(36),
	job_input_dataset_item_id VARCHAR(36),
	job_input_dataset_item_valid_from DATETIME(3),
	job_output_score_id VARCHAR(36),
	execution_trace_id VARCHAR(36),
	INDEX idx_project_job_trace (project_id, job_configuration_id, job_input_trace_id),
	INDEX idx_project_score (project_id, job_output_score_id),
	INDEX idx_job_configuration_id (job_configuration_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ========================================
-- Annotation Tables
-- ========================================

CREATE TABLE IF NOT EXISTS annotation_queues (
	id VARCHAR(36) PRIMARY KEY,
	name VARCHAR(255) NOT NULL,
	description TEXT,
	score_config_ids JSON,
	project_id VARCHAR(36) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_project_name (project_id, name),
	INDEX idx_project_id (id, project_id),
	INDEX idx_project_created (project_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS annotation_queue_items (
	id VARCHAR(36) PRIMARY KEY,
	queue_id VARCHAR(36) NOT NULL,
	object_id VARCHAR(36) NOT NULL,
	object_type VARCHAR(50) NOT NULL,
	status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
	locked_at DATETIME(3),
	locked_by_user_id VARCHAR(36),
	annotator_user_id VARCHAR(36),
	completed_at DATETIME(3),
	project_id VARCHAR(36) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	INDEX idx_project_id (id, project_id),
	INDEX idx_project_queue_status (project_id, queue_id, status),
	INDEX idx_object (object_id, object_type, project_id, queue_id),
	INDEX idx_annotator_user_id (annotator_user_id),
	INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS annotation_queue_assignments (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	user_id VARCHAR(36) NOT NULL,
	queue_id VARCHAR(36) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_project_queue_user (project_id, queue_id, user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ========================================
-- Comment Tables
-- ========================================

CREATE TABLE IF NOT EXISTS comments (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	object_type VARCHAR(50) NOT NULL,
	object_id VARCHAR(36) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	content TEXT NOT NULL,
	author_user_id VARCHAR(36),
	data_field VARCHAR(255),
	path JSON,
	range_start JSON,
	range_end JSON,
	INDEX idx_project_object (project_id, object_type, object_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS comment_reactions (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	comment_id VARCHAR(36) NOT NULL,
	user_id VARCHAR(36) NOT NULL,
	emoji VARCHAR(255) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_comment_user_emoji (comment_id, user_id, emoji)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ========================================
-- Notification Tables
-- ========================================

CREATE TABLE IF NOT EXISTS notification_preferences (
	id VARCHAR(36) PRIMARY KEY,
	user_id VARCHAR(36) NOT NULL,
	project_id VARCHAR(36) NOT NULL,
	channel VARCHAR(50) NOT NULL,
	type VARCHAR(50) NOT NULL,
	enabled TINYINT(1) NOT NULL DEFAULT 1,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_user_project_channel_type (user_id, project_id, channel, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ========================================
-- Audit / Cron Tables
-- ========================================

CREATE TABLE IF NOT EXISTS audit_logs (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	type VARCHAR(50) NOT NULL DEFAULT 'USER',
	api_key_id VARCHAR(36),
	user_id VARCHAR(36),
	org_id VARCHAR(36) NOT NULL,
	user_org_role VARCHAR(50),
	project_id VARCHAR(36),
	user_project_role VARCHAR(50),
	resource_type VARCHAR(255) NOT NULL,
	resource_id VARCHAR(255) NOT NULL,
	action VARCHAR(255) NOT NULL,
	`before` TEXT,
	`after` TEXT,
	INDEX idx_project_id (project_id),
	INDEX idx_api_key_id (api_key_id),
	INDEX idx_user_id (user_id),
	INDEX idx_org_id (org_id),
	INDEX idx_created_at (created_at),
	INDEX idx_updated_at (updated_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS cron_jobs (
	name VARCHAR(255) PRIMARY KEY,
	last_run DATETIME(3),
	job_started_at DATETIME(3),
	state TEXT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS background_migrations (
	id VARCHAR(36) PRIMARY KEY,
	name VARCHAR(255) NOT NULL UNIQUE,
	script TEXT NOT NULL,
	args JSON NOT NULL,
	state JSON NOT NULL DEFAULT '{}',
	finished_at DATETIME(3),
	failed_at DATETIME(3),
	failed_reason TEXT,
	worker_id VARCHAR(36),
	locked_at DATETIME(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ========================================
-- Batch Export / Action Tables
-- ========================================

CREATE TABLE IF NOT EXISTS batch_exports (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	project_id VARCHAR(36) NOT NULL,
	user_id VARCHAR(36) NOT NULL,
	finished_at DATETIME(3),
	expires_at DATETIME(3),
	name VARCHAR(255) NOT NULL,
	status VARCHAR(50) NOT NULL,
	query JSON NOT NULL,
	format VARCHAR(255) NOT NULL,
	url TEXT,
	log TEXT,
	INDEX idx_project_user (project_id, user_id),
	INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS batch_actions (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	project_id VARCHAR(36) NOT NULL,
	user_id VARCHAR(36) NOT NULL,
	action_type VARCHAR(255) NOT NULL,
	table_name VARCHAR(255) NOT NULL,
	status VARCHAR(50) NOT NULL,
	finished_at DATETIME(3),
	query JSON NOT NULL,
	config JSON,
	total_count INT,
	processed_count INT,
	failed_count INT,
	log TEXT,
	INDEX idx_project_user (project_id, user_id),
	INDEX idx_status (status),
	INDEX idx_project_action_type (project_id, action_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ========================================
-- Media Tables
-- ========================================

CREATE TABLE IF NOT EXISTS media (
	id VARCHAR(36) NOT NULL,
	sha_256_hash CHAR(44) NOT NULL,
	project_id VARCHAR(36) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	uploaded_at DATETIME(3),
	upload_http_status INT,
	upload_http_error TEXT,
	bucket_path VARCHAR(255) NOT NULL,
	bucket_name VARCHAR(255) NOT NULL,
	content_type VARCHAR(255) NOT NULL,
	content_length BIGINT NOT NULL,
	PRIMARY KEY (id, project_id),
	UNIQUE KEY uk_project_id (project_id, id),
	UNIQUE KEY uk_project_sha (project_id, sha_256_hash),
	INDEX idx_project_created (project_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS trace_media (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	media_id VARCHAR(36) NOT NULL,
	trace_id VARCHAR(36) NOT NULL,
	field VARCHAR(255) NOT NULL,
	UNIQUE KEY uk_project_trace_media_field (project_id, trace_id, media_id, field),
	INDEX idx_project_media (project_id, media_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS observation_media (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	media_id VARCHAR(36) NOT NULL,
	trace_id VARCHAR(36) NOT NULL,
	observation_id VARCHAR(36) NOT NULL,
	field VARCHAR(255) NOT NULL,
	UNIQUE KEY uk_project_trace_obs_media_field (project_id, trace_id, observation_id, media_id, field),
	INDEX idx_project_media (project_id, media_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ========================================
-- Dashboard Tables
-- ========================================

CREATE TABLE IF NOT EXISTS dashboards (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	created_by VARCHAR(36),
	updated_by VARCHAR(36),
	project_id VARCHAR(36),
	name VARCHAR(255) NOT NULL,
	description VARCHAR(255) NOT NULL,
	definition JSON NOT NULL,
	filters JSON NOT NULL DEFAULT '[]'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS dashboard_widgets (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	created_by VARCHAR(36),
	updated_by VARCHAR(36),
	project_id VARCHAR(36),
	name VARCHAR(255) NOT NULL,
	description VARCHAR(255) NOT NULL,
	view VARCHAR(100) NOT NULL,
	dimensions JSON NOT NULL,
	metrics JSON NOT NULL,
	filters JSON NOT NULL,
	chart_type VARCHAR(50) NOT NULL,
	chart_config JSON NOT NULL,
	min_version INT NOT NULL DEFAULT 1
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS table_view_presets (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	project_id VARCHAR(36) NOT NULL,
	name VARCHAR(255) NOT NULL,
	table_name VARCHAR(255) NOT NULL,
	created_by VARCHAR(36),
	updated_by VARCHAR(36),
	filters JSON NOT NULL,
	column_order JSON NOT NULL,
	column_visibility JSON NOT NULL,
	search_query TEXT,
	order_by JSON,
	UNIQUE KEY uk_project_table_name (project_id, table_name, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS default_views (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	project_id VARCHAR(36) NOT NULL,
	user_id VARCHAR(36),
	view_name VARCHAR(255) NOT NULL,
	view_id VARCHAR(255) NOT NULL,
	INDEX idx_project_view_name (project_id, view_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ========================================
-- Integration Tables
-- ========================================

CREATE TABLE IF NOT EXISTS sso_configs (
	domain VARCHAR(255) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	auth_provider VARCHAR(255) NOT NULL,
	auth_config JSON
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS posthog_integrations (
	project_id VARCHAR(36) PRIMARY KEY,
	encrypted_posthog_api_key VARCHAR(255) NOT NULL,
	posthog_host_name VARCHAR(255) NOT NULL,
	last_sync_at DATETIME(3),
	enabled TINYINT(1) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	export_source VARCHAR(50) NOT NULL DEFAULT 'TRACES_OBSERVATIONS'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS mixpanel_integrations (
	project_id VARCHAR(36) PRIMARY KEY,
	encrypted_mixpanel_project_token VARCHAR(255) NOT NULL,
	mixpanel_region VARCHAR(255) NOT NULL,
	last_sync_at DATETIME(3),
	enabled TINYINT(1) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	export_source VARCHAR(50) NOT NULL DEFAULT 'TRACES_OBSERVATIONS'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS blob_storage_integrations (
	project_id VARCHAR(36) PRIMARY KEY,
	type VARCHAR(50) NOT NULL,
	bucket_name VARCHAR(255) NOT NULL,
	prefix VARCHAR(255) NOT NULL,
	access_key_id VARCHAR(255),
	secret_access_key VARCHAR(255),
	region VARCHAR(255) NOT NULL,
	endpoint VARCHAR(255),
	force_path_style TINYINT(1) NOT NULL,
	next_sync_at DATETIME(3),
	last_sync_at DATETIME(3),
	enabled TINYINT(1) NOT NULL,
	export_frequency VARCHAR(50) NOT NULL,
	file_type VARCHAR(50) NOT NULL DEFAULT 'CSV',
	export_mode VARCHAR(50) NOT NULL DEFAULT 'FULL_HISTORY',
	export_start_date DATETIME(3),
	export_source VARCHAR(50) NOT NULL DEFAULT 'TRACES_OBSERVATIONS',
	compressed TINYINT(1) NOT NULL DEFAULT 1,
	last_error TEXT,
	last_error_at DATETIME(3),
	last_failure_notification_sent_at DATETIME(3),
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS slack_integrations (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL UNIQUE,
	team_id VARCHAR(255) NOT NULL,
	team_name VARCHAR(255) NOT NULL,
	bot_token VARCHAR(255) NOT NULL,
	bot_user_id VARCHAR(255) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	INDEX idx_team_id (team_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ========================================
-- Automation Tables
-- ========================================

CREATE TABLE IF NOT EXISTS actions (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	project_id VARCHAR(36) NOT NULL,
	type VARCHAR(50) NOT NULL,
	config JSON NOT NULL,
	INDEX idx_project_id (project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS triggers (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	project_id VARCHAR(36) NOT NULL,
	event_source VARCHAR(255) NOT NULL,
	event_actions JSON,
	filter JSON,
	status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE',
	INDEX idx_project_id (project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS automations (
	id VARCHAR(36) PRIMARY KEY,
	name VARCHAR(255) NOT NULL,
	trigger_id VARCHAR(36) NOT NULL,
	action_id VARCHAR(36) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	project_id VARCHAR(36) NOT NULL,
	INDEX idx_project_action_trigger (project_id, action_id, trigger_id),
	INDEX idx_project_name (project_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS automation_executions (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	source_id VARCHAR(36) NOT NULL,
	automation_id VARCHAR(36) NOT NULL,
	trigger_id VARCHAR(36) NOT NULL,
	action_id VARCHAR(36) NOT NULL,
	project_id VARCHAR(36) NOT NULL,
	status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
	input JSON NOT NULL,
	output JSON,
	started_at DATETIME(3),
	finished_at DATETIME(3),
	error TEXT,
	INDEX idx_trigger_id (trigger_id),
	INDEX idx_action_id (action_id),
	INDEX idx_project_id (project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ========================================
-- LLM Schema / Tool Tables
-- ========================================

CREATE TABLE IF NOT EXISTS llm_schemas (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	project_id VARCHAR(36) NOT NULL,
	name VARCHAR(255) NOT NULL,
	description VARCHAR(255) NOT NULL,
	schema JSON NOT NULL,
	UNIQUE KEY uk_project_name (project_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS llm_tools (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	project_id VARCHAR(36) NOT NULL,
	name VARCHAR(255) NOT NULL,
	description TEXT NOT NULL,
	parameters JSON NOT NULL,
	UNIQUE KEY uk_project_name (project_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ========================================
-- Billing / Cloud Tables
-- ========================================

CREATE TABLE IF NOT EXISTS billing_meter_backups (
	stripe_customer_id VARCHAR(255) NOT NULL,
	meter_id VARCHAR(255) NOT NULL,
	start_time DATETIME(3) NOT NULL,
	end_time DATETIME(3) NOT NULL,
	aggregated_value INT NOT NULL,
	event_name VARCHAR(255) NOT NULL,
	org_id VARCHAR(36) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_stripe_meter_start_end (stripe_customer_id, meter_id, start_time, end_time),
	INDEX idx_stripe_meter_time (stripe_customer_id, meter_id, start_time, end_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS cloud_spend_alerts (
	id VARCHAR(36) PRIMARY KEY,
	org_id VARCHAR(36) NOT NULL,
	title VARCHAR(255) NOT NULL,
	threshold DECIMAL(65,30) NOT NULL,
	triggered_at DATETIME(3),
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	INDEX idx_org_id (org_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ========================================
-- Survey / Misc Tables
-- ========================================

CREATE TABLE IF NOT EXISTS surveys (
	id VARCHAR(36) PRIMARY KEY,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	survey_name VARCHAR(50) NOT NULL,
	response JSON NOT NULL,
	user_id VARCHAR(36),
	user_email VARCHAR(255),
	org_id VARCHAR(36)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS pending_deletions (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	object VARCHAR(255) NOT NULL,
	object_id VARCHAR(36) NOT NULL,
	is_deleted TINYINT(1) NOT NULL DEFAULT 0,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	INDEX idx_project_object (project_id, object, is_deleted, object_id, id),
	INDEX idx_object (object_id, object)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ========================================
-- Prisma Migration Table
-- ========================================

CREATE TABLE IF NOT EXISTS _prisma_migrations (
	id VARCHAR(36) PRIMARY KEY,
	checksum VARCHAR(255) NOT NULL,
	finished_at DATETIME(3),
	migration_name VARCHAR(255) NOT NULL,
	logs TEXT,
	rolled_back_at DATETIME(3),
	applied_steps_count INT NOT NULL DEFAULT 0,
	applied_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
