-- 003: pg_catalog Emulation Tables
-- PostgreSQL system catalog emulation for Prisma ORM compatibility
-- These tables are automatically created by agentx-proxy on startup.
-- This file is for reference only.

CREATE TABLE IF NOT EXISTS pg_type (
	oid INT NOT NULL PRIMARY KEY,
	typname VARCHAR(255) NOT NULL,
	typnamespace INT NOT NULL DEFAULT 11,
	typtype CHAR(1) NOT NULL DEFAULT 'b',
	typcategory CHAR(1) NOT NULL DEFAULT 'S',
	typrelid INT NOT NULL DEFAULT 0,
	typelem INT NOT NULL DEFAULT 0,
	typarray INT NOT NULL DEFAULT 0,
	typinput VARCHAR(255) NOT NULL DEFAULT '-',
	typoutput VARCHAR(255) NOT NULL DEFAULT '-',
	typreceive VARCHAR(255) NOT NULL DEFAULT '-',
	typsend VARCHAR(255) NOT NULL DEFAULT '-',
	typmod_in VARCHAR(255) NOT NULL DEFAULT '-',
	typmod_out VARCHAR(255) NOT NULL DEFAULT '-',
	typalign CHAR(1) NOT NULL DEFAULT 'i',
	typstorage CHAR(1) NOT NULL DEFAULT 'p',
	typnotnull TINYINT(1) NOT NULL DEFAULT 0,
	typbasetype INT NOT NULL DEFAULT 0,
	typtypmod INT NOT NULL DEFAULT -1,
	typndims INT NOT NULL DEFAULT 0,
	typcollation INT NOT NULL DEFAULT 0,
	typdefaultbin TEXT,
	typdefault TEXT,
	typacl TEXT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS pg_class (
	oid INT NOT NULL PRIMARY KEY,
	relname VARCHAR(255) NOT NULL,
	relnamespace INT NOT NULL DEFAULT 2200,
	reltype INT NOT NULL DEFAULT 0,
	reloftype INT NOT NULL DEFAULT 0,
	relowner INT NOT NULL DEFAULT 0,
	relam INT NOT NULL DEFAULT 0,
	relfilenode INT NOT NULL DEFAULT 0,
	reltablespace INT NOT NULL DEFAULT 0,
	relpages INT NOT NULL DEFAULT 0,
	reltuples FLOAT NOT NULL DEFAULT 0,
	relallvisible INT NOT NULL DEFAULT 0,
	reltoastrelid INT NOT NULL DEFAULT 0,
	relhasindex TINYINT(1) NOT NULL DEFAULT 0,
	relisshared TINYINT(1) NOT NULL DEFAULT 0,
	relpersistence CHAR(1) NOT NULL DEFAULT 'p',
	relkind CHAR(1) NOT NULL DEFAULT 'r',
	relnatts INT NOT NULL DEFAULT 0,
	relchecks INT NOT NULL DEFAULT 0,
	relhasoids TINYINT(1) NOT NULL DEFAULT 0,
	relhaspkey TINYINT(1) NOT NULL DEFAULT 0,
	relhasrules TINYINT(1) NOT NULL DEFAULT 0,
	relhastriggers TINYINT(1) NOT NULL DEFAULT 0,
	relhassubclass TINYINT(1) NOT NULL DEFAULT 0,
	relrowsecurity TINYINT(1) NOT NULL DEFAULT 0,
	relforcerowsecurity TINYINT(1) NOT NULL DEFAULT 0,
	relispopulated TINYINT(1) NOT NULL DEFAULT 1,
	relreplident CHAR(1) NOT NULL DEFAULT 'd',
	relispartition TINYINT(1) NOT NULL DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS pg_attribute (
	attrelid INT NOT NULL,
	attname VARCHAR(255) NOT NULL,
	atttypid INT NOT NULL,
	attstattarget INT NOT NULL DEFAULT -1,
	attlen SMALLINT NOT NULL,
	attnum SMALLINT NOT NULL,
	attndims INT NOT NULL DEFAULT 0,
	attcacheoff INT NOT NULL DEFAULT -1,
	atttypmod INT NOT NULL DEFAULT -1,
	attbyval TINYINT(1) NOT NULL DEFAULT 0,
	attstorage CHAR(1) NOT NULL DEFAULT 'p',
	attalign CHAR(1) NOT NULL DEFAULT 'i',
	attnotnull TINYINT(1) NOT NULL DEFAULT 0,
	atthasdef TINYINT(1) NOT NULL DEFAULT 0,
	atthasmissing TINYINT(1) NOT NULL DEFAULT 0,
	attidentity CHAR(1) NOT NULL DEFAULT '',
	attgenerated CHAR(1) NOT NULL DEFAULT '',
	attisdropped TINYINT(1) NOT NULL DEFAULT 0,
	attislocal TINYINT(1) NOT NULL DEFAULT 1,
	attinhcount INT NOT NULL DEFAULT 0,
	attcollation INT NOT NULL DEFAULT 0,
	attacl TEXT,
	attoptions TEXT,
	attfdwoptions TEXT,
	PRIMARY KEY (attrelid, attnum)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS pg_namespace (
	oid INT NOT NULL PRIMARY KEY,
	nspname VARCHAR(255) NOT NULL,
	nspowner INT NOT NULL DEFAULT 0,
	nspacl TEXT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS pg_index (
	indexrelid INT NOT NULL,
	indrelid INT NOT NULL,
	indnatts SMALLINT NOT NULL,
	indnkeyatts SMALLINT NOT NULL,
	indisunique TINYINT(1) NOT NULL DEFAULT 0,
	indisprimary TINYINT(1) NOT NULL DEFAULT 0,
	indisexclusion TINYINT(1) NOT NULL DEFAULT 0,
	indimmediate TINYINT(1) NOT NULL DEFAULT 1,
	indisclustered TINYINT(1) NOT NULL DEFAULT 0,
	indisvalid TINYINT(1) NOT NULL DEFAULT 1,
	indcheckxmin TINYINT(1) NOT NULL DEFAULT 0,
	indisready TINYINT(1) NOT NULL DEFAULT 1,
	indislive TINYINT(1) NOT NULL DEFAULT 1,
	indisreplident TINYINT(1) NOT NULL DEFAULT 0,
	indkey TEXT,
	indcollation TEXT,
	indclass TEXT,
	indoption TEXT,
	indexprs TEXT,
	indpred TEXT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS pg_proc (
	oid INT NOT NULL PRIMARY KEY,
	proname VARCHAR(255) NOT NULL,
	pronamespace INT NOT NULL DEFAULT 11,
	proowner INT NOT NULL DEFAULT 0,
	prolang INT NOT NULL DEFAULT 0,
	procost FLOAT NOT NULL DEFAULT 1,
	prorows FLOAT NOT NULL DEFAULT 0,
	provariadic INT NOT NULL DEFAULT 0,
	prosupport VARCHAR(255),
	prokind CHAR(1) NOT NULL DEFAULT 'f',
	prosecdef TINYINT(1) NOT NULL DEFAULT 0,
	proleakproof TINYINT(1) NOT NULL DEFAULT 0,
	proisstrict TINYINT(1) NOT NULL DEFAULT 0,
	proretset TINYINT(1) NOT NULL DEFAULT 0,
	provolatile CHAR(1) NOT NULL DEFAULT 'v',
	proparallel CHAR(1) NOT NULL DEFAULT 's',
	pronargs SMALLINT NOT NULL,
	pronargdefaults SMALLINT NOT NULL DEFAULT 0,
	prorettype INT NOT NULL,
	proargtypes TEXT,
	proallargtypes TEXT,
	proargmodes TEXT,
	proargnames TEXT,
	proargdefaults TEXT,
	protrftypes TEXT,
	prosrc TEXT,
	probin TEXT,
	proconfig TEXT,
	proacl TEXT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS pg_enum (
	oid INT NOT NULL PRIMARY KEY,
	enumtypid INT NOT NULL,
	enumsortorder FLOAT NOT NULL,
	enumlabel VARCHAR(255) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS pg_constraint (
	oid INT NOT NULL PRIMARY KEY,
	conname VARCHAR(255) NOT NULL,
	connamespace INT NOT NULL DEFAULT 2200,
	contype CHAR(1) NOT NULL,
	condeferrable TINYINT(1) NOT NULL DEFAULT 0,
	condeferred TINYINT(1) NOT NULL DEFAULT 0,
	convalidated TINYINT(1) NOT NULL DEFAULT 1,
	conrelid INT NOT NULL DEFAULT 0,
	contypid INT NOT NULL DEFAULT 0,
	conindid INT NOT NULL DEFAULT 0,
	conparentid INT NOT NULL DEFAULT 0,
	confrelid INT NOT NULL DEFAULT 0,
	confupdtype CHAR(1) NOT NULL DEFAULT 'a',
	confdeltype CHAR(1) NOT NULL DEFAULT 'a',
	confmatchtype CHAR(1) NOT NULL DEFAULT 's',
	conislocal TINYINT(1) NOT NULL DEFAULT 1,
	coninhcount INT NOT NULL DEFAULT 0,
	connoinherit TINYINT(1) NOT NULL DEFAULT 0,
	conkey TEXT,
	confkey TEXT,
	conpfeqop TEXT,
	conppeqop TEXT,
	conffeqop TEXT,
	conexclop TEXT,
	conbin TEXT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS pg_description (
	objoid INT NOT NULL,
	classoid INT NOT NULL,
	objsubid INT NOT NULL,
	description TEXT,
	PRIMARY KEY (objoid, classoid, objsubid)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS pg_database (
	oid INT NOT NULL PRIMARY KEY,
	datname VARCHAR(255) NOT NULL,
	datdba INT NOT NULL DEFAULT 0,
	encoding INT NOT NULL DEFAULT 6,
	datcollate VARCHAR(255) NOT NULL,
	datctype VARCHAR(255) NOT NULL,
	datistemplate TINYINT(1) NOT NULL DEFAULT 0,
	datallowconn TINYINT(1) NOT NULL DEFAULT 1,
	datconnlimit INT NOT NULL DEFAULT -1,
	datfrozenxid INT NOT NULL DEFAULT 0,
	datminmxid INT NOT NULL DEFAULT 1,
	dattablespace INT NOT NULL DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
