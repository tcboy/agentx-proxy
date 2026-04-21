-- 001: OLTP Tables
-- Generated from Langfuse Prisma schema
-- These tables are automatically created by agentx-proxy on startup.
-- This file is for reference only.

CREATE TABLE IF NOT EXISTS accounts (
	id VARCHAR(36) PRIMARY KEY,
	user_id VARCHAR(36) NOT NULL,
	type VARCHAR(255) NOT NULL,
	provider VARCHAR(255) NOT NULL,
	provider_account_id VARCHAR(255) NOT NULL,
	refresh_token TEXT,
	access_token TEXT,
	expires_at BIGINT,
	token_type VARCHAR(255),
	scope TEXT,
	id_token TEXT,
	session_state VARCHAR(255),
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
	email VARCHAR(255) NOT NULL UNIQUE,
	email_verified DATETIME(3),
	image TEXT,
	password VARCHAR(255),
	admin TINYINT(1) NOT NULL DEFAULT 0,
	feature_flags JSON,
	v4_beta_enabled TINYINT(1) NOT NULL DEFAULT 0,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS verification_tokens (
	identifier VARCHAR(255) NOT NULL,
	token VARCHAR(255) NOT NULL,
	expires DATETIME(3) NOT NULL,
	PRIMARY KEY (identifier, token)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS organizations (
	id VARCHAR(36) PRIMARY KEY,
	name VARCHAR(255) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	metadata JSON,
	cloud_config JSON,
	billing_email VARCHAR(255),
	billing_address TEXT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS projects (
	id VARCHAR(36) PRIMARY KEY,
	name VARCHAR(255) NOT NULL,
	org_id VARCHAR(36) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	metadata JSON,
	retention_days INT,
	has_traces TINYINT(1) NOT NULL DEFAULT 0,
	INDEX idx_org_id (org_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS organization_memberships (
	org_id VARCHAR(36) NOT NULL,
	user_id VARCHAR(36) NOT NULL,
	role VARCHAR(50) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	PRIMARY KEY (org_id, user_id),
	INDEX idx_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS project_memberships (
	project_id VARCHAR(36) NOT NULL,
	user_id VARCHAR(36) NOT NULL,
	role VARCHAR(50) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	PRIMARY KEY (project_id, user_id),
	INDEX idx_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS api_keys (
	id VARCHAR(36) PRIMARY KEY,
	public_key VARCHAR(255) NOT NULL UNIQUE,
	hashed_secret_key VARCHAR(255) NOT NULL,
	scope VARCHAR(50) NOT NULL,
	project_id VARCHAR(36),
	org_id VARCHAR(36),
	display_name VARCHAR(255),
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	expires_at DATETIME(3),
	last_used_at DATETIME(3),
	INDEX idx_public_key (public_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS trace_sessions (
	id VARCHAR(36) NOT NULL,
	project_id VARCHAR(36) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	PRIMARY KEY (id, project_id),
	INDEX idx_project_id (project_id),
	INDEX idx_project_created (project_id, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS prompts (
	id VARCHAR(36) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	project_id VARCHAR(36) NOT NULL,
	name VARCHAR(255) NOT NULL,
	prompt JSON NOT NULL,
	config JSON,
	labels JSON NOT NULL,
	tags JSON,
	version INT NOT NULL,
	is_active TINYINT(1) NOT NULL DEFAULT 0,
	commit_version VARCHAR(36),
	type VARCHAR(50) NOT NULL DEFAULT 'text',
	PRIMARY KEY (id, project_id),
	UNIQUE KEY uk_project_name_version (project_id, name, version),
	INDEX idx_project_name_active (project_id, name, is_active)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS models (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36),
	model_name VARCHAR(255) NOT NULL,
	start_date DATETIME(3),
	unit VARCHAR(50),
	tokenizer_config JSON,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_project_model_start (project_id, model_name, start_date, unit)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS prices (
	id VARCHAR(36) PRIMARY KEY,
	model_id VARCHAR(36) NOT NULL,
	usage_type VARCHAR(50) NOT NULL,
	pricing_tier_id VARCHAR(36),
	price DECIMAL(65,30) NOT NULL,
	currency VARCHAR(10) NOT NULL DEFAULT 'USD',
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_model_usage_tier (model_id, usage_type, pricing_tier_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS pricing_tiers (
	id VARCHAR(36) PRIMARY KEY,
	model_id VARCHAR(36) NOT NULL,
	name VARCHAR(255) NOT NULL,
	priority INT NOT NULL,
	conditions JSON NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_model_priority (model_id, priority),
	UNIQUE KEY uk_model_name (model_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS datasets (
	id VARCHAR(36) NOT NULL,
	project_id VARCHAR(36) NOT NULL,
	name VARCHAR(255) NOT NULL,
	description TEXT,
	metadata JSON,
	input_schema JSON,
	expected_output_schema JSON,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	PRIMARY KEY (id, project_id),
	UNIQUE KEY uk_project_name (project_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS dataset_items (
	id VARCHAR(36) NOT NULL,
	project_id VARCHAR(36) NOT NULL,
	dataset_name VARCHAR(255) NOT NULL,
	input JSON NOT NULL,
	expected_output JSON,
	metadata JSON,
	source_trace_id VARCHAR(36),
	source_observation_id VARCHAR(36),
	status VARCHAR(50) NOT NULL DEFAULT 'ACTIVE',
	is_deleted TINYINT(1) NOT NULL DEFAULT 0,
	valid_from DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	valid_to DATETIME(3),
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	PRIMARY KEY (id, project_id, valid_from),
	INDEX idx_project_dataset (project_id, dataset_name),
	INDEX idx_trace_id (project_id, source_trace_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS dataset_runs (
	id VARCHAR(36) NOT NULL,
	project_id VARCHAR(36) NOT NULL,
	dataset_name VARCHAR(255) NOT NULL,
	name VARCHAR(255) NOT NULL,
	metadata JSON,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	PRIMARY KEY (id, project_id),
	INDEX idx_project_dataset (project_id, dataset_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS dataset_run_items (
	id VARCHAR(36) NOT NULL,
	project_id VARCHAR(36) NOT NULL,
	dataset_run_id VARCHAR(36) NOT NULL,
	dataset_run_name VARCHAR(255) NOT NULL,
	dataset_item_id VARCHAR(36) NOT NULL,
	trace_id VARCHAR(36),
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	PRIMARY KEY (id, project_id),
	INDEX idx_project_run (project_id, dataset_run_id),
	INDEX idx_project_item (project_id, dataset_item_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS scores (
	id VARCHAR(36) NOT NULL,
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
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	PRIMARY KEY (id, project_id),
	INDEX idx_project_trace (project_id, trace_id),
	INDEX idx_project_observation (project_id, observation_id),
	INDEX idx_project_name (project_id, name),
	INDEX idx_project_session (project_id, session_id),
	INDEX idx_project_dataset_run (project_id, dataset_run_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS score_configs (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	name VARCHAR(255) NOT NULL,
	data_type VARCHAR(50) NOT NULL,
	min_value DECIMAL(65,30),
	max_value DECIMAL(65,30),
	categories JSON,
	description TEXT,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_project_name (project_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS comments (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	object_type VARCHAR(50) NOT NULL,
	object_id VARCHAR(36) NOT NULL,
	content TEXT NOT NULL,
	author_id VARCHAR(36) NOT NULL,
	path JSON,
	range_start JSON,
	range_end JSON,
	data_field VARCHAR(255),
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	INDEX idx_project_object (project_id, object_type, object_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS comment_reactions (
	id VARCHAR(36) PRIMARY KEY,
	comment_id VARCHAR(36) NOT NULL,
	user_id VARCHAR(36) NOT NULL,
	emoji VARCHAR(50) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_comment_user_emoji (comment_id, user_id, emoji)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS notifications (
	id VARCHAR(36) PRIMARY KEY,
	user_id VARCHAR(36) NOT NULL,
	transaction_id VARCHAR(255),
	channel VARCHAR(50) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	INDEX idx_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS notification_preferences (
	id VARCHAR(36) PRIMARY KEY,
	user_id VARCHAR(36) NOT NULL,
	project_id VARCHAR(36) NOT NULL,
	channel VARCHAR(50) NOT NULL,
	type VARCHAR(50) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_user_project_channel_type (user_id, project_id, channel, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS sso_configs (
	id VARCHAR(36) PRIMARY KEY,
	org_id VARCHAR(36) NOT NULL,
	provider VARCHAR(50) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_org_provider (org_id, provider)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS audit_logs (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36),
	org_id VARCHAR(36) NOT NULL,
	user_id VARCHAR(36),
	user_email VARCHAR(255),
	resource_type VARCHAR(255) NOT NULL,
	resource_id VARCHAR(255) NOT NULL,
	action VARCHAR(255) NOT NULL,
	`before` TEXT,
	`after` TEXT,
	api_key_id VARCHAR(36),
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	INDEX idx_org_created (org_id, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS batch_exports (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	user_id VARCHAR(36) NOT NULL,
	name VARCHAR(255) NOT NULL,
	query JSON NOT NULL,
	status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	finished_at DATETIME(3),
	INDEX idx_project_status (project_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS cron_jobs (
	name VARCHAR(255) PRIMARY KEY,
	locked_at DATETIME(3),
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS media (
	id VARCHAR(36) NOT NULL,
	project_id VARCHAR(36) NOT NULL,
	content_type VARCHAR(255) NOT NULL,
	content_length BIGINT,
	upload_url TEXT,
	sha256_hash CHAR(44),
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	PRIMARY KEY (id, project_id),
	UNIQUE KEY uk_project_id (project_id, id),
	UNIQUE KEY uk_project_sha (project_id, sha256_hash)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS trace_media (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	trace_id VARCHAR(36) NOT NULL,
	media_id VARCHAR(36) NOT NULL,
	field VARCHAR(255) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_project_trace_media_field (project_id, trace_id, media_id, field)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS observation_media (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	trace_id VARCHAR(36) NOT NULL,
	observation_id VARCHAR(36) NOT NULL,
	media_id VARCHAR(36) NOT NULL,
	field VARCHAR(255) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_project_trace_obs_media_field (project_id, trace_id, observation_id, media_id, field)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS dashboards (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	name VARCHAR(255) NOT NULL,
	description TEXT,
	definition JSON NOT NULL,
	filters JSON,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	INDEX idx_project_id (project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS dashboard_widgets (
	id VARCHAR(36) PRIMARY KEY,
	dashboard_id VARCHAR(36) NOT NULL,
	view VARCHAR(100) NOT NULL,
	title VARCHAR(255),
	description TEXT,
	chart_type VARCHAR(50),
	dimensions JSON,
	metrics JSON,
	filters JSON,
	chart_config JSON,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	INDEX idx_dashboard_id (dashboard_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS table_view_presets (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	table_name VARCHAR(255) NOT NULL,
	name VARCHAR(255) NOT NULL,
	filters JSON,
	column_order JSON,
	column_visibility JSON,
	order_by JSON,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_project_table_name (project_id, table_name, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS llm_api_keys (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	provider VARCHAR(255) NOT NULL,
	adapter VARCHAR(50),
	display_name VARCHAR(255),
	config JSON,
	custom_models JSON,
	extra_header_keys JSON,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_project_provider (project_id, provider)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS default_llm_models (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	model_params JSON,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_project_id (project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS llm_schemas (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	name VARCHAR(255) NOT NULL,
	`schema` JSON NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_project_name (project_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS llm_tools (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	name VARCHAR(255) NOT NULL,
	description TEXT,
	parameters JSON NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_project_name (project_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS posthog_integrations (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	display_name VARCHAR(255) NOT NULL,
	integration_type VARCHAR(50) NOT NULL,
	integration_config JSON NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_project_type (project_id, integration_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS mixpanel_integrations (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	display_name VARCHAR(255) NOT NULL,
	integration_type VARCHAR(50) NOT NULL,
	integration_config JSON NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_project_type (project_id, integration_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS blob_storage_integrations (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	display_name VARCHAR(255) NOT NULL,
	type VARCHAR(50) NOT NULL,
	config JSON NOT NULL,
	last_log_at DATETIME(3),
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	INDEX idx_project_id (project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS slack_integrations (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	display_name VARCHAR(255),
	integration_type VARCHAR(50) NOT NULL,
	integration_config JSON NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_project_type (project_id, integration_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS actions (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	type VARCHAR(50) NOT NULL,
	config JSON NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	INDEX idx_project_type (project_id, type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS triggers (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	event_type VARCHAR(100) NOT NULL,
	event_actions JSON,
	filter JSON,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	INDEX idx_project_event (project_id, event_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS automations (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	trigger_id VARCHAR(36) NOT NULL,
	action_id VARCHAR(36) NOT NULL,
	name VARCHAR(255),
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	INDEX idx_project_id (project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS automation_executions (
	id VARCHAR(36) PRIMARY KEY,
	automation_id VARCHAR(36) NOT NULL,
	status VARCHAR(50) NOT NULL,
	input JSON,
	output JSON,
	error TEXT,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	INDEX idx_automation_id (automation_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS annotation_queues (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	name VARCHAR(255) NOT NULL,
	description TEXT,
	score_config_ids JSON,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_project_name (project_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS annotation_queue_items (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	queue_id VARCHAR(36) NOT NULL,
	object_type VARCHAR(50) NOT NULL,
	object_id VARCHAR(36) NOT NULL,
	status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_project_queue_object (project_id, queue_id, object_type, object_id),
	INDEX idx_project_queue_status (project_id, queue_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS annotation_queue_assignments (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	queue_id VARCHAR(36) NOT NULL,
	user_id VARCHAR(36) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_project_queue_user (project_id, queue_id, user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS eval_templates (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	name VARCHAR(255) NOT NULL,
	variable_mapping JSON NOT NULL,
	vars JSON NOT NULL,
	output_schema JSON NOT NULL,
	provider VARCHAR(255) NOT NULL,
	model VARCHAR(255) NOT NULL,
	model_params JSON,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_project_name (project_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS job_configurations (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	job_type VARCHAR(50) NOT NULL,
	status VARCHAR(50) NOT NULL,
	eval_template_id VARCHAR(36) NOT NULL,
	target_object VARCHAR(50) NOT NULL,
	filter JSON NOT NULL,
	variable_mapping JSON NOT NULL,
	time_scope JSON NOT NULL,
	delay DECIMAL(10,2),
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	INDEX idx_project_status (project_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS job_executions (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	job_configuration_id VARCHAR(36) NOT NULL,
	status VARCHAR(50) NOT NULL,
	start_time DATETIME(3) NOT NULL,
	end_time DATETIME(3),
	error TEXT,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	INDEX idx_project_job (project_id, job_configuration_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS surveys (
	id VARCHAR(36) PRIMARY KEY,
	name VARCHAR(255) NOT NULL,
	project_id VARCHAR(36) NOT NULL,
	response JSON NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	INDEX idx_project_name (project_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS billing_meter_backups (
	id VARCHAR(36) PRIMARY KEY,
	stripe_meter_id VARCHAR(255) NOT NULL,
	value DECIMAL(65,30) NOT NULL,
	timestamp DATETIME(3) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	INDEX idx_stripe_meter (stripe_meter_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS cloud_spend_alerts (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	threshold DECIMAL(65,30) NOT NULL,
	recipient_email VARCHAR(255) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	INDEX idx_project_id (project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS background_migrations (
	id VARCHAR(36) PRIMARY KEY,
	migration_name VARCHAR(255) NOT NULL UNIQUE,
	state JSON,
	args JSON,
	status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	finished_at DATETIME(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS batch_actions (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	user_id VARCHAR(36) NOT NULL,
	action VARCHAR(255) NOT NULL,
	query JSON NOT NULL,
	config JSON,
	status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
	finished_at DATETIME(3),
	INDEX idx_project_status (project_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS prompt_dependencies (
	id VARCHAR(36) PRIMARY KEY,
	parent_prompt_id VARCHAR(36) NOT NULL,
	child_prompt_id VARCHAR(36) NOT NULL,
	project_id VARCHAR(36) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	INDEX idx_parent (parent_prompt_id),
	INDEX idx_child (child_prompt_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS prompt_protected_labels (
	id VARCHAR(36) PRIMARY KEY,
	project_id VARCHAR(36) NOT NULL,
	label VARCHAR(255) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_project_label (project_id, label)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS membership_invitations (
	id VARCHAR(36) PRIMARY KEY,
	org_id VARCHAR(36) NOT NULL,
	email VARCHAR(255) NOT NULL,
	role VARCHAR(50) NOT NULL,
	inviter_id VARCHAR(36) NOT NULL,
	expires_at DATETIME(3) NOT NULL,
	created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	UNIQUE KEY uk_email_org (email, org_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

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
